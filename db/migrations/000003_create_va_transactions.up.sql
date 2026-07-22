CREATE TABLE IF NOT EXISTS va_transactions (
    id VARCHAR(36) PRIMARY KEY,
    partner_service_id VARCHAR(8) NOT NULL,
    customer_no VARCHAR(20) NOT NULL,
    virtual_account_no VARCHAR(28) NOT NULL,
    inquiry_request_id VARCHAR(128) UNIQUE NOT NULL,
    payment_request_id VARCHAR(30),
    status VARCHAR(2) NOT NULL DEFAULT '03',
    total_amount NUMERIC(16,2),
    paid_amount NUMERIC(16,2),
    currency VARCHAR(3) NOT NULL DEFAULT 'IDR',
    reference_no VARCHAR(11),
    transaction_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_va_transactions_virtual_account ON va_transactions(virtual_account_no);
CREATE INDEX IF NOT EXISTS idx_va_transactions_inquiry_request ON va_transactions(inquiry_request_id);

CREATE TABLE IF NOT EXISTS va_bill_details (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES va_transactions(id),
    bill_no VARCHAR(18) NOT NULL,
    bill_description_en VARCHAR(18),
    bill_description_id VARCHAR(18),
    bill_sub_company VARCHAR(5),
    bill_amount NUMERIC(16,2),
    bill_reference_no VARCHAR(11),
    status VARCHAR(2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_va_bill_details_transaction ON va_bill_details(transaction_id);
