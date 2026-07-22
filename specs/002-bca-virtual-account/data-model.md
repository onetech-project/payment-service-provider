# Data Model: BCA Virtual Account Integration

**Feature**: 002-bca-virtual-account
**Date**: 2026-07-22

## Domain Entities

### VA Inquiry Request (Inbound from BCA)

```go
// VAInquiryRequest represents inbound inquiry from BCA
type VAInquiryRequest struct {
    PartnerServiceID  string                 `json:"partnerServiceId"`   // 8 chars, left-padded
    CustomerNo        string                 `json:"customerNo"`         // up to 20 digits
    VirtualAccountNo  string                 `json:"virtualAccountNo"`   // partnerServiceId + customerNo
    TrxDateInit       *time.Time             `json:"trxDateInit,omitempty"`
    ChannelCode       int                    `json:"channelCode,omitempty"` // ISO 18245
    InquiryRequestID  string                 `json:"inquiryRequestId"`   // unique per transaction
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### VA Inquiry Response (Outbound to BCA)

```go
// VAInquiryResponse represents response to BCA inquiry
type VAInquiryResponse struct {
    ResponseCode    string             `json:"responseCode"`
    ResponseMessage string             `json:"responseMessage"`
    VirtualAccountData *VAAccountData  `json:"virtualAccountData,omitempty"`
}

// VAAccountData contains VA account and bill information
type VAAccountData struct {
    InquiryStatus     string            `json:"inquiryStatus"`     // "00"=success, "01"=failed
    InquiryReason     *BilingualText    `json:"inquiryReason,omitempty"`
    PartnerServiceID  string            `json:"partnerServiceId"`
    CustomerNo        string            `json:"customerNo"`
    VirtualAccountNo  string            `json:"virtualAccountNo"`
    VirtualAccountName string           `json:"virtualAccountName"`
    InquiryRequestID  string            `json:"inquiryRequestId"`
    TotalAmount       *Amount           `json:"totalAmount,omitempty"`
    SubCompany        string            `json:"subCompany,omitempty"`
    BillDetails       []BillDetail      `json:"billDetails,omitempty"`
    FreeTexts         []BilingualText   `json:"freeTexts,omitempty"`
    VirtualAccountTrxType string        `json:"virtualAccountTrxType,omitempty"`
    FeeAmount         *Amount           `json:"feeAmount,omitempty"`
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### VA Payment Notification (Inbound from BCA)

```go
// VAPaymentRequest represents inbound payment notification from BCA
type VAPaymentRequest struct {
    PartnerServiceID  string                 `json:"partnerServiceId"`
    CustomerNo        string                 `json:"customerNo"`
    VirtualAccountNo  string                 `json:"virtualAccountNo"`
    InquiryRequestID  string                 `json:"inquiryRequestId"`
    PaymentRequestID  string                 `json:"paymentRequestId"`
    PaidAmount        *Amount                `json:"paidAmount"`
    PaidBills         string                 `json:"paidBills,omitempty"` // hex flag
    TotalAmount       *Amount                `json:"totalAmount"`
    TrxDateTime       *time.Time             `json:"trxDateTime,omitempty"`
    TransactionDate   *time.Time             `json:"transactionDate"`
    ReferenceNo       string                 `json:"referenceNo,omitempty"`
    PaymentType       string                 `json:"paymentType,omitempty"`
    FlagAdvise        string                 `json:"flagAdvise,omitempty"`
    BillDetails       []VAPaymentBillDetail  `json:"billDetails,omitempty"`
    FreeTexts         []BilingualText        `json:"freeTexts,omitempty"`
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VAPaymentBillDetail extends BillDetail with payment-specific fields
type VAPaymentBillDetail struct {
    BillCode          string            `json:"billCode,omitempty"`
    BillNo            string            `json:"billNo"`
    BillName          string            `json:"billName,omitempty"`
    BillShortName     string            `json:"billShortName,omitempty"`
    BillDescription   *BilingualText    `json:"billDescription"`
    BillSubCompany    string            `json:"billSubCompany"`
    BillAmount        *Amount           `json:"billAmount"`
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
    BillReferenceNo   string            `json:"billReferenceNo"`
    Status            string            `json:"status"`
    Reason            *BilingualText    `json:"reason"`
}
```

### VA Payment Response (Outbound to BCA)

```go
// VAPaymentResponse represents response to BCA payment notification
type VAPaymentResponse struct {
    ResponseCode    string              `json:"responseCode"`
    ResponseMessage string              `json:"responseMessage"`
    VirtualAccountData *VAPaymentStatus `json:"virtualAccountData,omitempty"`
}

// VAPaymentStatus contains payment flag status
type VAPaymentStatus struct {
    PaymentFlagStatus string         `json:"paymentFlagStatus"` // "00"=success, "01"=reject, "02"=timeout, "03"=pending
    PaymentFlagReason *BilingualText `json:"paymentFlagReason"`
}
```

### VA Status Inquiry Request (Inbound from BCA)

```go
// VAStatusRequest represents inbound status inquiry from BCA
type VAStatusRequest struct {
    PartnerServiceID  string                 `json:"partnerServiceId"`
    CustomerNo        string                 `json:"customerNo"`
    VirtualAccountNo  string                 `json:"virtualAccountNo"`
    InquiryRequestID  string                 `json:"inquiryRequestId"`
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### VA Status Inquiry Response (Outbound to BCA)

```go
// VAStatusResponse represents response to BCA status inquiry
type VAStatusResponse struct {
    ResponseCode    string             `json:"responseCode"`
    ResponseMessage string             `json:"responseMessage"`
    VirtualAccountData *VAStatusData   `json:"virtualAccountData,omitempty"`
}

// VAStatusData contains status inquiry result
type VAStatusData struct {
    PaymentFlagStatus string         `json:"paymentFlagStatus"`
    PaymentFlagReason *BilingualText `json:"paymentFlagReason"`
    PartnerServiceID  string         `json:"partnerServiceId"`
    CustomerNo        string         `json:"customerNo"`
    VirtualAccountNo  string         `json:"virtualAccountNo"`
    InquiryRequestID  string         `json:"inquiryRequestId"`
    PaymentRequestID  string         `json:"paymentRequestId"`
    PaidAmount        *Amount        `json:"paidAmount"`
    PaidBills         string         `json:"paidBills,omitempty"`
    TotalAmount       *Amount        `json:"totalAmount"`
    TrxDateTime       *time.Time     `json:"trxDateTime,omitempty"`
    TransactionDate   *time.Time     `json:"transactionDate"`
    ReferenceNo       string         `json:"referenceNo,omitempty"`
    PaymentType       string         `json:"paymentType,omitempty"`
    BillDetails       []VAStatusBillDetail `json:"billDetails,omitempty"`
    FreeTexts         []BilingualText `json:"freeTexts,omitempty"`
    AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

## Shared Value Objects

```go
// Amount represents monetary amount with currency
type Amount struct {
    Value    string `json:"value"`    // ISO 4217 format, 2 decimals
    Currency string `json:"currency"` // IDR, USD, SGD
}

// BilingualText represents text in English and Indonesian
type BilingualText struct {
    English   string `json:"english"`
    Indonesia string `json:"indonesia"`
}

// BillDetail represents a single bill item
type BillDetail struct {
    BillNo          string         `json:"billNo"`
    BillDescription *BilingualText `json:"billDescription,omitempty"`
    BillSubCompany  string         `json:"billSubCompany,omitempty"`
    BillAmount      *Amount        `json:"billAmount,omitempty"`
    AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

## Configuration Entity

```go
// BCAVAConfig holds BCA Virtual Account configuration
type BCAVAConfig struct {
    // Authentication
    ClientID       string
    ClientSecret   string
    PrivateKeyPath string
    PublicKeyPath  string

    // Endpoints
    BaseURL          string
    TokenEndpoint    string
    InquiryEndpoint  string
    StatusEndpoint   string
    PaymentEndpoint  string

    // Channel
    ChannelID    string
    PartnerID    string
    Origin       string

    // Request
    RequestTimeout      int
    SignatureAlgorithm  string

    // VA Defaults
    DefaultSubCompany  string
    DefaultChannelCode int

    // Logging
    DebugLogging      bool
    CorrelationHeader string
}
```

## Repository Interfaces

```go
// VARepository defines persistence operations for VA transactions
type VARepository interface {
    SaveInquiry(ctx context.Context, inquiry *VAInquiryRecord) error
    GetInquiry(ctx context.Context, inquiryRequestID string) (*VAInquiryRecord, error)
    SavePayment(ctx context.Context, payment *VAPaymentRecord) error
    GetPayment(ctx context.Context, paymentRequestID string) (*VAPaymentRecord, error)
    UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error
}

// VAInquiryRecord represents a persisted inquiry
type VAInquiryRecord struct {
    ID               string
    PartnerServiceID string
    CustomerNo       string
    VirtualAccountNo string
    InquiryRequestID string
    Status           string
    TotalAmount      string
    Currency         string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// VAPaymentRecord represents a persisted payment
type VAPaymentRecord struct {
    ID               string
    PartnerServiceID string
    CustomerNo       string
    VirtualAccountNo string
    InquiryRequestID string
    PaymentRequestID string
    PaidAmount       string
    Currency         string
    Status           string
    ReferenceNo      string
    TransactionDate  time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

## Gateway Interface

```go
// VAGateway defines outbound BCA VA API operations
type VAGateway interface {
    Inquiry(ctx context.Context, req *VAInquiryRequest) (*VAInquiryResponse, error)
    PaymentStatus(ctx context.Context, req *VAStatusRequest) (*VAStatusResponse, error)
}
```

## State Transitions

### Payment Flag Status

```
[Initial] --> "03" (Pending) --> "00" (Success)
                             --> "01" (Reject)
                             --> "02" (Timeout)
```

### Inquiry Status

```
[Initial] --> "00" (Success - bill returned)
           --> "01" (Failed - reason required)
```

## Database Tables (New)

### va_transactions

```sql
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
```

### va_bill_details

```sql
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
```
