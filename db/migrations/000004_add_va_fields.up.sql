-- Add SNAP-compliant columns to va_transactions
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS customer_name VARCHAR(255);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS customer_email VARCHAR(255);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS customer_phone VARCHAR(30);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS trx_id VARCHAR(64);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS expired_date TIMESTAMPTZ;
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS virtual_account_trx_type VARCHAR(1) DEFAULT 'C';
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS notification_url VARCHAR(512);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS journal_num VARCHAR(6);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS payment_type VARCHAR(1);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS flag_advise VARCHAR(1);
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS paid_bills VARCHAR(6);

-- Set NOT NULL constraints after adding columns with defaults
ALTER TABLE va_transactions ALTER COLUMN customer_name SET NOT NULL;
ALTER TABLE va_transactions ALTER COLUMN trx_id SET NOT NULL;
ALTER TABLE va_transactions ALTER COLUMN notification_url SET NOT NULL;

-- Add index for partner_service_id
CREATE INDEX IF NOT EXISTS idx_va_transactions_partner_service ON va_transactions(partner_service_id);

-- Add SNAP-compliant columns to va_bill_details
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_amount_currency VARCHAR(3);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_amount_label VARCHAR(25);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_amount_value VARCHAR(25);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS biller_reference_id VARCHAR(64);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS reason_en VARCHAR(64);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS reason_id VARCHAR(64);
