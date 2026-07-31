# Contract Delta: SNAP VA Inquiry & Payment Endpoints

Source of truth: `aspi-open-api-va.yaml` (unchanged by this feature — the contract already documents correct behavior; this file records what the *implementation* must now match).

## POST /transfer-va/inquiry (Service Code 24)

**Request body — corrected shape**:
```json
{
  "partnerServiceId": "string",
  "customerNo": "string",
  "virtualAccountNo": "string",
  "txnDateInit": "2026-07-23T10:00:00+07:00",
  "amount": { "value": "100000.00", "currency": "IDR" },
  "channelCode": 6011,
  "inquiryRequestId": "string",
  "additionalInfo": {}
}
```

- `inquiryRequestId` and `amount` are mandatory (per `aspi-open-api-va.yaml:150`). Missing either → `400` with `"Invalid Mandatory Field [<field>]"`.
- `txnDateInit` (not `trxDateInit`) is the correct field name for the transaction-init timestamp.

**Response body — as of Phase 6** (`virtualAccountData`): if the `virtualAccountNo` was previously created via `create-va`, the response now also echoes `billDetails` (read back from `va_bill_details`, see data-model.md Addendum) alongside the existing `partnerServiceId`/`customerNo`/`virtualAccountNo`/`virtualAccountName`/`inquiryRequestId`/`totalAmount` fields. Repeated inquiries against the same `virtualAccountNo` no longer create duplicate `va_transactions` rows.

## POST /transfer-va/payment (Service Code 25)

**Request body — corrected shape, as of Phase 7**:
```json
{
  "partnerServiceId": "string",
  "customerNo": "string",
  "virtualAccountNo": "string",
  "virtualAccountName": "string",
  "virtualAccountEmail": "string",
  "virtualAccountPhone": "string",
  "trxId": "string",
  "paymentRequestId": "string",
  "channelCode": 6011,
  "hashedSourceAccountNo": "string",
  "sourceBankCode": "string",
  "paidAmount": { "value": "100000.00", "currency": "IDR" },
  "cumulativePaymentAmount": { "value": "100000.00", "currency": "IDR" },
  "paidBills": "string",
  "totalAmount": { "value": "100000.00", "currency": "IDR" },
  "trxDateTime": "2026-07-23T10:00:00+07:00",
  "referenceNo": "string",
  "journalNum": "string",
  "paymentType": "1",
  "flagAdvise": "Y",
  "subCompany": "string",
  "billDetails": [{ "billNo": "string", "billAmount": { "value": "100000.00", "currency": "IDR" } }],
  "freeTexts": [{ "english": "string", "indonesia": "string" }]
}
```

- **Mandatory (as of Phase 7)**: `trxId`, `paymentRequestId`, `paidAmount`. `inquiryRequestId` is
  still accepted (optional, `omitempty`) for backward compatibility, but is no longer the
  vendor-facing mandatory identifier — the ASPI sample sends `trxId` on this endpoint, not
  `inquiryRequestId`.
- `totalAmount` is optional; when present, its value must equal `paidAmount.value` or the request is rejected as an amount mismatch.
- `transactionDate` is **not** a valid field for this request — it MUST NOT be required, and any value sent under that name is ignored (not part of the spec schema).
- `channelCode`, `hashedSourceAccountNo`, `sourceBankCode`, `cumulativePaymentAmount`, `journalNum`,
  `subCompany`, `virtualAccountName/Email/Phone` are optional pass-through fields, persisted but not
  echoed back except where noted below. `billDetails`, when provided, are persisted (same
  `va_bill_details` mechanism as `create-va`) and echoed back in the response. `freeTexts`, when
  provided, are persisted (`free_texts` JSONB column) and echoed back verbatim.

**Response body — as of Phase 7** (`virtualAccountData`, i.e. `VAPaymentStatus`): echoes
`partnerServiceId`, `customerNo`, `virtualAccountNo`, `virtualAccountName`, `virtualAccountEmail`,
`virtualAccountPhone`, `trxId`, `paymentRequestId`, `paidAmount`, `paidBills`, `totalAmount`,
`trxDateTime`, `referenceNo`, `journalNum`, `paymentType`, `flagAdvise`, `billDetails`, `freeTexts`
alongside `paymentFlagStatus`/`paymentFlagReason`, matching `PaymentResponse.virtualAccountData`
(`aspi-open-api-va.yaml:228-254`) — Phase 6 only added the identity/amount fields; Phase 7 fills in
the remaining echo set plus `billDetails`/`freeTexts`.

## POST /transfer-va/status (Service Code 29) — added in Phase 7

**Request body**:
```json
{
  "partnerServiceId": "string",
  "customerNo": "string",
  "virtualAccountNo": "string",
  "inquiryRequestId": "string",
  "paymentRequestId": "string",
  "additionalInfo": {}
}
```

- `partnerServiceId`, `customerNo`, `virtualAccountNo`, `inquiryRequestId` are mandatory (existing
  behavior, unchanged). `paymentRequestId` is a new, optional field — present in the ASPI sample but
  `inquiryRequestId` remains the primary lookup key (`GetPayment(ctx, req.InquiryRequestID)`, whose
  underlying query also matches on `payment_request_id`).

**Response body** (`virtualAccountData`, i.e. `VAStatusData`) — fixed/completed in Phase 7:
- **Bug fix**: `totalAmount` previously echoed `paidAmount`'s value (aliased at both write time in
  `SavePayment` and read time in `Status()`); now reflects the actual bill total, sourced from the
  new `VAPaymentRecord.TotalAmount` column.
- **Newly populated** (fields existed on the struct but `Status()` never set them): `trxDateTime`
  (from the new `trx_date_time` column — the vendor's own transaction timestamp, distinct from
  `transactionDate`, the settlement time), `paymentType`, `paidBills`, `billDetails` (read back via
  `GetVABillDetails`, for both the paid and pending/inquiry-only response branches), `freeTexts`
  (read back from the same `free_texts` column `/payment` writes).
- **New field**: `flagAdvise` (didn't exist on `VAStatusData` before Phase 7).

## POST /transfer-va/create-va (Service Code 27) — added in Phase 6

**Request body — corrected shape**:
```json
{
  "partnerServiceId": "string",
  "customerNo": "string",
  "virtualAccountNo": "string",
  "virtualAccountName": "string",
  "trxId": "string",
  "totalAmount": { "value": "100000.00", "currency": "IDR" },
  "billDetails": [{ "billNo": "string", "billAmount": { "value": "100000.00", "currency": "IDR" } }],
  "additionalInfo": { "dbUrlProcess": "https://merchant.example.com/callback" }
}
```

- `partnerServiceId`, `customerNo`, `virtualAccountNo` (per `VAIdentity.required`) and `virtualAccountName`, `trxId` (per `VAUpsertRequest.required`, `aspi-open-api-va.yaml:301`) are mandatory. `virtualAccountNo` is **client-supplied** — the server no longer overwrites it with a self-generated `partnerServiceId+customerNo` value.
- There is no top-level `notificationUrl` field in the spec. The merchant payment-callback URL is a proprietary extension carried in `additionalInfo.dbUrlProcess` (the spec's own extension slot for this endpoint, `aspi-open-api-va.yaml:317-320`) — optional.
- `billDetails`, when provided, are persisted to `va_bill_details` and returned by both this response and subsequent `inquiry` calls against the same `virtualAccountNo`.
- A `virtualAccountNo` is reusable: `create-va` only rejects with `4092700` Conflict when that number currently has a **pending** (`status = "03"`, i.e. unpaid) transaction. Once paid/expired/deleted, the same number may start a new transaction cycle.

This is a behavior-contract fix, not a new contract — no new endpoints or schema versions are introduced.
