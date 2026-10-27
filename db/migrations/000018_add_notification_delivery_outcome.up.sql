-- va_notification_deliveries recorded only whether a callback was ENQUEUED:
-- "success" meant "handed to Asynq", and what the worker then saw — a merchant
-- timeout, a 5xx, retries running out — was never written back. A callback
-- that never reached the merchant looked identical to one that did.
--
-- The worker now updates the row it was enqueued for (Asynq task header
-- carries the row id): status moves queued -> retrying -> success | failed,
-- with the attempt count, the merchant's HTTP status and the error. Rows
-- written before this migration keep their old "success" (= enqueued).
--
-- payload is the exact callback that was enqueued, so a bulk resend
-- redelivers that event's own data instead of rebuilding it from whichever
-- transaction cycle of a reused VA happens to be current. NULL on older rows,
-- which bulk resend therefore skips rather than guess.
ALTER TABLE va_notification_deliveries
    ADD COLUMN IF NOT EXISTS payload JSONB,
    ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS http_status INT,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ;

-- Bulk resend selects original (trigger='auto') callbacks by time range.
CREATE INDEX IF NOT EXISTS idx_va_notification_deliveries_trigger_attempted_at
    ON va_notification_deliveries (trigger, attempted_at);
