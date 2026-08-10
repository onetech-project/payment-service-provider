-- Records the outcome of every payment-flag request BCA sends, keyed on the
-- exact pair BCA's double-flagging rule names: X-EXTERNAL-ID and
-- paymentRequestId.
--
-- Why a table of its own, when va_transactions already stores
-- payment_request_id: that column is only written when a payment SUCCEEDS.
-- Every rejection (Invalid Bill, Invalid Amount, Paid Bill, expired) returns
-- without persisting anything, so a re-flag of a rejected payment had nothing
-- to compare against and was simply recomputed from scratch — the vendor got
-- the same 404 twice instead of the 4042518 "Inconsistent Request" the spec
-- requires. X-EXTERNAL-ID was not stored anywhere at all; it lived only as the
-- Redis idempotency key, which expires.
--
-- Deliberately NOT the request/response audit log. That is high-volume,
-- write-once, read-almost-never data and belongs in Loki via Alloy. This table
-- is on the correctness path: a 4042518 answer must still be right long after
-- any log retention window has passed, so it stays narrow and permanent.
CREATE TABLE IF NOT EXISTS va_payment_flags (
    id VARCHAR(36) PRIMARY KEY,
    -- The two halves of BCA's condition. Both NOT NULL: a row that cannot
    -- identify which flag it describes cannot answer a replay of it either.
    external_id VARCHAR(36) NOT NULL,
    payment_request_id VARCHAR(128) NOT NULL,
    virtual_account_no VARCHAR(28) NOT NULL,
    -- The first request's verdict, kept as the vendor saw it.
    response_code VARCHAR(7) NOT NULL,
    response_message TEXT NOT NULL,
    -- Duplicated out of virtual_account_data so operators can answer "how many
    -- flags did we reject today?" without digging through JSONB.
    payment_flag_status VARCHAR(2) NOT NULL,
    -- The whole virtualAccountData block of the first response. A double-flag
    -- must echo paymentFlagStatus and paymentFlagReason "according to the
    -- results of the first request"; storing the rendered block rather than
    -- its ingredients means the replay is byte-for-byte that answer, amounts
    -- and reason text included, with no chance of re-deriving them differently
    -- than the first time.
    virtual_account_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The lookup key, and the guard that makes recording idempotent: the writer
-- uses ON CONFLICT DO NOTHING, so a concurrent double-flag that races past the
-- read still leaves the FIRST request's verdict on file — which is precisely
-- the one the spec says to echo.
CREATE UNIQUE INDEX IF NOT EXISTS uq_va_payment_flags_external_payment
    ON va_payment_flags (external_id, payment_request_id);

-- Supports "what happened to this VA?" during a dispute, newest first.
CREATE INDEX IF NOT EXISTS idx_va_payment_flags_va_no
    ON va_payment_flags (virtual_account_no, created_at DESC);
