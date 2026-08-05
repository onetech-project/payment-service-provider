-- VA registry: the durable identity of a virtual account number, split out of
-- va_transactions (feature 013-no-bill-payment-transaction).
--
-- Before this migration va_transactions did two jobs at once: it WAS the VA
-- (identity, holder name, notification URL, expiry) and it WAS the transaction
-- (amount, payment request id, settlement status). That conflation is why a
-- no-bill VA could only ever be paid once -- the single pending row was
-- consumed by the first payment. va_accounts now owns identity; va_transactions
-- keeps its original meaning of one row per payment/inquiry event.
--
-- id uses VARCHAR(36) app-generated UUIDs (via google/uuid) to match the
-- existing convention in this schema (va_transactions.id, master_va_type.id).
-- The backfill below uses gen_random_uuid()::text, which is core in PG13+ and
-- needs no extension.
CREATE TABLE IF NOT EXISTS va_accounts (
    id VARCHAR(36) PRIMARY KEY,
    partner_service_id VARCHAR(8) NOT NULL,
    customer_no VARCHAR(20) NOT NULL,
    virtual_account_no VARCHAR(28) NOT NULL,
    va_type VARCHAR(2) NOT NULL,
    -- billing is denormalized from master_va_type at registration time, on
    -- purpose. The payment and inquiry paths need to know "is this a no-bill
    -- VA?" on every call, and resolving it here means (a) no master-data
    -- lookup on the hot path, and (b) an already-issued VA keeps the contract
    -- it was issued under. If an operator later edits master_va_type, that
    -- changes how NEW registrations are classified — it must not silently
    -- change the behavior of VA numbers already published to customers.
    billing VARCHAR(10) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255),
    customer_phone VARCHAR(30),
    trx_id VARCHAR(64) NOT NULL,
    notification_url VARCHAR(512),
    status VARCHAR(10) NOT NULL DEFAULT 'ACTIVE',
    expired_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_va_accounts_virtual_account_no UNIQUE (virtual_account_no),
    CONSTRAINT uq_va_accounts_partner_customer UNIQUE (partner_service_id, customer_no),
    CONSTRAINT chk_va_accounts_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'EXPIRED')),
    CONSTRAINT chk_va_accounts_billing CHECK (billing IN ('none', 'variable', 'fixed'))
);

-- uq_va_accounts_virtual_account_no is the ON CONFLICT target for the
-- registration upsert (FR-005) and makes the payment/inquiry lookup an exact
-- point read instead of GetVAByVirtualAccountNo's "ORDER BY created_at DESC
-- LIMIT 1" heuristic.
--
-- uq_va_accounts_partner_customer replaces the check-then-act inside
-- RegisterStaticCustomerNo (a Redis lock plus COUNT(*)) with a real constraint
-- the database enforces (FR-007).

CREATE INDEX IF NOT EXISTS idx_va_accounts_partner_service_id ON va_accounts(partner_service_id);

-- Backfill one registration per distinct virtual_account_no from the existing
-- managed-type transactions, so VAs created under the old flow keep working
-- (FR-022, SC-006). Holder fields are taken from the most recent transaction
-- for that VA number.
--
-- Status derivation from the latest transaction:
--   '04' (deleted)  -> INACTIVE
--   '02' (expired)  -> EXPIRED   (expired_date is copied too, so the inline
--                                 expiry detection would reach the same
--                                 conclusion; setting it here avoids briefly
--                                 resurrecting an already-expired VA)
--   anything else   -> ACTIVE
--
-- ON CONFLICT DO NOTHING (untargeted, so it covers both unique constraints)
-- makes re-running this migration safe and tolerates any legacy row pair that
-- would violate uq_va_accounts_partner_customer.
--
-- billing is resolved by joining master_va_type, the same source /create-va
-- reads. An INNER JOIN is deliberate: a transaction whose va_type no longer
-- exists in master data cannot be classified, so it is left unregistered and
-- keeps using the legacy transaction-based path rather than being guessed at.
INSERT INTO va_accounts (
    id, partner_service_id, customer_no, virtual_account_no, va_type, billing,
    customer_name, customer_email, customer_phone, trx_id, notification_url,
    status, expired_date, created_at, updated_at
)
SELECT
    gen_random_uuid()::text,
    t.partner_service_id,
    t.customer_no,
    t.virtual_account_no,
    t.va_type,
    m.billing,
    t.customer_name,
    t.customer_email,
    t.customer_phone,
    t.trx_id,
    t.notification_url,
    CASE t.status
        WHEN '04' THEN 'INACTIVE'
        WHEN '02' THEN 'EXPIRED'
        ELSE 'ACTIVE'
    END,
    t.expired_date,
    t.created_at,
    NOW()
FROM (
    SELECT DISTINCT ON (virtual_account_no)
        partner_service_id, customer_no, virtual_account_no, va_type,
        customer_name, customer_email, customer_phone, trx_id,
        COALESCE(notification_url, '') AS notification_url,
        status, expired_date, created_at
    FROM va_transactions
    WHERE va_type IN ('01', '02', '03', '04', '05', '06')
    ORDER BY virtual_account_no, created_at DESC
) AS t
JOIN master_va_type m ON m.va_type = t.va_type
ON CONFLICT DO NOTHING;
