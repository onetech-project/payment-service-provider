-- Drop SNAP-compliant columns from va_bill_details
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS reason_id;
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS reason_en;
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS biller_reference_id;
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS bill_amount_value;
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS bill_amount_label;
ALTER TABLE va_bill_details DROP COLUMN IF EXISTS bill_amount_currency;

-- Drop index
DROP INDEX IF EXISTS idx_va_transactions_partner_service;

-- Drop SNAP-compliant columns from va_transactions
ALTER TABLE va_transactions DROP COLUMN IF EXISTS paid_bills;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS flag_advise;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS payment_type;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS journal_num;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS notification_url;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS virtual_account_trx_type;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS expired_date;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS trx_id;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS customer_phone;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS customer_email;
ALTER TABLE va_transactions DROP COLUMN IF EXISTS customer_name;
