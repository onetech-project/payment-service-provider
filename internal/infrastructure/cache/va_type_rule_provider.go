// Package cache implements domain.VATypeRuleProvider (feature
// 006-static-dynamic-va amendment): VA type rule / partner service ID master
// data backed by PostgreSQL, cached in Redis with a 5-minute TTL and
// immediate refresh on write, falling back to an in-process snapshot when
// Redis is temporarily unavailable. This orchestration lives in its own
// package (rather than inside database/ or redis/ directly) since it
// combines both plus background-refresh lifecycle — see plan.md's amendment
// "Structure Decision" for the rationale.
package cache

import (
	"context"
	"sync"
	"time"

	"backbone-new/internal/domain"
)

// masterDataRepository is the minimal PostgreSQL read surface this provider
// needs from database.MasterVADataRepository, kept as a local interface for
// testability (mirrors the vaLocker pattern in va_repository.go).
type masterDataRepository interface {
	ListVATypes(ctx context.Context) ([]domain.VATypeRule, error)
	ListPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error)
	CreateVAType(ctx context.Context, rule domain.VATypeRule) error
	UpdateVAType(ctx context.Context, rule domain.VATypeRule) error
	DeleteVAType(ctx context.Context, vaType string) error
	CreatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error
	UpdatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error
	DeletePartnerServiceID(ctx context.Context, partnerServiceID string) error
}

// masterDataCache is the minimal Redis cache surface this provider needs
// from redis.MasterDataCache, kept as a local interface for testability.
type masterDataCache interface {
	GetVATypes(ctx context.Context) ([]domain.VATypeRule, error)
	SetVATypes(ctx context.Context, rules []domain.VATypeRule) error
	GetPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error)
	SetPartnerServiceIDs(ctx context.Context, records []domain.PartnerServiceIDRecord) error
}

// DefaultRefreshInterval is the scheduled safety-net refresh interval
// (FR-017's "5 minutes"). Mutations made through this application's own
// data-access layer should call RefreshNow instead of waiting for this.
const DefaultRefreshInterval = 5 * time.Minute

// CachedVATypeRuleProvider implements domain.VATypeRuleProvider.
type CachedVATypeRuleProvider struct {
	repo  masterDataRepository
	cache masterDataCache

	mu         sync.RWMutex
	vaTypes    []domain.VATypeRule
	partnerIDs []domain.PartnerServiceIDRecord

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewCachedVATypeRuleProvider creates a new provider. Call RefreshNow once
// before serving traffic to warm both the cache and the in-process snapshot,
// and Start to begin the scheduled background refresh.
func NewCachedVATypeRuleProvider(repo masterDataRepository, cache masterDataCache) *CachedVATypeRuleProvider {
	return &CachedVATypeRuleProvider{
		repo:   repo,
		cache:  cache,
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduled background refresh at the given interval. Safe
// to call once per provider instance; the goroutine exits when Stop is called.
func (p *CachedVATypeRuleProvider) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = p.RefreshNow(ctx)
			case <-p.stopCh:
				return
			}
		}
	}()
}

// Stop halts the background refresh goroutine started by Start.
func (p *CachedVATypeRuleProvider) Stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
}

// RefreshNow reloads both master data lists from PostgreSQL and writes them
// through to the Redis cache and the in-process snapshot. Call this
// immediately after any mutation made through the application's own
// data-access layer, per FR-017.
func (p *CachedVATypeRuleProvider) RefreshNow(ctx context.Context) error {
	vaTypes, err := p.repo.ListVATypes(ctx)
	if err != nil {
		return err
	}
	partnerIDs, err := p.repo.ListPartnerServiceIDs(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.vaTypes = vaTypes
	p.partnerIDs = partnerIDs
	p.mu.Unlock()

	// Best-effort: a Redis write failure here doesn't invalidate the refresh
	// itself — the in-process snapshot just updated above is still correct,
	// and the next scheduled tick or cache read will retry populating Redis.
	_ = p.cache.SetVATypes(ctx, vaTypes)
	_ = p.cache.SetPartnerServiceIDs(ctx, partnerIDs)
	return nil
}

func (p *CachedVATypeRuleProvider) snapshotVATypes() []domain.VATypeRule {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.vaTypes
}

func (p *CachedVATypeRuleProvider) snapshotPartnerIDs() []domain.PartnerServiceIDRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.partnerIDs
}

// vaTypes returns the current VA type rules: cache hit if available,
// otherwise refilled from PostgreSQL (updating the cache and snapshot),
// otherwise the last in-process snapshot if Redis itself is unreachable,
// otherwise a direct PostgreSQL read as a last resort.
func (p *CachedVATypeRuleProvider) currentVATypes(ctx context.Context) ([]domain.VATypeRule, error) {
	cached, err := p.cache.GetVATypes(ctx)
	if err == nil {
		if cached != nil {
			p.mu.Lock()
			p.vaTypes = cached
			p.mu.Unlock()
			return cached, nil
		}
		// Cache miss (not a Redis error) — refill from PostgreSQL.
		fresh, dbErr := p.repo.ListVATypes(ctx)
		if dbErr == nil {
			p.mu.Lock()
			p.vaTypes = fresh
			p.mu.Unlock()
			_ = p.cache.SetVATypes(ctx, fresh)
			return fresh, nil
		}
		err = dbErr
	}

	// Redis (or the refill) failed — fall back to the last known-good
	// in-process snapshot rather than failing the request (FR-018).
	if snap := p.snapshotVATypes(); snap != nil {
		return snap, nil
	}

	// No snapshot yet (e.g. cold start with Redis down) — last resort: read
	// PostgreSQL directly.
	fresh, dbErr := p.repo.ListVATypes(ctx)
	if dbErr != nil {
		return nil, err
	}
	return fresh, nil
}

func (p *CachedVATypeRuleProvider) currentPartnerIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error) {
	cached, err := p.cache.GetPartnerServiceIDs(ctx)
	if err == nil {
		if cached != nil {
			p.mu.Lock()
			p.partnerIDs = cached
			p.mu.Unlock()
			return cached, nil
		}
		fresh, dbErr := p.repo.ListPartnerServiceIDs(ctx)
		if dbErr == nil {
			p.mu.Lock()
			p.partnerIDs = fresh
			p.mu.Unlock()
			_ = p.cache.SetPartnerServiceIDs(ctx, fresh)
			return fresh, nil
		}
		err = dbErr
	}

	if snap := p.snapshotPartnerIDs(); snap != nil {
		return snap, nil
	}

	fresh, dbErr := p.repo.ListPartnerServiceIDs(ctx)
	if dbErr != nil {
		return nil, err
	}
	return fresh, nil
}

// LookupVATypeRule implements domain.VATypeRuleProvider.
func (p *CachedVATypeRuleProvider) LookupVATypeRule(ctx context.Context, partnerServiceID, vaType string) (domain.VATypeRule, bool, error) {
	rules, err := p.currentVATypes(ctx)
	if err != nil {
		return domain.VATypeRule{}, false, err
	}
	for _, rule := range rules {
		if rule.VAType == vaType && rule.PartnerServiceID == partnerServiceID {
			return rule, true, nil
		}
	}
	return domain.VATypeRule{}, false, nil
}

// IsReservedPartnerServiceID implements domain.VATypeRuleProvider.
func (p *CachedVATypeRuleProvider) IsReservedPartnerServiceID(ctx context.Context, partnerServiceID string) (bool, error) {
	records, err := p.currentPartnerIDs(ctx)
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.PartnerServiceID == partnerServiceID {
			return true, nil
		}
	}
	return false, nil
}

// CreateVAType creates a master_va_type row and immediately refreshes the
// cache/snapshot so the change is visible to the next /create-va request
// without waiting for the scheduled interval (FR-017).
func (p *CachedVATypeRuleProvider) CreateVAType(ctx context.Context, rule domain.VATypeRule) error {
	if err := p.repo.CreateVAType(ctx, rule); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

// UpdateVAType updates a master_va_type row and immediately refreshes the
// cache/snapshot.
func (p *CachedVATypeRuleProvider) UpdateVAType(ctx context.Context, rule domain.VATypeRule) error {
	if err := p.repo.UpdateVAType(ctx, rule); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

// DeleteVAType deletes a master_va_type row and immediately refreshes the
// cache/snapshot.
func (p *CachedVATypeRuleProvider) DeleteVAType(ctx context.Context, vaType string) error {
	if err := p.repo.DeleteVAType(ctx, vaType); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

// CreatePartnerServiceID creates a master_partner_service_ids row and
// immediately refreshes the cache/snapshot.
func (p *CachedVATypeRuleProvider) CreatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	if err := p.repo.CreatePartnerServiceID(ctx, record); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

// UpdatePartnerServiceID updates a master_partner_service_ids row and
// immediately refreshes the cache/snapshot.
func (p *CachedVATypeRuleProvider) UpdatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	if err := p.repo.UpdatePartnerServiceID(ctx, record); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

// DeletePartnerServiceID deletes a master_partner_service_ids row and
// immediately refreshes the cache/snapshot.
func (p *CachedVATypeRuleProvider) DeletePartnerServiceID(ctx context.Context, partnerServiceID string) error {
	if err := p.repo.DeletePartnerServiceID(ctx, partnerServiceID); err != nil {
		return err
	}
	return p.RefreshNow(ctx)
}

var _ domain.VATypeRuleProvider = (*CachedVATypeRuleProvider)(nil)
