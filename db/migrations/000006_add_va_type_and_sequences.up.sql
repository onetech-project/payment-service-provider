-- Adds VA type classification to va_transactions and a per-vaType sequence
-- counter table backing system-generated customerNo values for dynamic VAs
-- (feature 006-static-dynamic-va).
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS va_type VARCHAR(2);
CREATE INDEX IF NOT EXISTS idx_va_transactions_customer_no ON va_transactions(partner_service_id, customer_no);

CREATE TABLE IF NOT EXISTS va_customer_no_sequences (
    va_type VARCHAR(2) PRIMARY KEY,
    next_seq BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO va_customer_no_sequences (va_type, next_seq)
VALUES ('04', 1), ('05', 1), ('06', 1)
ON CONFLICT (va_type) DO NOTHING;
