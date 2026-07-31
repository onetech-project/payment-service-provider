ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS channel_code VARCHAR(10);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS hashed_source_account_no VARCHAR(64);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS source_bank_code VARCHAR(10);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS sub_company VARCHAR(10);
