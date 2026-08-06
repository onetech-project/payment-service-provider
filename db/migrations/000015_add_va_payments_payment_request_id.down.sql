DROP INDEX IF EXISTS uq_va_payments_payment_request_id;
ALTER TABLE va_payments DROP COLUMN IF EXISTS payment_request_id;
