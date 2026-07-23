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

**Request body — corrected shape**:
```json
{
  "paymentRequestId": "string",
  "paidAmount": { "value": "100000.00", "currency": "IDR" },
  "totalAmount": { "value": "100000.00", "currency": "IDR" },
  "trxDateTime": "2026-07-23T10:00:00+07:00"
}
```

- `paymentRequestId` and `paidAmount` are mandatory (per `aspi-open-api-va.yaml:195`). Missing either → `400` with `"Invalid Mandatory Field [<field>]"`.
- `totalAmount` is optional; when present, its value must equal `paidAmount.value` or the request is rejected as an amount mismatch.
- `transactionDate` is **not** a valid field for this request — it MUST NOT be required, and any value sent under that name is ignored (not part of the spec schema).

**Response body — as of Phase 6** (`virtualAccountData`, i.e. `VAPaymentStatus`): now echoes `partnerServiceId`, `customerNo`, `virtualAccountNo`, `trxId`, `paymentRequestId`, `paidAmount`, `totalAmount`, `trxDateTime`, `referenceNo` alongside `paymentFlagStatus`/`paymentFlagReason`, matching `PaymentResponse.virtualAccountData` (`aspi-open-api-va.yaml:228-254`) — previously only the flag/reason pair was returned.

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
