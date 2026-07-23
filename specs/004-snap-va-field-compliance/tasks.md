---

description: "Task list for SNAP VA Field & Validation Compliance Fix"
---

# Tasks: SNAP VA Field & Validation Compliance Fix

**Input**: Design documents from `/specs/004-snap-va-field-compliance/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Included and REQUIRED — the project constitution (Principle III) mandates TDD for all bug fixes: existing tests currently assert the *wrong* (spec-violating) behavior and must be updated to assert correct behavior first, then fail against current code, before the production fix is made.

**Organization**: Tasks are grouped by user story (US1 = Inquiry field fix, US2 = Payment validation fix) to enable independent implementation and testing of each.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2)

## Path Conventions

Single Go backend project. All paths relative to repository root `/home/faris/code/manjo/payment-service-provider`.

---

## Phase 1: Setup

**Purpose**: No new project scaffolding is needed — this is a fix within the existing `internal/domain` / `internal/usecase` packages. Setup is limited to confirming the current baseline is green before changes begin.

- [X] T001 Run `go test ./internal/domain/... ./internal/usecase/... ./internal/infrastructure/... ./internal/adapter/...` and confirm the full existing suite passes before any change (baseline snapshot)

---

## Phase 2: Foundational

**Purpose**: No shared blocking infrastructure changes are required — both user stories touch independent fields on independent request structs/functions. This phase is a no-op; proceed directly to user stories.

**Checkpoint**: Foundation confirmed — US1 and US2 can proceed in any order, including in parallel.

---

## Phase 3: User Story 1 - Vendor sends spec-compliant Inquiry request (Priority: P1) 🎯 MVP

**Goal**: `VAInquiryRequest` correctly parses the spec's `txnDateInit` field and gains a mandatory, validated `amount` field.

**Independent Test**: Send an Inquiry payload with `txnDateInit` and `amount` per the ASPI spec; verify both are parsed/validated and a request missing `amount` is rejected with a mandatory-field error.

### Tests for User Story 1 ⚠️

> Write these tests FIRST; confirm they FAIL against current code before implementing.

- [X] T002 [P] [US1] Update/add struct tests in `internal/domain/va_test.go` asserting `VAInquiryRequest` unmarshals `txnDateInit` (not `trxDateInit`) into `TrxDateInit`, and unmarshals `amount` into a new `Amount` field
- [X] T003 [P] [US1] Add usecase test(s) in `internal/usecase/va_usecase_test.go` for `Inquiry`: (a) request with `txnDateInit` + `amount` succeeds and parses both values, (b) request missing `amount` is rejected with `"Invalid Mandatory Field [amount]"`

### Implementation for User Story 1

- [X] T004 [US1] In `internal/domain/va.go`, rename the `VAInquiryRequest.TrxDateInit` json tag from `trxDateInit` to `txnDateInit` (currently line 15)
- [X] T005 [US1] In `internal/domain/va.go`, add `Amount *Amount` field with `json:"amount"` to `VAInquiryRequest` (depends on T004 being in the same struct block)
- [X] T006 [US1] In `internal/usecase/va_usecase.go`, in the `Inquiry` function, add a mandatory-field check: return `domain.NewDomainError("4002402", "Invalid Mandatory Field [amount]", nil)` when `req.Amount == nil`
- [X] T007 [US1] Run `go test ./internal/domain/... ./internal/usecase/... -run 'Inquiry' -v` and confirm T002/T003 now pass

**Checkpoint**: Inquiry request is fully spec-compliant and independently testable/deployable.

---

## Phase 4: User Story 2 - Vendor sends spec-compliant Payment request (Priority: P2)

**Goal**: `VAPaymentRequest`/`Payment` usecase validation requires only `paymentRequestId` + `paidAmount` (per spec), no longer requires `transactionDate` (not in spec) or `totalAmount` (optional in spec).

**Independent Test**: Send a Payment payload with only `paymentRequestId` + `paidAmount`; verify it is accepted (not rejected for missing `transactionDate`/`totalAmount`), and that omitting `paymentRequestId` or `paidAmount` is still rejected.

### Tests for User Story 2 ⚠️

> Write these tests FIRST; confirm they FAIL against current code before implementing.

- [X] T008 [P] [US2] Update `TestVAUsecase_Payment_Success` in `internal/usecase/va_usecase_test.go` (~L125-138) to omit `TransactionDate` and `TotalAmount`, and assert success
- [X] T009 [P] [US2] Update `TestVAUsecase_Payment_NotifiesMerchant` in `internal/usecase/va_usecase_test.go` (~L164-178) to omit `TransactionDate`/`TotalAmount` as needed, asserting success
- [X] T010 [P] [US2] Update `TestVAUsecase_Payment_NoNotificationURL_SkipsCallback` in `internal/usecase/va_usecase_test.go` (~L205-219) to omit `TransactionDate`/`TotalAmount` as needed, asserting success
- [X] T011 [P] [US2] Update `TestVAUsecase_Payment_AmountMismatch` in `internal/usecase/va_usecase_test.go` (~L233-246) to keep `TotalAmount` present (since the mismatch check only applies when it's present) and confirm it still triggers correctly
- [X] T012 [P] [US2] Add new test cases in `internal/usecase/va_usecase_test.go`: (a) missing `PaymentRequestID` → rejected naming `paymentRequestId`, (b) missing `PaidAmount` → rejected naming `paidAmount`, (c) request with `PaidAmount` set and `TotalAmount == nil` → accepted, no mismatch check performed

### Implementation for User Story 2

- [X] T013 [US2] In `internal/usecase/va_usecase.go` `Payment` function (~L105), split the combined `PaidAmount == nil || TotalAmount == nil` check into a `PaidAmount == nil` mandatory check only
- [X] T014 [US2] In `internal/usecase/va_usecase.go` `Payment` function, add a `req.PaymentRequestID == ""` mandatory check returning `"Invalid Mandatory Field [paymentRequestId]"`
- [X] T015 [US2] In `internal/usecase/va_usecase.go` `Payment` function (~L109-111), remove the `req.TransactionDate == nil` mandatory check entirely
- [X] T016 [US2] In `internal/usecase/va_usecase.go` `Payment` function (~L128), guard the amount-mismatch check (`PaidAmount.Value != TotalAmount.Value`) behind `req.TotalAmount != nil`
- [X] T017 [US2] In `internal/usecase/va_usecase.go` `Payment` function (~L168), replace the `TransactionDate: *req.TransactionDate` dereference when building the persisted record with a derived value (`req.TrxDateTime` if present, else current server time) — do not reintroduce `TransactionDate` as a request-bound field
- [X] T018 [US2] Audit `internal/domain/va.go` for the `TransactionDate`/`TotalAmount` occurrences on `VAPaymentRequest` (~L57-59) and remove/adjust the `TransactionDate` field's role as a request input per [data-model.md](./data-model.md), keeping `TotalAmount` optional (`omitempty`)
- [X] T019 [US2] Update `TestVAUsecase_Status_Success` in `internal/usecase/va_usecase_test.go` (~L260-315) if it depends on the old `VAPaymentRequest` shape, so it still constructs valid fixtures under the corrected struct
- [X] T020 [US2] Run `go test ./internal/domain/... ./internal/usecase/... -run 'Payment|Status' -v` and confirm T008-T012 now pass

**Checkpoint**: Payment request validation is fully spec-compliant; both US1 and US2 are independently functional.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Confirm no regressions outside the two stories and that persistence/handler layers remain consistent.

- [X] T021 [P] Update `internal/adapter/delivery/http/handler/va_handler_test.go` (~L104-105) fixtures to match the corrected `VAInquiryRequest`/`VAPaymentRequest` shapes
- [X] T022 Audit `internal/infrastructure/database/va_repository.go` (L54, 81, 133, 161, 267, 273, 394) to confirm `VAInquiryRecord`/`VAPaymentRecord` persistence still receives a valid `TransactionDate`/`TotalAmount` value under the new optional/derived rules
- [X] T023 Run `go test ./... -cover` and confirm overall coverage remains ≥ 90% per constitution Principle XI, with zero regressions outside the touched files
- [X] T024 Run [quickstart.md](./quickstart.md) manual validation scenarios against a locally running instance (or `httptest`) to confirm end-to-end behavior matches [contracts/snap-va-inquiry-payment.md](./contracts/snap-va-inquiry-payment.md)

---

## Phase 6: Addendum — Additional ASPI Compliance Fixes (completed post-MVP)

**Purpose**: Fixes found via ad-hoc review against `aspi-open-api-va.yaml` and real end-to-end testing, beyond the original US1/US2 scope. See [spec.md Addendum](./spec.md#addendum-additional-aspi-compliance-fixes-post-mvp-same-feature-branch) for full rationale on each.

- [X] T025 [P] Add identity/amount echo fields to `VAPaymentStatus` (`internal/domain/va.go`) and populate them in both the success and idempotent-replay paths of `Payment()` (`internal/usecase/va_usecase.go`)
- [X] T026 [P] Make `MerchantCreateVARequest.VirtualAccountNo` mandatory and use the client-supplied value as-is in `CreateVA` (`internal/usecase/merchant_va_usecase.go`), instead of self-generating it
- [X] T027 [P] Drop `X-CLIENT-KEY` from `SNAPAuthMiddleware`'s default `RequiredHeaders` (`internal/adapter/delivery/http/middleware/snap_auth.go`, `internal/infrastructure/config/vendor_config.go`) and update `.env.vendor.channel.example`
- [X] T028 [P] Move `notificationUrl` off `MerchantCreateVARequest` as a top-level field; read/write it via `additionalInfo.dbUrlProcess` instead (`internal/usecase/merchant_va_usecase.go`)
- [X] T029 [US1] Stop `Inquiry()` from inserting a duplicate `VAInquiryRecord` when a merchant VA already exists for the `virtualAccountNo`; reflect the existing record's data instead (`internal/usecase/va_usecase.go`)
- [X] T030 Add `va_bill_details` missing columns (`bill_code`, `bill_name`, `bill_short_name`) via migration `db/migrations/000005_add_va_bill_details_missing_fields.up/down.sql`
- [X] T031 Implement `VARepository.SaveBillDetails` (delete+insert in one DB transaction) and call it from `CreateVA`; fix `SaveInquiry` to `RETURNING id` so the true persisted row id is available for linkage
- [X] T032 [US1] Wire `Inquiry()` to read back bill details via `GetVABillDetails` and include them in the response `VAAccountData.BillDetails`
- [X] T033 Fix `CreateVA`'s VA-reuse conflict check (`internal/usecase/merchant_va_usecase.go`): conflict only when the existing transaction is still pending (`status == "03"`), not when it's already terminal (paid/expired/deleted)
- [X] T034 Fix `GetVAByVirtualAccountNo` to `ORDER BY created_at DESC LIMIT 1` and scope `UpdateVAStatus` to `AND status = '03'`, since a `virtualAccountNo` can now have multiple historical rows (`internal/infrastructure/database/va_repository.go`)
- [X] T035 [P] Rework `scripts/vendor-inquiry-va.sh`, `scripts/vendor-payment-va.sh`, `scripts/merchant-create-va.sh` for spec-correct payloads/headers, `-f <env-file>` secret loading, `-b`/`-d` bill details, and stderr-only diagnostics
- [X] T036 Add `scripts/e2e-va-flow.sh`: token → create-va → inquiry → payment → merchant-callback verification (local throwaway HTTP listener) in one command

**Checkpoint**: All known deviations from `aspi-open-api-va.yaml` in the implemented endpoints (inquiry/payment/create-va) are resolved; only the unimplemented Service Code 28-35 endpoints remain out of scope.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — run first as a baseline check
- **Foundational (Phase 2)**: No-op for this feature
- **User Story 1 (Phase 3)** and **User Story 2 (Phase 4)**: Both depend only on Phase 1 baseline; they touch different struct fields and different validation branches in the same `Payment`/`Inquiry` functions of `va_usecase.go` — safe to implement in parallel by different people, but if one person is doing both, do them sequentially to avoid merge noise in the same file
- **Polish (Phase 5)**: Depends on both US1 and US2 being complete

### Within Each User Story

- Tests (T002-T003 / T008-T012) MUST be written and confirmed failing before implementation (T004-T007 / T013-T020)
- Struct field changes before usecase validation changes within a story

### Parallel Opportunities

- T002 and T003 (US1 tests) can run in parallel — different files/no shared state
- T008-T012 (US2 tests) can all be authored in parallel as they're independent test functions in the same file (coordinate to avoid edit conflicts, or assign to one author)
- US1 (Phase 3) and US2 (Phase 4) can be worked on in parallel by two people since they touch different fields, though both land in `va_usecase.go`/`va.go` — expect a straightforward merge

---

## Parallel Example: User Story 1

```bash
# Launch both test-authoring tasks together:
Task: "Update struct tests for txnDateInit/amount in internal/domain/va_test.go"
Task: "Add Inquiry usecase tests for amount mandatory-field in internal/usecase/va_usecase_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup baseline check
2. Complete Phase 3: User Story 1 (Inquiry field fix)
3. **STOP and VALIDATE**: Run T007, confirm Inquiry behavior is spec-compliant
4. Ship as MVP — resolves the highest-risk silent-failure bug first

### Incremental Delivery

1. Setup baseline → Phase 3 (US1) → validate → Phase 4 (US2) → validate → Phase 5 polish
2. Each story lands as its own commit/PR for independent review, per the user's stated priority ordering (#1/#2 before #3, though both are covered here since #2 and #3 are both part of this feature's two stories)

---

## Notes

- [P] tasks touch different files or independent test functions — no shared state conflicts
- No new entities, endpoints, or infrastructure — this is a surgical validation/field-mapping fix
- Per project convention, no legacy aliasing is introduced for the old field names or old mandatory-field rules (see spec.md Assumptions)
- Commit after each task or logical group (tests-red, then implementation-green, per task)
- Out of scope (still deferred, per spec.md Assumptions): Service Code 28-35 endpoints only — everything else originally deferred (response-echo fields, `virtualAccountNo` handling, header mismatch) was picked up and fixed in Phase 6 (see spec.md Addendum)
