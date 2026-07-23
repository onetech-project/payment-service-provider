-- va_bill_details was missing bill_code, bill_name, bill_short_name, even
-- though GetVABillDetails (internal/infrastructure/database/va_repository.go)
-- already selects them per the ASPI BillDetail schema — this migration adds
-- them so bill details can actually be persisted and read back.
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_code VARCHAR(2);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_name VARCHAR(30);
ALTER TABLE va_bill_details ADD COLUMN IF NOT EXISTS bill_short_name VARCHAR(30);
