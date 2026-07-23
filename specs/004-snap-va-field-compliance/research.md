# Phase 0 Research: SNAP VA Field & Validation Compliance Fix

No `NEEDS CLARIFICATION` markers remain in the Technical Context — this is a bug fix against an existing, already-integrated feature, so the unknowns below are resolved by reading the current code and spec directly rather than external research.

## Decision: Field rename source

- **Decision**: Rename `VAInquiryRequest.TrxDateInit` json tag from `trxDateInit` to `txnDateInit` (`internal/domain/va.go:15`).
- **Rationale**: `aspi-open-api-va.yaml:152` defines the field as `txnDateInit` under `InquiryRequest`. A vendor sending a spec-compliant payload currently has this field silently dropped (parses to `nil`) since Go's `encoding/json` leaves unmatched fields as zero-value with no error.
- **Alternatives considered**: Accepting both tag names via a custom `UnmarshalJSON` (rejected — adds legacy-alias complexity the project convention explicitly avoids for a straight bug fix; no external caller is known to depend on the wrong name since it was never spec-compliant).

## Decision: Add mandatory `amount` to Inquiry

- **Decision**: Add an `Amount *domain.Amount` field (`json:"amount"`) to `VAInquiryRequest`, and add a nil-check in the `Inquiry` usecase function (`internal/usecase/va_usecase.go`, function starting line 24) returning the existing `"Invalid Mandatory Field [amount]"` domain error style.
- **Rationale**: `aspi-open-api-va.yaml:150` lists `amount` in `InquiryRequest.required`. Today the field doesn't exist on the struct at all, so it can never be bound, validated, or used.
- **Alternatives considered**: Making it optional with a default (rejected — contradicts the spec's `required` list and Success Criteria SC-003).

## Decision: Correct Payment mandatory-field validation

- **Decision**: In `internal/usecase/va_usecase.go` `Payment` function:
  - Remove the `req.TransactionDate == nil` check (lines 109-111) and the `TransactionDate` field usage at line 168.
  - Split the combined `PaidAmount == nil || TotalAmount == nil` check (line 105) into a `PaidAmount == nil` mandatory check only; `TotalAmount` becomes optional.
  - Preserve the existing amount-mismatch check (line 128) but guard it behind `req.TotalAmount != nil` since it's now optional.
  - Add a `PaymentRequestID == ""` mandatory check (spec requires it; not currently validated).
- **Rationale**: `aspi-open-api-va.yaml:195` sets `required: [paymentRequestId, paidAmount]` for `PaymentRequest`; `transactionDate` does not exist in the spec's `PaymentRequest` schema at all (only `trxDateTime`, optional, line 209); `totalAmount` is present but optional (line 208).
- **Alternatives considered**: Keeping `TransactionDate` as an optional field for internal bookkeeping (rejected for the *request* struct — the spec has no such field, so keeping it invites vendors to keep omitting a field the code silently expects; downstream persistence can still derive an internal timestamp from `TrxDateTime` or `time.Now()` if a record needs one — that mapping decision is confirmed against `va_repository.go` during implementation, not spec-affecting).

## Downstream touch points requiring lockstep update (from code inspection)

- `internal/usecase/va_usecase_test.go`: `TestVAUsecase_Payment_Success` (L125), `_NotifiesMerchant` (L164), `_NoNotificationURL_SkipsCallback` (L205), `_AmountMismatch` (L233), `TestVAUsecase_Status_Success` (L260) — all set `TotalAmount`/`TransactionDate` and must be updated to match corrected mandatory-field rules.
- `internal/infrastructure/database/va_repository.go` (L54, 81, 133, 161, 267, 273, 394) — `VAInquiryRecord`/`VAPaymentRecord` persistence fields; confirm whether `TransactionDate` column still needs a value derived from `TrxDateTime`/server time now that it's not a request input.
- `internal/domain/va.go` (L209, 230, 276, 302, 362, 367, 404) — other DTOs referencing `TotalAmount`/`TransactionDate`; audit each for whether it's request-facing (must match spec) vs. response/record-facing (may retain internal field for persistence/echo purposes).
- `internal/domain/va_test.go` (L89, 93, 145) and handler tests (`va_handler_test.go` L104-105, `merchant_va_handler_test.go` L57) — update fixtures to match corrected struct shape.
- `internal/usecase/merchant_va_usecase.go` / `merchant_va_usecase_test.go` — out of scope for this feature (Merchant VA dashboard create/upsert flow, not Inquiry/Payment); verified during implementation that no shared struct is affected, otherwise flag as a cross-feature dependency.

**Output**: All unknowns resolved; no `NEEDS CLARIFICATION` remain.
