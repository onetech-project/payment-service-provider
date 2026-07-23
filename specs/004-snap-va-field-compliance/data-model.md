# Phase 1 Data Model: SNAP VA Field & Validation Compliance Fix

This feature modifies two existing request entities; no new entities or storage schema are introduced. Persistence-side records are affected only insofar as they must stay consistent with the corrected request shape.

## VAInquiryRequest (`internal/domain/va.go`)

Represents an incoming SNAP Inquiry request (Service Code 24).

| Field | Type | JSON Tag (current → corrected) | Required? | Notes |
|---|---|---|---|---|
| PartnerServiceID | string | `partnerServiceId` | yes (unchanged) | |
| CustomerNo | string | `customerNo` | yes (unchanged) | |
| VirtualAccountNo | string | `virtualAccountNo` | yes (unchanged) | |
| TrxDateInit | *time.Time | `trxDateInit` → **`txnDateInit`** | optional (per spec, not in `required` list) | Rename only; parses silently to nil today under wrong tag |
| Amount | *Amount | *(new field)* → **`amount`** | **yes (new mandatory)** | Missing entirely today; must be added, bound, and validated |
| ChannelCode | int | `channelCode` | optional (unchanged) | |
| InquiryRequestID | string | `inquiryRequestId` | yes (unchanged) | |
| AdditionalInfo | map[string]interface{} | `additionalInfo` | optional (unchanged) | |

**Validation rules**:
- `InquiryRequestID` and `Amount` are mandatory; missing either MUST return a domain error in the existing `"Invalid Mandatory Field [<field>]"` style, using `amount` as the field name for the new check.
- `TrxDateInit`/`Amount` type shapes stay the same (`*time.Time`, `*Amount`); only the wire field name and the presence of `Amount` change.

## VAPaymentRequest (`internal/domain/va.go`)

Represents an incoming SNAP Payment request (Service Code 25).

| Field | Type | Required? (current code vs. spec) | Change |
|---|---|---|---|
| PaymentRequestID | string | not currently validated as mandatory / **mandatory per spec** | Add mandatory check |
| PaidAmount | *Amount | mandatory (correct) | No change |
| TotalAmount | *Amount | currently mandatory / **optional per spec** | Remove from mandatory check; keep field for optional mismatch validation when present |
| TrxDateTime | *time.Time | optional (correct) | No change |
| TransactionDate | *time.Time | currently mandatory / **does not exist in spec** | Remove mandatory check and remove field's role as request input |

**Validation rules**:
- Mandatory: `PaymentRequestID` (non-empty), `PaidAmount` (non-nil).
- Optional: `TotalAmount` — when present, the existing amount-mismatch check (`PaidAmount.Value != TotalAmount.Value`) still applies; when absent, the mismatch check is skipped.
- `TransactionDate` is no longer read from the incoming request as a validation input. Any internal record still needing a transaction-date value derives it from `TrxDateTime` (if present) or the server's processing time — decided at implementation time against `internal/infrastructure/database/va_repository.go`'s existing `VAPaymentRecord` shape, without adding it back as a *request* field.

## Downstream/Persistence Entities (consistency only, not redesigned)

- **VAInquiryRecord** / **VAPaymentRecord** (`internal/infrastructure/database/va_repository.go`): retain their existing `TotalAmount`/`TransactionDate` storage columns; only the *source* of the value (mandatory request field vs. derived/optional) changes.
- **VAAccountData / VAStatusData** (response-facing DTOs in `internal/domain/va.go`): unaffected by this initial phase.

## Addendum: entities changed in Phase 6 (see spec.md Addendum)

The items below were originally marked out of scope in this document but were addressed later under the same branch:

- **VAPaymentStatus** (`internal/domain/va.go`): gained `PartnerServiceID`, `CustomerNo`, `VirtualAccountNo`, `TrxID`, `PaymentRequestID`, `PaidAmount`, `TotalAmount`, `TrxDateTime`, `ReferenceNo` — previously only `PaymentFlagStatus`/`PaymentFlagReason`, which didn't echo `PaymentResponse.virtualAccountData` per spec (l.228-254).
- **MerchantCreateVARequest** (`internal/domain/va.go`): `VirtualAccountNo` is now mandatory (was `omitempty` and ignored server-side); `NotificationURL` top-level field was removed — that data now travels via `AdditionalInfo["dbUrlProcess"]`, the spec's own extension slot for `VAUpsertRequest` (l.317-320).
- **VAInquiryRecord** (`internal/domain/va.go` / `va_transactions` table): a `virtualAccountNo` can now have **multiple rows** across its lifetime (one per transaction cycle) instead of exactly one — `GetVAByVirtualAccountNo` returns the most recent (`ORDER BY created_at DESC LIMIT 1`), and `UpdateVAStatus`/the create-va conflict check are scoped to the currently-pending (`status = '03'`) row only.
- **BillDetail persistence** (`va_bill_details` table): now actually written by `VARepository.SaveBillDetails` (delete+insert per transaction id) and read back by both `GetVABillDetails` (merchant dashboard) and `Inquiry()` (vendor-facing response) — previously dead code, and the table was missing `bill_code`/`bill_name`/`bill_short_name` columns that `GetVABillDetails`'s SELECT already referenced.
