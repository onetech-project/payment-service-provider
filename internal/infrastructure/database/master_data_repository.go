package database

import (
	"context"

	"backbone-new/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MasterVADataRepository implements domain.MasterDataRepository using
// PostgreSQL (feature 006-static-dynamic-va amendment: master_va_type /
// master_partner_service_ids tables).
type MasterVADataRepository struct {
	pool *pgxpool.Pool
}

// NewMasterVADataRepository creates a new master data repository.
func NewMasterVADataRepository(pool *pgxpool.Pool) *MasterVADataRepository {
	return &MasterVADataRepository{pool: pool}
}

// ListVATypes returns all master_va_type rows.
func (r *MasterVADataRepository) ListVATypes(ctx context.Context) ([]domain.VATypeRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT va_type, dynamic, billing, description, partner_service_id
		FROM master_va_type
		ORDER BY va_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.VATypeRule
	for rows.Next() {
		var rule domain.VATypeRule
		var billing string
		var description *string
		if err := rows.Scan(&rule.VAType, &rule.Dynamic, &billing, &description, &rule.PartnerServiceID); err != nil {
			return nil, err
		}
		rule.Billing = domain.VATypeBilling(billing)
		if description != nil {
			rule.Description = *description
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// ListPartnerServiceIDs returns all master_partner_service_ids rows.
func (r *MasterVADataRepository) ListPartnerServiceIDs(ctx context.Context) ([]domain.PartnerServiceIDRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT partner_service_id, bank_code
		FROM master_partner_service_ids
		ORDER BY partner_service_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []domain.PartnerServiceIDRecord
	for rows.Next() {
		var record domain.PartnerServiceIDRecord
		if err := rows.Scan(&record.PartnerServiceID, &record.BankCode); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// CreateVAType inserts a new master_va_type row.
func (r *MasterVADataRepository) CreateVAType(ctx context.Context, rule domain.VATypeRule) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO master_va_type (id, va_type, dynamic, billing, description, partner_service_id)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New().String(), rule.VAType, rule.Dynamic, string(rule.Billing), rule.Description, rule.PartnerServiceID,
	)
	return err
}

// UpdateVAType updates an existing master_va_type row by va_type.
func (r *MasterVADataRepository) UpdateVAType(ctx context.Context, rule domain.VATypeRule) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE master_va_type
		SET dynamic = $2, billing = $3, description = $4, partner_service_id = $5, updated_at = NOW()
		WHERE va_type = $1`,
		rule.VAType, rule.Dynamic, string(rule.Billing), rule.Description, rule.PartnerServiceID,
	)
	return err
}

// DeleteVAType removes a master_va_type row by va_type.
func (r *MasterVADataRepository) DeleteVAType(ctx context.Context, vaType string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM master_va_type WHERE va_type = $1`, vaType)
	return err
}

// CreatePartnerServiceID inserts a new master_partner_service_ids row.
func (r *MasterVADataRepository) CreatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO master_partner_service_ids (id, partner_service_id, bank_code)
		VALUES ($1, $2, $3)`,
		uuid.New().String(), record.PartnerServiceID, record.BankCode,
	)
	return err
}

// UpdatePartnerServiceID updates an existing master_partner_service_ids row.
func (r *MasterVADataRepository) UpdatePartnerServiceID(ctx context.Context, record domain.PartnerServiceIDRecord) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE master_partner_service_ids
		SET bank_code = $2, updated_at = NOW()
		WHERE partner_service_id = $1`,
		record.PartnerServiceID, record.BankCode,
	)
	return err
}

// DeletePartnerServiceID removes a master_partner_service_ids row.
func (r *MasterVADataRepository) DeletePartnerServiceID(ctx context.Context, partnerServiceID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM master_partner_service_ids WHERE partner_service_id = $1`, partnerServiceID)
	return err
}

var _ domain.MasterDataRepository = (*MasterVADataRepository)(nil)
