-- Records each individual payment made against a variable-bill VA so
-- cumulative progress toward total_amount can be tracked and audited
-- (feature 006-static-dynamic-va, FR-013).
CREATE TABLE IF NOT EXISTS va_payments (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES va_transactions(id),
    amount NUMERIC(16,2) NOT NULL,
    reference_no VARCHAR(11),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_va_payments_transaction ON va_payments(transaction_id);
