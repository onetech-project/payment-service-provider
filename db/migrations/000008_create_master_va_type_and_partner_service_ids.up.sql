-- Master data for VA type rules and reserved partner service IDs (feature
-- 006-static-dynamic-va amendment), replacing the previously hardcoded
-- vaTypeRules/reservedVAPartnerServiceIDs maps in internal/domain/va.go so
-- operators can add/adjust these without a code deployment.
-- id columns use VARCHAR(36) app-generated UUIDs (via google/uuid), matching
-- the existing convention in this schema (e.g. va_transactions.id) rather
-- than a native UUID column + pgcrypto/uuid-ossp extension, which isn't
-- enabled anywhere else in this database.
CREATE TABLE IF NOT EXISTS master_partner_service_ids (
    id VARCHAR(36) PRIMARY KEY,
    partner_service_id VARCHAR(8) UNIQUE NOT NULL,
    bank_code VARCHAR(20) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS master_va_type (
    id VARCHAR(36) PRIMARY KEY,
    va_type VARCHAR(2) UNIQUE NOT NULL,
    dynamic BOOLEAN NOT NULL,
    billing VARCHAR(10) NOT NULL CHECK (billing IN ('none', 'variable', 'fixed')),
    description VARCHAR(255),
    partner_service_id VARCHAR(8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_master_va_type_partner_service_id ON master_va_type(partner_service_id);

-- Seed data mirrors the previously hardcoded rules exactly, so first-boot
-- behavior is unchanged (spec.md User Story 4, Acceptance Scenario 1). Fixed
-- literal ids are used (rather than a generated value) so this seed insert is
-- reproducibly idempotent across environments.
INSERT INTO master_partner_service_ids (id, partner_service_id, bank_code) VALUES
    ('00000000-0000-0000-0000-000000015973', '15973', 'BANK-15973'),
    ('00000000-0000-0000-0000-000000015974', '15974', 'BANK-15974'),
    ('00000000-0000-0000-0000-000000015975', '15975', 'BANK-15975')
ON CONFLICT (partner_service_id) DO NOTHING;

INSERT INTO master_va_type (id, va_type, dynamic, billing, description, partner_service_id) VALUES
    ('00000000-0000-0000-0000-000000000001', '01', FALSE, 'none',     'Static no bill',       '15973'),
    ('00000000-0000-0000-0000-000000000002', '02', FALSE, 'variable', 'Static variable bill',  '15974'),
    ('00000000-0000-0000-0000-000000000003', '03', FALSE, 'fixed',    'Static fixed bill',     '15975'),
    ('00000000-0000-0000-0000-000000000004', '04', TRUE,  'none',     'Dynamic no bill',       '15973'),
    ('00000000-0000-0000-0000-000000000005', '05', TRUE,  'variable', 'Dynamic variable bill', '15974'),
    ('00000000-0000-0000-0000-000000000006', '06', TRUE,  'fixed',    'Dynamic fixed bill',    '15975')
ON CONFLICT (va_type) DO NOTHING;
