DROP TABLE IF EXISTS va_customer_no_sequences;
DROP INDEX IF EXISTS idx_va_transactions_customer_no;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS va_type;
