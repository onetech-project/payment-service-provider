-- Audit trail of merchant notification delivery attempts (auto-triggered
-- expiry callbacks and manual admin resends), feature
-- 007-merchant-expiry-callback. Purely additive: does not alter
-- va_transactions.
CREATE TABLE IF NOT EXISTS va_notification_deliveries (
    id VARCHAR(36) PRIMARY KEY,
    virtual_account_no VARCHAR(28) NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    trigger VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    error_detail TEXT
);

CREATE INDEX IF NOT EXISTS idx_va_notification_deliveries_va_no ON va_notification_deliveries(virtual_account_no);
CREATE INDEX IF NOT EXISTS idx_va_notification_deliveries_va_no_event_trigger ON va_notification_deliveries(virtual_account_no, event_type, trigger);
