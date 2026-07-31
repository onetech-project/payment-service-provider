ALTER TABLE va_transactions DROP COLUMN IF EXISTS channel_code;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS hashed_source_account_no;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS source_bank_code;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS sub_company;
