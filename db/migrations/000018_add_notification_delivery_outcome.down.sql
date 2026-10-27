DROP INDEX IF EXISTS idx_va_notification_deliveries_trigger_attempted_at;
ALTER TABLE va_notification_deliveries
    DROP COLUMN IF EXISTS last_attempt_at,
    DROP COLUMN IF EXISTS http_status,
    DROP COLUMN IF EXISTS attempt_count,
    DROP COLUMN IF EXISTS payload;
