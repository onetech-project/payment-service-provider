# Feature Specification: Merchant VA Dashboard

**Feature Branch**: `003-merchant-va-dashboard`

**Created**: 2026-07-22

**Status**: Draft

**Input**: User description: "Endpoint untuk create VA transactions untuk dashboard internal merchant, inquiry untuk list transaksi, dan notifikasi ketika status transaksi sudah dibayar. Format endpoint, request, dan response sesuai SNAP VA API."

## Clarifications

### Session 2026-07-23

- Q: How should merchant notification webhook be configured? → A: Merchant sends webhook URL in the create VA request payload (per-transaction), not via manual config.
- Q: What is the correct create VA endpoint path? → A: `/v1.0/transfer-va/create-va` (SNAP service code 27)
- Q: Should VA transactions have expiry and cancellation support? → A: Yes, VA transactions must have expiry date (`expiredDate`) and merchants can delete pending transactions via `DELETE /v1.0/transfer-va/delete-va` (SNAP service code 31).

### Session 2026-07-23 (SNAP ASPI Alignment)

- Q: Verify all endpoints against SNAP ASPI documentation? → A: Aligned with ASPI OpenAPI spec (`aspi-open-api.yaml`). Key corrections: cancel-va → delete-va (DELETE method, service code 31), added missing SNAP fields, response codes now include service code prefix.

### Session 2026-07-23 (ASPI OpenAPI YAML Verification)

- Q: Verify all request/response schemas against aspi-open-api.yaml? → A: Corrected per YAML: `trxId` is REQUIRED in Create VA (VAUpsertRequest), `totalAmount`/`billDetails`/`expiredDate`/`virtualAccountTrxType` are OPTIONAL, added missing BillDetail fields (`billAmountLabel`, `billAmountValue`, `billerReferenceId`, `reason`), Payment response includes `cumulativePaymentAmount`/`paidBills`/`journalNum`/`paymentType`/`flagAdvise`, `CHANNEL-ID` header is OPTIONAL, `responseCode` is STRING (max 7 chars), Inquiry uses `txnDateInit` (not `trxDateInit`).

## SNAP VA API Reference (ASPI OpenAPI)

| Service Code | Name | Method | Path | Response Code Prefix |
|-------------|------|--------|------|---------------------|
| 24 | Inquiry | POST | `transfer-va/inquiry` | 2002400 |
| 25 | Payment | POST | `transfer-va/payment` | 2002500 |
| 26 | Inquiry Status | POST | `transfer-va/status` | 2002600 |
| 27 | Create VA | POST | `transfer-va/create-va` | 2002700 |
| 28 | Update VA | PUT | `transfer-va/update-va` | 2002800 |
| 29 | Update Status VA | PUT | `transfer-va/update-status` | 2002900 |
| 30 | Inquiry VA | POST | `transfer-va/inquiry-va` | 2003000 |
| 31 | Delete VA | DELETE | `transfer-va/delete-va` | 2003100 |

## SNAP Headers (from ASPI OpenAPI)

| Header | Required | Description |
|--------|----------|-------------|
| X-TIMESTAMP | Yes | ISO 8601 datetime |
| X-SIGNATURE | Yes | HMAC-SHA512 signature |
| X-PARTNER-ID | Yes | Partner identifier |
| X-EXTERNAL-ID | Yes | External ID for idempotency |
| X-ORIGIN | No | Origin hostname |
| X-IP-ADDRESS | No | Client IP address |
| X-DEVICE-ID | No | Client device ID |
| X-LATITUDE | No | Latitude |
| X-LONGITUDE | No | Longitude |
| CHANNEL-ID | No | Channel identifier |
| Authorization | Yes | Bearer token (B2B) |
| Authorization-Customer | No | Customer token (if needed) |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create VA Transaction (Priority: P1)

As a merchant admin, I want to create Virtual Account transactions through the internal dashboard so that customers can pay their bills via VA.

**Why this priority**: Creating VA transactions is the core function that enables the entire payment flow. Without this, merchants cannot generate VA numbers for customers.

**Independent Test**: Can be fully tested by sending a create VA request with valid SNAP fields, and verifying the VA number is generated and stored in the database.

**Acceptance Scenarios**:

1. **Given** a merchant admin sends a create VA request with required fields (`partnerServiceId`, `customerNo`, `virtualAccountName`, `trxId`) and optional fields (`totalAmount`, `billDetails`, `expiredDate`, `virtualAccountTrxType`, `feeAmount`, `freeTexts`, `notificationUrl`), **When** the system receives the request, **Then** the system generates a VA number (`partnerServiceId + customerNo`), stores the transaction, and returns success with response code `2002700` and `VAUpsertResponse` data.
2. **Given** a merchant admin sends a duplicate `trxId`, **When** the system processes the request, **Then** the system returns the existing VA transaction without creating a duplicate (idempotent).
3. **Given** a merchant admin enters invalid field format, **When** the system validates the request, **Then** the system returns SNAP error response code `4002700` (Invalid Field Format) or `4002701` (Invalid Mandatory Field).
4. **Given** a merchant admin creates a VA with an `expiredDate`, **When** the VA expires before payment, **Then** the system marks the transaction as expired (status "02") and the VA can no longer be paid.

---

### User Story 2 - Inquiry VA Transactions (Priority: P2)

As a merchant admin, I want to view a list of VA transactions in the dashboard so that I can monitor payment status and track transactions.

**Why this priority**: Merchants need visibility into their VA transactions to reconcile payments and handle customer inquiries.

**Independent Test**: Can be fully tested by querying transactions with filters (date range, status, VA number) and verifying the list returns correct results.

**Acceptance Scenarios**:

1. **Given** a merchant admin requests a list of transactions, **When** the system processes the request, **Then** the system returns a paginated list of VA transactions with status.
2. **Given** a merchant admin filters by date range, **When** the system processes the request, **Then** the system returns only transactions within the specified period.
3. **Given** a merchant admin filters by payment status, **When** the system processes the request, **Then** the system returns only transactions matching the status.

**Note**: The list endpoint is a merchant dashboard convenience API. SNAP standard provides `inquiry-va` (service code 30, `InquiryVARequest`) for single VA lookup by `partnerServiceId`, `customerNo`, `virtualAccountNo`, and optional `trxId`.

---

### User Story 3 - Payment Notification (Priority: P3)

As a merchant admin, I want to receive notifications when a VA transaction is paid so that I can update the customer's billing status.

**Why this priority**: Real-time payment notifications enable merchants to process orders, deliver services, and reconcile accounts immediately.

**Independent Test**: Can be fully tested by simulating a payment notification and verifying the merchant receives the notification at the webhook URL provided during VA creation.

**Acceptance Scenarios**:

1. **Given** a customer makes a payment at the bank, **When** the system receives the payment notification (SNAP `PaymentRequest`), **Then** the system updates the transaction status and sends a webhook notification to the URL provided during VA creation.
2. **Given** a payment notification is received for a transaction, **When** the system processes the notification, **Then** the transaction status changes from "pending" (03) to "paid" (00).
3. **Given** multiple payments for the same VA number, **When** the system processes the notifications, **Then** each payment is recorded separately with correct amounts.

---

### User Story 4 - Delete VA Transaction (Priority: P4)

As a merchant admin, I want to delete a pending VA transaction so that the VA can no longer be paid by the customer.

**Why this priority**: Merchants need the ability to delete VA transactions that are no longer valid (e.g., order cancelled, customer changed payment method). Follows SNAP `delete-va` API (service code 31).

**Independent Test**: Can be fully tested by creating a VA transaction, then sending a delete request, and verifying the transaction status changes and the VA can no longer accept payments.

**Acceptance Scenarios**:

1. **Given** a merchant admin has a pending VA transaction, **When** the system receives a delete request (`DELETE /v1.0/transfer-va/delete-va`) with `partnerServiceId`, `customerNo`, `virtualAccountNo`, **Then** the system updates the transaction status to "deleted" and returns success with response code `2003100`.
2. **Given** a merchant admin tries to delete an already paid VA transaction, **When** the system processes the request, **Then** the system returns an error (SNAP 405/01).
3. **Given** a merchant admin tries to delete an already deleted VA transaction, **When** the system processes the request, **Then** the system returns the existing status (idempotent).

---

### Edge Cases

- What happens when a VA number is expired before payment? → Transaction status changes to "expired" (02), VA can no longer be paid. SNAP response code 404/19.
- How does the system handle partial payments for multi-bill transactions? → Depends on `virtualAccountTrxType` (I=Partial, M=Minimum, L=Maximum, N=Open Minimum, X=Open Maximum).
- What happens when the same VA number receives concurrent payment notifications? → Redis lock via idempotency middleware prevents concurrent processing.
- How does the system handle network timeouts when calling external bank APIs? → Timeout propagation via context, Asynq retry for async operations.
- What happens when a merchant tries to create a VA with an invalid amount format? → Validate at usecase level, return 400/00 (Invalid Field Format).
- What happens when the merchant webhook URL is unreachable? → Retry 3 times with exponential backoff (10s, 30s, 60s), then log failure.
- What happens when a delete request is received for an expired transaction? → Return error indicating terminal state (SNAP 405/01).
- What happens when `trxId` is missing in Create VA? → Return SNAP error 400/01 (Invalid Mandatory Field), since `trxId` is required per ASPI spec.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose `POST /v1.0/transfer-va/create-va` endpoint (SNAP service code 27) accepting `VAUpsertRequest` body. Required fields: `partnerServiceId`, `customerNo`, `virtualAccountName`, `trxId`. Optional: `totalAmount`, `billDetails`, `freeTexts`, `virtualAccountTrxType`, `feeAmount`, `expiredDate`, `additionalInfo`.
- **FR-002**: System MUST expose `POST /v1.0/transfer-va/list` endpoint to query VA transactions (merchant dashboard convenience API, queries internal database).
- **FR-003**: System MUST receive and process payment notifications from banks via `POST /v1.0/transfer-va/payment` (SNAP service code 25) accepting `PaymentRequest` body. Required: `paymentRequestId`, `paidAmount`. Optional: `virtualAccountName`, `virtualAccountEmail`, `virtualAccountPhone`, `trxId`, `channelCode`, `hashedSourceAccountNo`, `sourceBankCode`, `cumulativePaymentAmount`, `paidBills`, `totalAmount`, `trxDateTime`, `referenceNo`, `journalNum`, `paymentType`, `flagAdvise`, `subCompany`, `billDetails`, `freeTexts`, `additionalInfo`.
- **FR-004**: System MUST store VA transactions in the database with all SNAP fields including `notificationUrl` (merchant-specific, not in SNAP spec).
- **FR-005**: System MUST generate unique VA numbers following the SNAP format: `partnerServiceId` (8 digits) + `customerNo` (up to 20 digits).
- **FR-006**: System MUST support both single bill and multi-bill transactions (up to 24 bill items per SNAP spec). BillDetail fields per ASPI: `billCode`, `billNo`, `billName`, `billShortName`, `billDescription` (MultiLangText), `billSubCompany`, `billAmount` (Amount), `billAmountLabel`, `billAmountValue`, `billReferenceNo`, `billerReferenceId`, `status`, `reason` (MultiLangText), `additionalInfo`.
- **FR-007**: System MUST notify merchants when payments are received by sending a POST webhook to the `notificationUrl` provided during VA creation.
- **FR-008**: System MUST validate SNAP headers: `X-TIMESTAMP` (required), `X-SIGNATURE` (required), `X-PARTNER-ID` (required), `X-EXTERNAL-ID` (required). Optional: `X-ORIGIN`, `X-IP-ADDRESS`, `X-DEVICE-ID`, `X-LATITUDE`, `X-LONGITUDE`, `CHANNEL-ID`.
- **FR-009**: System MUST enforce idempotency using `X-EXTERNAL-ID` header.
- **FR-010**: System MUST return SNAP-compliant response codes as STRING (max 7 chars): `2002700` (create success), `2003100` (delete success), `2002500` (payment success).
- **FR-011**: System MUST enforce VA expiry — expired VAs (past `expiredDate`) cannot accept payments and return SNAP error response.
- **FR-012**: System MUST expose `DELETE /v1.0/transfer-va/delete-va` endpoint (SNAP service code 31) accepting `DeleteVARequest` body. Required: `partnerServiceId`, `customerNo`, `virtualAccountNo`. Optional: `trxId`, `additionalInfo`.
- **FR-013**: System MUST only allow deletion of pending (status "03") transactions; paid, expired, or already deleted transactions return appropriate SNAP error.
- **FR-014**: Create VA `virtualAccountTrxType` values per ASPI: C=Closed, O=Open, I=Partial, M=Minimum, L=Maximum, N=Open Minimum, X=Open Maximum.
- **FR-015**: Create VA response (`VAUpsertResponse`) MUST include: `partnerServiceId`, `customerNo`, `virtualAccountNo`, `virtualAccountName`, `virtualAccountEmail`, `virtualAccountPhone`, `trxId`, `totalAmount`, `billDetails`, `freeTexts`, `virtualAccountTrxType`, `feeAmount`, `expiredDate`, `lastUpdateDate`, `paymentDate`, `additionalInfo` (with `dbUrlProcess`).
- **FR-016**: Payment response (`PaymentResponse`) MUST include: `paymentFlagStatus`, `paymentFlagReason`, `partnerServiceId`, `customerNo`, `virtualAccountNo`, `trxId`, `paymentRequestId`, `paidAmount`, `paidBills`, `totalAmount`, `trxDateTime`, `referenceNo`, `journalNum`, `paymentType`, `flagAdvise`, `billDetails`, `freeTexts`, `additionalInfo`.

### Key Entities

- **VA Transaction**: Represents a Virtual Account payment request with SNAP-compliant fields.
- **VA Bill Detail**: Individual bill items (max 24) with fields: `billCode`, `billNo`, `billName`, `billShortName`, `billDescription` (MultiLangText), `billSubCompany`, `billAmount` (Amount), `billAmountLabel`, `billAmountValue`, `billReferenceNo`, `billerReferenceId`, `status`, `reason` (MultiLangText), `additionalInfo`.
- **Payment Notification**: Record of a payment received for a VA transaction.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Create VA requests are processed and responded to within 3 seconds on average.
- **SC-002**: Transaction list queries return results within 2 seconds for up to 10,000 records.
- **SC-003**: Payment webhook notifications are delivered to merchants within 5 seconds of receipt.
- **SC-004**: System handles 500 concurrent create VA requests without degradation.
- **SC-005**: 100% of SNAP API responses follow the standard response code format (e.g., `2002700` for create, `2003100` for delete).
- **SC-006**: Delete VA requests are processed within 1 second on average.

## Assumptions

- The existing VA transaction and bill detail tables will be used to store merchant-created transactions.
- Merchant authentication will use the existing SNAP B2B token system.
- Payment notifications will be received from banks via webhooks following SNAP VA Payment API format.
- Merchant notification webhook URL is provided per-transaction in the `notificationUrl` field of the create VA request payload.
- VA numbers follow the SNAP standard format: partnerServiceId (8 digits) + customerNo (up to 20 digits).
- Multi-bill transactions support up to 24 bill items per transaction per SNAP spec.
- VA transactions have an `expiredDate`; expired transactions cannot accept payments.
- The delete-va endpoint follows SNAP service code 31 with HTTP DELETE method.
- Response codes follow SNAP format as STRING: first 3 digits = HTTP status code, last 4 digits = service code (e.g., `2002700` = 200 + service 27).
- All schemas follow ASPI OpenAPI spec (`aspi-open-api.yaml`): `VAUpsertRequest`/`Response` for create, `PaymentRequest`/`Response` for payment, `DeleteVARequest`/`Response` for delete.
