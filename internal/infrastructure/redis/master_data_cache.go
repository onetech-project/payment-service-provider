package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backbone-new/internal/domain"

	"github.com/redis/go-redis/v9"
)

// masterDataCacheTTL is the scheduled safety-net refresh interval (feature
// 006-static-dynamic-va amendment, FR-017). Explicit RefreshNow() calls from
// the write path keep this fresher in practice; the TTL just bounds how
// stale the cache can get if no mutation ever happens.
const masterDataCacheTTL = 5 * time.Minute

const (
	vaTypesCacheKey           = "master:va_types"
	partnerServiceIDsCacheKey = "master:partner_service_ids"
)

// MasterDataCache is a Redis-backed cache for the VA type rule / partner
// service ID master data, mirroring the existing ClientKeyCache pattern in
// this package (thin TTL-based wrapper, JSON-serialized values).
type MasterDataCache struct {
	client *Client
}

// NewMasterDataCache creates a new master data cache.
func NewMasterDataCache(client *Client) *MasterDataCache {
	return &MasterDataCache{client: client}
}

// GetVATypes returns the cached VA type rules, or (nil, nil) on a cache miss.
func (c *MasterDataCache) GetVATypes(ctx context.Context) ([]domain.VATypeRule, error) {
	val, err := c.client.GetClient().Get(ctx, vaTypesCacheKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get va type rules from redis cache: %w", err)
	}
	var rules []domain.VATypeRule
	if err := json.Unmarshal(val, &rules); err != nil {
		return nil, fmt.Errorf("failed to decode cached va type rules: %w", err)
	}
	return rules, nil
}

// SetVATypes caches the given VA type rules with the standard TTL.
func (c *MasterDataCache) SetVATypes(ctx context.Context, rules []domain.VATypeRule) error {
	val, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to encode va type rules for cache: %w", err)
	}
	return c.client.GetClient().Set(ctx, vaTypesCacheKey, val, masterDataCacheTTL).Err()
}

// GetPartnerServiceIDs returns the cached partner service ID records, or
// (nil, nil) on a cache miss.
func (c *MasterDataCache) GetPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error) {
	val, err := c.client.GetClient().Get(ctx, partnerServiceIDsCacheKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get partner service ids from redis cache: %w", err)
	}
	var records []domain.PartnerServiceIDRecord
	if err := json.Unmarshal(val, &records); err != nil {
		return nil, fmt.Errorf("failed to decode cached partner service ids: %w", err)
	}
	return records, nil
}

// SetPartnerServiceIDs caches the given partner service ID records with the
// standard TTL.
func (c *MasterDataCache) SetPartnerServiceIDs(ctx context.Context, records []domain.PartnerServiceIDRecord) error {
	val, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to encode partner service ids for cache: %w", err)
	}
	return c.client.GetClient().Set(ctx, partnerServiceIDsCacheKey, val, masterDataCacheTTL).Err()
}
