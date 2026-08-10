-- Audit trail for outbound vendor status inquiries
-- (feature 014-vendor-status-reconciliation).
--
-- This table already existed in at least one deployed database without a
-- corresponding migration, so a fresh environment built from db/migrations/
-- did not have it while an older one did. The definition here matches the
-- deployed shape exactly and is written IF NOT EXISTS, so applying it is a
-- no-op where the table is already present and a repair everywhere else.
--
-- Why an audit table at all: a reconciler that silently stops working looks
-- identical to one with nothing to reconcile — both produce no callbacks and
-- no errors. Recording every attempt, including the ones that concluded
-- "nothing to do", is what makes the difference visible.
CREATE TABLE IF NOT EXISTS va_status_inquiry_attempts (
    id VARCHAR(36) PRIMARY KEY,
    virtual_account_no VARCHAR(28) NOT NULL,
    -- The vendor asked, so the trail stays readable once more than one vendor
    -- exposes a status service.
    client_id VARCHAR(64) NOT NULL,
    payment_request_id VARCHAR(30),
    -- settled | pending | not_paid | ambiguous | already_settled | error
    -- (domain.ReconcileOutcome* constants).
    outcome VARCHAR(32) NOT NULL,
    -- The vendor's own answer, kept raw alongside our interpretation of it:
    -- an outcome is a judgement, these are the evidence for it.
    bca_response_code VARCHAR(7),
    bca_payment_flag_status VARCHAR(2),
    duration_ms INTEGER,
    error_detail TEXT,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Supports "what happened to this VA?" when handling a customer complaint.
CREATE INDEX IF NOT EXISTS idx_va_status_inquiry_attempts_va_no
    ON va_status_inquiry_attempts(virtual_account_no);

-- Supports "is the reconciler still running, and what is it concluding?" —
-- the per-vendor operational view, newest first.
CREATE INDEX IF NOT EXISTS idx_va_status_inquiry_attempts_client_time
    ON va_status_inquiry_attempts(client_id, attempted_at DESC);
