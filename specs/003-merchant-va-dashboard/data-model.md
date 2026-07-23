# Data Model: Merchant VA Dashboard

**Date**: 2026-07-23 | **Feature**: 003-merchant-va-dashboard

Reference: `aspi-open-api.yaml` (ASPI SNAP OpenAPI spec)

## Existing Entities (Extended)

### VA Transaction (`va_transactions` table — extended via migration 000004)

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| id | VARCHAR(36) | PK | UUID |
| partner_service_id | VARCHAR(8) | NOT NULL | SNAP: 8 digits |
| customer_no | VARCHAR(20) | NOT NULL | SNAP: up to 20 digits |
| customer_name | VARCHAR(255) | NOT NULL | SNAP: virtualAccountName (required in VAUpsertRequest) |
| customer_email | VARCHAR(255) | | SNAP: virtualAccountEmail (optional) |
| customer_phone | VARCHAR(30) | | SNAP: virtualAccountPhone (optional, format: 62xxxxxxxxxxxxx) |
| virtual_account_no | VARCHAR(28) | NOT NULL | SNAP: partnerServiceId + customerNo |
| trx_id | VARCHAR(64) | NOT NULL | SNAP: trxId (required in VAUpsertRequest) |
| inquiry_request_id | VARCHAR(128) | UNIQUE, NOT NULL | Idempotency key |
| payment_request_id | VARCHAR(128) | | Set when payment received (SNAP max 128) |
| status | VARCHAR(2) | NOT NULL, DEFAULT '03' | 00=Success, 01=Reject, 02=Expired, 03=Pending, 04=Deleted |
| total_amount | NUMERIC(16,2) | | SNAP: Amount.value (optional) |
| paid_amount | NUMERIC(16,2) | | SNAP: paidAmount |
| currency | VARCHAR(3) | NOT NULL, DEFAULT 'IDR' | SNAP: Amount.currency (ISO 4217) |
| reference_no | VARCHAR(64) | | SNAP: referenceNo (optional) |
| journal_num | VARCHAR(6) | | SNAP: journalNum (optional) |
| payment_type | VARCHAR(1) | | SNAP: paymentType (optional) |
| flag_advise | VARCHAR(1) | | SNAP: flagAdvise (optional) |
| paid_bills | VARCHAR(6) | | SNAP: paidBills hex string (optional) |
| transaction_date | TIMESTAMPTZ | | SNAP: trxDateTime (optional) |
| expired_date | TIMESTAMPTZ | | SNAP: expiredDate (optional in VAUpsertRequest) |
| virtual_account_trx_type | VARCHAR(1) | DEFAULT 'C' | SNAP: C,O,I,M,L,N,X (optional) |
| notification_url | VARCHAR(512) | NOT NULL | Merchant webhook URL (not in SNAP spec) |
| created_at | TIMESTAMPTZ | NOT NULL | Record creation |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update (maps to SNAP: lastUpdateDate) |

**New columns (migration 000004)**: `customer_name`, `customer_email`, `customer_phone`, `trx_id`, `expired_date`, `virtual_account_trx_type`, `notification_url`, `journal_num`, `payment_type`, `flag_advise`, `paid_bills`

**Indexes**: `idx_va_transactions_virtual_account`, `idx_va_transactions_inquiry_request`, `idx_va_transactions_partner_service`

### VA Bill Detail (`va_bill_details` table — extended)

Per ASPI `BillDetail` schema:

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| id | VARCHAR(36) | PK | UUID |
| transaction_id | VARCHAR(36) | FK → va_transactions(id), NOT NULL | Parent transaction |
| bill_code | VARCHAR(2) | | SNAP: billCode (optional) |
| bill_no | VARCHAR(18) | NOT NULL | SNAP: billNo (optional) |
| bill_name | VARCHAR(20) | | SNAP: billName (optional) |
| bill_short_name | VARCHAR(10) | | SNAP: billShortName (optional) |
| bill_description_en | VARCHAR(18) | | SNAP: billDescription.english |
| bill_description_id | VARCHAR(18) | | SNAP: billDescription.indonesia |
| bill_sub_company | VARCHAR(5) | | SNAP: billSubCompany (mandatory if subCompany sent) |
| bill_amount | NUMERIC(16,2) | | SNAP: billAmount.value |
| bill_amount_currency | VARCHAR(3) | | SNAP: billAmount.currency |
| bill_amount_label | VARCHAR(25) | | SNAP: billAmountLabel (optional) |
| bill_amount_value | VARCHAR(25) | | SNAP: billAmountValue (optional) |
| bill_reference_no | VARCHAR(15) | | SNAP: billReferenceNo (optional) |
| biller_reference_id | VARCHAR(64) | | SNAP: billerReferenceId (optional) |
| status | VARCHAR(2) | | SNAP: status (optional) |
| reason_en | VARCHAR(64) | | SNAP: reason.english (optional) |
| reason_id | VARCHAR(64) | | SNAP: reason.indonesia (optional) |
| created_at | TIMESTAMPTZ | NOT NULL | Record creation |

**New columns**: `bill_amount_currency`, `bill_amount_label`, `bill_amount_value`, `biller_reference_id`, `reason_en`, `reason_id`

**Indexes**: `idx_va_bill_details_transaction`

## Domain Types (Go)

### Create VA (VAUpsertRequest per ASPI)

```go
// MerchantCreateVARequest maps to ASPI VAUpsertRequest (Service Code 27)
type MerchantCreateVARequest struct {
    PartnerServiceID    string                 `json:"partnerServiceId"` // Required (from VAIdentity)
    CustomerNo          string                 `json:"customerNo"`       // Required (from VAIdentity)
    VirtualAccountNo    string                 `json:"virtualAccountNo,omitempty"` // Computed
    VirtualAccountName  string                 `json:"virtualAccountName"` // Required
    VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
    VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"`
    TrxID               string                 `json:"trxId"` // Required per ASPI
    TotalAmount         *Amount                `json:"totalAmount,omitempty"` // Optional
    BillDetails         []BillDetail           `json:"billDetails,omitempty"` // Optional, max 24
    FreeTexts           []BilingualText        `json:"freeTexts,omitempty"` // Optional, max 25
    VirtualAccountTrxType string               `json:"virtualAccountTrxType,omitempty"` // C,O,I,M,L,N,X
    FeeAmount           *Amount                `json:"feeAmount,omitempty"`
    ExpiredDate         *time.Time             `json:"expiredDate,omitempty"`
    AdditionalInfo      map[string]interface{} `json:"additionalInfo,omitempty"` // includes dbUrlProcess
    NotificationURL     string                 `json:"notificationUrl"` // Merchant-specific
}

// MerchantCreateVAResponse maps to ASPI VAUpsertResponse
type MerchantCreateVAResponse struct {
    ResponseCode    string         `json:"responseCode"` // String, max 7
    ResponseMessage string         `json:"responseMessage"` // String, max 150
    VirtualAccountData *MerchantVAData `json:"virtualAccountData,omitempty"`
}

// MerchantVAData maps to VAUpsertResponse.virtualAccountData
type MerchantVAData struct {
    PartnerServiceID    string                 `json:"partnerServiceId"`
    CustomerNo          string                 `json:"customerNo"`
    VirtualAccountNo    string                 `json:"virtualAccountNo"`
    VirtualAccountName  string                 `json:"virtualAccountName"`
    VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
    VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"`
    TrxID               string                 `json:"trxId"`
    TotalAmount         *Amount                `json:"totalAmount,omitempty"`
    BillDetails         []BillDetail           `json:"billDetails,omitempty"`
    FreeTexts           []BilingualText        `json:"freeTexts,omitempty"`
    VirtualAccountTrxType string               `json:"virtualAccountTrxType,omitempty"`
    FeeAmount           *Amount                `json:"feeAmount,omitempty"`
    ExpiredDate         *time.Time             `json:"expiredDate,omitempty"`
    LastUpdateDate      *time.Time             `json:"lastUpdateDate,omitempty"`
    PaymentDate         *time.Time             `json:"paymentDate,omitempty"`
    AdditionalInfo      map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### Delete VA (DeleteVARequest per ASPI)

```go
// MerchantDeleteVARequest maps to ASPI DeleteVARequest (Service Code 31)
type MerchantDeleteVARequest struct {
    PartnerServiceID string                 `json:"partnerServiceId"` // Required (VAIdentity)
    CustomerNo       string                 `json:"customerNo"`       // Required (VAIdentity)
    VirtualAccountNo string                 `json:"virtualAccountNo"` // Required (VAIdentity)
    TrxID            string                 `json:"trxId,omitempty"`  // Optional
    AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MerchantDeleteVAResponse maps to ASPI DeleteVAResponse
type MerchantDeleteVAResponse struct {
    ResponseCode    string                 `json:"responseCode"`
    ResponseMessage string                 `json:"responseMessage"`
    VirtualAccountData *MerchantDeleteVAData `json:"virtualAccountData,omitempty"`
}

type MerchantDeleteVAData struct {
    PartnerServiceID string                 `json:"partnerServiceId"`
    CustomerNo       string                 `json:"customerNo"`
    VirtualAccountNo string                 `json:"virtualAccountNo"`
    TrxID            string                 `json:"trxId,omitempty"`
    AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### Payment (PaymentRequest per ASPI)

```go
// PaymentNotificationPayload maps to ASPI PaymentRequest (Service Code 25)
type PaymentNotificationPayload struct {
    PartnerServiceID         string                 `json:"partnerServiceId"`
    CustomerNo               string                 `json:"customerNo"`
    VirtualAccountNo         string                 `json:"virtualAccountNo"`
    VirtualAccountName       string                 `json:"virtualAccountName,omitempty"`
    VirtualAccountEmail      string                 `json:"virtualAccountEmail,omitempty"`
    VirtualAccountPhone      string                 `json:"virtualAccountPhone,omitempty"`
    TrxID                    string                 `json:"trxId,omitempty"`
    PaymentRequestID         string                 `json:"paymentRequestId"` // Required
    PaidAmount               *Amount                `json:"paidAmount"`       // Required
    CumulativePaymentAmount  *Amount                `json:"cumulativePaymentAmount,omitempty"`
    PaidBills                string                 `json:"paidBills,omitempty"` // Hex, max 6
    TotalAmount              *Amount                `json:"totalAmount,omitempty"`
    TrxDateTime              string                 `json:"trxDateTime,omitempty"`
    ReferenceNo              string                 `json:"referenceNo,omitempty"`
    JournalNum               string                 `json:"journalNum,omitempty"`
    PaymentType              string                 `json:"paymentType,omitempty"`
    FlagAdvise               string                 `json:"flagAdvise,omitempty"`
    BillDetails              []BillDetail           `json:"billDetails,omitempty"`
    FreeTexts                []BilingualText        `json:"freeTexts,omitempty"`
    AdditionalInfo           map[string]interface{} `json:"additionalInfo,omitempty"`
    NotificationURL          string                 `json:"notificationUrl"` // Merchant-specific
}
```

### BillDetail (per ASPI)

```go
// BillDetail maps to ASPI BillDetail schema
type BillDetail struct {
    BillCode        string                 `json:"billCode,omitempty"`
    BillNo          string                 `json:"billNo"`
    BillName        string                 `json:"billName,omitempty"`
    BillShortName   string                 `json:"billShortName,omitempty"`
    BillDescription *BilingualText         `json:"billDescription,omitempty"`
    BillSubCompany  string                 `json:"billSubCompany,omitempty"`
    BillAmount      *Amount                `json:"billAmount,omitempty"`
    BillAmountLabel string                 `json:"billAmountLabel,omitempty"`
    BillAmountValue string                 `json:"billAmountValue,omitempty"`
    BillReferenceNo string                 `json:"billReferenceNo,omitempty"`
    BillerReferenceID string               `json:"billerReferenceId,omitempty"`
    Status          string                 `json:"status,omitempty"`
    Reason          *BilingualText         `json:"reason,omitempty"`
    AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}
```

### Shared Value Objects (per ASPI)

```go
type Amount struct {
    Value    string `json:"value"`    // String, format: "12345678.00"
    Currency string `json:"currency"` // ISO 4217, e.g., "IDR"
}

type BilingualText struct {
    English   string `json:"english"`
    Indonesia string `json:"indonesia"`
}

type Pagination struct {
    Page       int `json:"page"`
    PageSize   int `json:"pageSize"`
    TotalRows  int `json:"totalRows"`
    TotalPages int `json:"totalPages"`
}
```

## State Transitions

### VA Transaction Status

```
[03: Pending] ──payment received──→ [00: Success]
[03: Pending] ──deleted───────────→ [04: Deleted]
[03: Pending] ──expired───────────→ [02: Expired]
[00: Success] (terminal)
[01: Reject]  (terminal)
[02: Expired] (terminal)
[04: Deleted] (terminal)
```

### Deletion Rules (SNAP service code 31)

| Current Status | Can Delete? | Result |
|---------------|-------------|--------|
| 03 (Pending) | Yes | → 04 (Deleted), response `2003100` |
| 00 (Success) | No | Error: SNAP 405/01 |
| 02 (Expired) | No | Error: SNAP 405/01 |
| 04 (Deleted) | Yes (idempotent) | Returns current status |

## Validation Rules (from ASPI OpenAPI)

- `partnerServiceId`: String, max 8 chars
- `customerNo`: String, max 20 chars
- `virtualAccountNo`: String, max 28 chars
- `virtualAccountName`: String (required in VAUpsertRequest)
- `trxId`: String, max 64 chars (required in VAUpsertRequest)
- `paymentRequestId`: String, max 128 chars (required in PaymentRequest)
- `responseCode`: String, max 7 chars
- `responseMessage`: String, max 150 chars
- `paidBills`: String, max 6 chars (hex)
- `amount.value`: String, format "12345678.00"
- `amount.currency`: String, ISO 4217
- `virtualAccountTrxType`: String, enum [C,O,I,M,L,N,X]
- `billDetails`: Array, max 24 items
- `freeTexts`: Array, max 25 items
