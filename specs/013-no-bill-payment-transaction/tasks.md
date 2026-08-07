---

description: "Task list for feature 013-no-bill-payment-transaction"
---

# Tasks: No-Bill VA — Transaction Created at Payment, Not at Create-VA

**Input**: Design documents from `/specs/013-no-bill-payment-transaction/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Test tasks are INCLUDED and MANDATORY. Constitution Principle III makes TDD non-optional for this project, and Principle XI requires ≥90% coverage. Every implementation task below is preceded by its failing test.

**Organization**: Tasks are grouped by user story. Note the shipping constraint in [Deployment Constraint](#-deployment-constraint) below — it overrides the usual "US1 alone is the MVP" pattern.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US6)
- Exact file paths are included in every task

## Path Conventions

Go service, Clean Architecture per the constitution's Directory Layout Convention. Repository root is `/home/faris/code/manjo/payment-service-provider`. Tests live beside their subjects as `*_test.go` — there is no separate `tests/` tree.

---

## ⚠️ Deployment Constraint

**US1, US2, and US3 MUST ship together as one unit.** They are all P1 and this is not a coincidence.

Once US1 lands, a no-bill `/create-va` writes only a `va_accounts` row. Until US3 lands, `VAUsecase.Inquiry` still resolves via `GetVAByVirtualAccountNo`, finds nothing, and falls through to the ad-hoc branch at [va_usecase.go:128](../../internal/usecase/va_usecase.go#L128) — which inserts an inquiry row with `status = "00"`. The subsequent payment then hits the `merchantVA.Status != "03"` guard at [va_usecase.go:260](../../internal/usecase/va_usecase.go#L260) and is rejected with `4092500`.

**Deploying US1 alone would break no-bill payments entirely.** The MVP is US1+US2+US3, not US1. See [Implementation Strategy](#implementation-strategy).

---

## Phase 1: Setup

**Purpose**: Working branch and a green baseline to measure regressions against

- [X] T001 Create and switch to branch `013-no-bill-payment-transaction` from `main`
- [X] T002 Capture the green baseline: run `go test ./... > /tmp/baseline-tests.txt` and `go test -coverprofile=/tmp/baseline-cov.out ./... && go tool cover -func=/tmp/baseline-cov.out | tail -1`, recording the current coverage percentage for the Phase 9 comparison
- [X] T003 Bring the stack up and confirm migrations are current: `docker compose up -d && docker compose up -d migrate`, then verify `SELECT MAX(version) FROM schema_migrations;` returns `12`

**Checkpoint**: Baseline recorded, stack running at migration `000012`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Schema, domain types, and repository methods that every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete. Nothing here changes runtime behavior — it only adds capability.

### Migration

- [X] T004 Create `db/migrations/000014_create_va_accounts.up.sql` with the `va_accounts` table per [data-model.md](./data-model.md#new-entity-va_accounts): `id VARCHAR(36) PRIMARY KEY`, `partner_service_id VARCHAR(8) NOT NULL`, `customer_no VARCHAR(20) NOT NULL`, `virtual_account_no VARCHAR(28) NOT NULL`, `va_type VARCHAR(2) NOT NULL`, `customer_name VARCHAR(255) NOT NULL`, `customer_email VARCHAR(255)`, `customer_phone VARCHAR(30)`, `trx_id VARCHAR(64) NOT NULL`, `notification_url VARCHAR(512)`, `status VARCHAR(10) NOT NULL DEFAULT 'ACTIVE'`, `expired_date TIMESTAMPTZ`, `created_at`/`updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- [X] T005 Add constraints and indexes to `db/migrations/000014_create_va_accounts.up.sql`: `UNIQUE (virtual_account_no)`, `UNIQUE (partner_service_id, customer_no)`, `CHECK (status IN ('ACTIVE','INACTIVE','EXPIRED'))`, `INDEX (partner_service_id)`
- [X] T006 Add the backfill INSERT to `db/migrations/000014_create_va_accounts.up.sql` using `SELECT DISTINCT ON (virtual_account_no) ... FROM va_transactions WHERE va_type IN ('01','02','03','04','05','06') ORDER BY virtual_account_no, created_at DESC`, deriving `status = 'INACTIVE'` where the latest row's status is `'04'` else `'ACTIVE'`, with `ON CONFLICT DO NOTHING` for idempotent re-runs (FR-022, SC-006)
- [X] T007 Create `db/migrations/000014_create_va_accounts.down.sql` containing only `DROP TABLE IF EXISTS va_accounts;`
- [X] T008 Apply and verify: `docker compose up -d migrate`, then confirm the backfill count matches `SELECT COUNT(DISTINCT virtual_account_no) FROM va_transactions WHERE va_type IS NOT NULL;` (quickstart Scenario 10)

### Domain layer

- [X] T009 [P] Add `VAAccount` struct and `VAAccountStatus` constants (`VAAccountStatusActive`/`Inactive`/`Expired`) to `internal/domain/va.go`, fields per [data-model.md](./data-model.md#domain-types-internaldomainvago), no driver imports (Constitution I)
- [X] T010 [P] Add `VAAccountListFilter`, `VAAccountListItem`, and `VATransactionListItem` structs to `internal/domain/va.go`
- [X] T011 [P] Add sentinel errors `ErrVAAccountNotFound`, `ErrVAAccountInactive`, `ErrVAPaymentDuplicate` to `internal/domain/va.go` alongside the existing `ErrVACustomerNoAlreadyRegistered`
- [X] T012 Extend the `domain.VARepository` interface in `internal/domain/va.go` with `SaveVAAccount`, `GetVAAccount`, `GetVAAccountByPartnerAndCustomer`, `UpdateVAAccountStatus`, `ListVAAccounts`, `SaveNoBillPayment`, `ListVATransactions` — all taking `ctx context.Context` first (Constitution IV). Compilation now fails for every mock; that failure is the checkpoint.

### Repository layer

- [X] T013 [P] Write failing error-mapping tests in `internal/infrastructure/database/va_repository_test.go`: `pgx.ErrNoRows` → `ErrVAAccountNotFound`, PostgreSQL unique-violation (SQLSTATE `23505`) → `ErrVAPaymentDuplicate`. Follow the file's existing convention — live-DB behavior is proven by quickstart, not here.
- [X] T014 Implement `SaveVAAccount` in `internal/infrastructure/database/va_repository.go` as `INSERT ... ON CONFLICT (virtual_account_no) DO UPDATE SET customer_name, customer_email, customer_phone, trx_id, notification_url, expired_date, status='ACTIVE', updated_at=NOW() RETURNING id`, scanning `id` back for the same reason `SaveInquiry` does (the conflict path keeps the original id)
- [X] T015 [P] Implement `GetVAAccount` and `GetVAAccountByPartnerAndCustomer` in `internal/infrastructure/database/va_repository.go`, returning `ErrVAAccountNotFound` on `pgx.ErrNoRows` and the driver error verbatim otherwise — the distinction `isNotFound` at [va_usecase.go:22](../../internal/usecase/va_usecase.go#L22) depends on
- [X] T016 [P] Implement `UpdateVAAccountStatus` in `internal/infrastructure/database/va_repository.go` with `WHERE virtual_account_no = $1 AND status = 'ACTIVE'`, returning `ErrVAAccountNotFound` when zero rows are affected — this guard is what makes the expiry callback exactly-once (research.md R-005)
- [X] T017 [P] Implement `SaveNoBillPayment` in `internal/infrastructure/database/va_repository.go` as a plain `INSERT` into `va_transactions` (**not** an upsert), mapping a unique violation on `inquiry_request_id` to `ErrVAPaymentDuplicate` (research.md R-003)
- [X] T018 Repoint `RegisterStaticCustomerNo` in `internal/infrastructure/database/va_repository.go` from `va_transactions` to `va_accounts`, keeping the `withLock` wrapper and the `ErrVACustomerNoAlreadyRegistered` result unchanged
- [X] T019 Update `MockVARepository` in `internal/usecase/va_usecase_test.go` and `MockMerchantVARepository` in `internal/usecase/merchant_va_usecase_test.go` with the seven new methods so the packages compile again

**Checkpoint**: `go build ./...` and `go test ./...` both pass with zero behavior change. Foundation ready.

---

## Phase 3: User Story 1 - Register a No-Bill VA Once (Priority: P1) 🎯 MVP part 1 of 3

**Goal**: `/create-va` for VA types `01`/`04` persists only a VA registration and writes zero transaction rows.

**Independent Test**: Send one `/create-va` with `additionalInfo.vaType: "01"`, assert `2002700`, assert one `va_accounts` row, assert zero `va_transactions` rows for that VA number.

### Tests for User Story 1 ⚠️ Write first, confirm they FAIL

- [X] T020 [P] [US1] Failing test in `internal/usecase/merchant_va_usecase_test.go`: `/create-va` with `vaType: "01"` calls `SaveVAAccount` exactly once and **never** calls `SaveInquiry`. Reproduces Problem Statement defect 3 (US1 AS1, FR-001, SC-002).
- [X] T021 [P] [US1] Failing test in `internal/usecase/merchant_va_usecase_test.go`: `/create-va` with `vaType: "04"` and empty `customerNo` calls `NextCustomerNoSequence`, registers under the generated number, derives `virtualAccountNo`, and calls no `SaveInquiry` (US1 AS2, FR-003).
- [X] T022 [P] [US1] Failing test in `internal/usecase/merchant_va_usecase_test.go`: a repeat `/create-va` on an already-registered no-bill VA returns `2002700` with updated holder details and still no `SaveInquiry` — asserting it does **not** return `4092700`/`4092701` (US1 AS3, FR-005).
- [X] T023 [P] [US1] Failing test in `internal/usecase/merchant_va_usecase_test.go`: `/create-va` for a no-bill type carrying `totalAmount` returns `4002706` and writes nothing (US1 AS4, FR-006).

### Implementation for User Story 1

- [X] T024 [US1] In `MerchantVAUsecase.CreateVA` (`internal/usecase/merchant_va_usecase.go`), after `vaTypeRule` resolution and field validation, add the no-bill branch keyed on `vaTypeRule.Billing == domain.VATypeBillingNone` — **not** on `vaType == "01" || vaType == "04"`, so operator-added no-bill types inherit the flow with no deploy (Constitution II)
- [X] T025 [US1] In the no-bill branch, build a `domain.VAAccount` from the request (partner service ID, resolved customer number, resolved VA number, VA type, holder name/email/phone, `trxId`, `notificationURLFromAdditionalInfo`, `expiredDate`, `status = ACTIVE`) and persist via `SaveVAAccount`; skip `SaveInquiry`, skip `SaveBillDetails`, and skip the pending-transaction conflict check at [merchant_va_usecase.go:163](../../internal/usecase/merchant_va_usecase.go#L163)
- [X] T026 [US1] Bypass `RegisterStaticCustomerNo` for no-bill static (`01`) so a repeat call updates rather than conflicts (FR-005), while leaving the call intact for static bill types — the deliberate asymmetry from research.md R-002
- [X] T027 [US1] For bill-bearing managed types (`02`,`03`,`05`,`06`), call `SaveVAAccount` **in addition to** the existing `SaveInquiry`, leaving every existing guard and error code untouched (FR-021, spec A-002). Unmanaged/legacy requests write no registration.
- [X] T028 [US1] Map a `SaveVAAccount` failure to `5002700` Internal Server Error, matching the existing `SaveInquiry` failure mapping
- [X] T029 [US1] Add the structured log line `event=va_account_registered virtual_account_no=%s va_type=%s` in the no-bill branch, following the `event=... key=value` form used by `markExpiredAndNotify` (Constitution VIII)
- [X] T030 [US1] Run quickstart Scenarios 1 and 6 to confirm zero transactions on registration and distinct dynamic VA numbers

**Checkpoint**: US1 tests green. ⚠️ **Do not deploy here** — see [Deployment Constraint](#-deployment-constraint).

---

## Phase 4: User Story 2 - Customer Pays a No-Bill VA Repeatedly (Priority: P1) 🎯 MVP part 2 of 3

**Goal**: Each payment into a registered no-bill VA creates its own settled transaction; the second payment succeeds exactly like the first.

**Independent Test**: Seed a no-bill registration, send three payments with distinct `paymentRequestId`s, assert three independent settled transactions and three callbacks.

### Tests for User Story 2 ⚠️ Write first, confirm they FAIL

- [X] T031 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: two payments with distinct `paymentRequestId` into one registered no-bill VA both return `2002500` with `paymentFlagStatus: "00"`, producing two `SaveNoBillPayment` calls. Reproduces Problem Statement defects 1 and 2 (US2 AS1/AS2, FR-008, FR-010).
- [X] T032 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: the persisted record carries `InquiryRequestID == req.PaymentRequestID`, `Status == "00"`, and `TotalAmount == PaidAmount` (FR-009, research.md R-003).
- [X] T033 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: holder name, email, phone, notification URL, and `trxId` are inherited from the registration when the payment request omits them (US2 AS5, FR-013).
- [X] T034 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: a payment for a `virtualAccountNo` with no registration returns `4042519` and persists nothing (US2 AS3, FR-011).
- [X] T035 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: a repeated `paymentRequestId` returns the original response via the `GetPayment` short-circuit with no second persist (US2 AS4, FR-012).
- [X] T036 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: a `SaveNoBillPayment` returning `ErrVAPaymentDuplicate` (the race that slips past the short-circuit) causes a re-read and replay of the original response — no duplicate row, no duplicate callback.
- [X] T037 [P] [US2] Failing test in `internal/usecase/va_usecase_test.go`: exactly one `va.payment.received` callback per settled payment, and zero callbacks when the registration has an empty `notification_url` (US2 AS6, FR-014).

### Implementation for User Story 2

- [X] T038 [US2] In `VAUsecase.Payment` (`internal/usecase/va_usecase.go`), insert the no-bill branch **after** the `GetPayment` idempotency short-circuit at [va_usecase.go:183](../../internal/usecase/va_usecase.go#L183) and **before** the `GetVAByVirtualAccountNo` lookup at [va_usecase.go:236](../../internal/usecase/va_usecase.go#L236); resolve the registration via `GetVAAccount` and branch only when found, ACTIVE, and no-bill
- [X] T039 [US2] Build the `domain.VAPaymentRecord` in the branch with `InquiryRequestID = req.PaymentRequestID` unconditionally (ignoring the vendor's `inquiryRequestId`/`trxId` for this key), `Status = "00"`, `TotalAmount = req.PaidAmount.Value`, and holder/notification fields inherited from the registration; persist via `SaveNoBillPayment`
- [X] T040 [US2] Handle `ErrVAPaymentDuplicate` from `SaveNoBillPayment` by re-reading through `GetPayment` and replaying that response, so a concurrent duplicate never produces a second row or a second callback (Constitution X)
- [X] T041 [US2] Bypass the `req.TotalAmount != nil && PaidAmount != TotalAmount` mismatch check at [va_usecase.go:323](../../internal/usecase/va_usecase.go#L323) on this branch — a no-bill VA has no bill amount to match against
- [X] T042 [US2] Add a `paidAmount <= 0` guard returning `4002501` with nothing persisted (Edge Cases)
- [X] T043 [US2] Return the success response per [contracts/payment-no-bill.md](./contracts/payment-no-bill.md), echoing `totalAmount` equal to `paidAmount` and `paymentFlagStatus: "00"`
- [X] T044 [US2] Route the callback through the existing `notifyMerchantWithVA`, sourcing `notificationURL` and `trxID` from the registration, and record the `va_notification_deliveries` audit row as the existing paths do
- [X] T045 [US2] Add the structured log line `event=va_nobill_payment_recorded virtual_account_no=%s payment_request_id=%s` (Constitution VIII)
- [X] T046 [US2] Run quickstart Scenarios 3 and 4 to confirm three payments settle independently and the replay creates no fourth row

**Checkpoint**: US2 tests green. Payments work; inquiry still broken until US3.

---

## Phase 5: User Story 3 - Inquiry Against a Registered but Unpaid No-Bill VA (Priority: P1) 🎯 MVP part 3 of 3

**Goal**: Inquiry resolves a no-bill VA from its registration and creates nothing.

**Independent Test**: Seed a registration with no transactions, send an inquiry, assert `2002400` with the registered holder name and zero persists.

### Tests for User Story 3 ⚠️ Write first, confirm they FAIL

- [X] T047 [P] [US3] Failing test in `internal/usecase/va_usecase_test.go`: inquiry against a registered, never-paid no-bill VA returns `2002400` / `inquiryStatus: "00"` with the registered `customer_name` as `virtualAccountName`, and **never** calls `SaveInquiry` (US3 AS1, FR-015, FR-016, SC-002).
- [X] T048 [P] [US3] Failing test in `internal/usecase/va_usecase_test.go`: `totalAmount.value` is `"0.00"` regardless of the request's own `amount` (spec A-005).
- [X] T049 [P] [US3] Failing test in `internal/usecase/va_usecase_test.go`: inquiry succeeds unchanged when the VA already has settled payments — prior payments never block a new inquiry (US3 AS2).
- [X] T050 [P] [US3] Failing test in `internal/usecase/va_usecase_test.go`: a VA number with no registration still falls through to the existing legacy path, unchanged (US3 fall-through, FR-022).

### Implementation for User Story 3

- [X] T051 [US3] In `VAUsecase.Inquiry` (`internal/usecase/va_usecase.go`), insert the registry branch **after** the `GetInquiry` idempotency short-circuit at [va_usecase.go:60](../../internal/usecase/va_usecase.go#L60) and **before** the `GetVAByVirtualAccountNo` lookup at [va_usecase.go:86](../../internal/usecase/va_usecase.go#L86); branch only when `GetVAAccount` returns an ACTIVE no-bill registration
- [X] T052 [US3] Build the response from the registration per [contracts/inquiry-no-bill.md](./contracts/inquiry-no-bill.md) and return **without** calling `SaveInquiry`, reporting `totalAmount` as `0.00` and omitting `billDetails`
- [X] T053 [US3] Preserve the fall-through: any non-`ErrVAAccountNotFound` error from `GetVAAccount` returns `5002400`, while a genuine not-found continues to the existing path untouched (mirrors the `isNotFound` discipline at [va_usecase.go:22](../../internal/usecase/va_usecase.go#L22))
- [X] T054 [US3] Run quickstart Scenario 2 to confirm the holder name is returned and zero rows are written

**Checkpoint**: 🎯 **MVP COMPLETE.** US1+US2+US3 together are the first safely deployable unit. Run quickstart Scenarios 1–6 end to end before deploying.

---

## Phase 6: User Story 4 - Query the Status of One No-Bill Payment (Priority: P2)

**Goal**: A status query resolves to one payment and returns that payment's own amount and timestamp.

**Independent Test**: Make two payments into one no-bill VA, query each by its own identifier, assert each returns its own values.

### Tests for User Story 4 ⚠️ Write first, confirm they FAIL

- [X] T055 [P] [US4] Failing test in `internal/usecase/va_usecase_test.go`: with two settled no-bill payments, `Status` queried with the first payment's identifier returns that payment's own `paidAmount`, `referenceNo`, and `transactionDate` — not the second's (US4 AS1, FR-018).
- [X] T056 [P] [US4] Failing test in `internal/usecase/va_usecase_test.go`: a status query for an identifier with no matching payment returns `4042619` (US4 AS2).

### Implementation for User Story 4

- [X] T057 [US4] Verify `VAUsecase.Status` (`internal/usecase/va_usecase.go`) resolves correctly given `inquiry_request_id == payment_request_id` for no-bill rows; adjust only if the tests above expose a gap — no change is expected, since `GetPayment` already keys on `payment_request_id`
- [X] T058 [US4] Confirm `totalAmount` falls back to `paidAmount` for no-bill rows via the existing logic at [va_usecase.go:641](../../internal/usecase/va_usecase.go#L641), and that `billDetails` comes back empty rather than nil-panicking

**Checkpoint**: Per-payment reconciliation works.

---

## Phase 7: User Story 5 - Bill-Bearing VA Types Keep Their Current Behavior (Priority: P2)

**Goal**: Regression guard. `02`, `03`, `05`, `06` behave exactly as before.

**Independent Test**: Re-run every existing acceptance scenario for the four bill-bearing types and diff against the Phase 1 baseline.

### Tests for User Story 5 ⚠️ These must pass unmodified

- [X] T059 [P] [US5] Run the full existing suite for bill-bearing types (`go test ./internal/usecase/... ./internal/infrastructure/... ./internal/adapter/...`) and diff against `/tmp/baseline-tests.txt` from T002 — any newly failing test is a regression, not an expectation to update (SC-005)
- [X] T060 [P] [US5] Add a test in `internal/usecase/merchant_va_usecase_test.go` asserting fixed-bill (`03`, `06`) still creates a pending `03` transaction at create-VA time bound to `totalAmount` (US5 AS1, FR-021)
- [X] T061 [P] [US5] Add a test in `internal/usecase/va_usecase_test.go` asserting variable-bill (`02`, `05`) still routes through `SaveVAPayment` with cumulative tracking and the `00`/`03` paymentFlagStatus logic intact (US5 AS2)
- [X] T062 [P] [US5] Add a test in `internal/usecase/merchant_va_usecase_test.go` asserting a repeat `/create-va` on a bill-bearing VA with a pending transaction still returns the conflict, and that static bill types still return `4092701` on a duplicate `customerNo` (US5 AS3, research.md R-002)

### Implementation for User Story 5

- [X] T063 [US5] Fix any regression the tests above surface, preserving existing response codes exactly — no bill-bearing behavior may change in this feature
- [X] T064 [US5] Run quickstart Scenario 9 plus `./scripts/e2e-va-flow.sh`, `./scripts/e2e-dynamic-va-flow.sh`, `./scripts/e2e-expired-callback-flow.sh`, `./scripts/e2e-va-cancel-flow.sh`

**Checkpoint**: Zero regressions on bill-bearing flows.

---

## Phase 8: User Story 6 - Deactivate a Registered No-Bill VA (Priority: P3)

**Goal**: Delete-VA deactivates the registration; expiry is detected on the registration with exactly-once callback.

**Independent Test**: Register a no-bill VA, deactivate it, assert inquiry and payment are both rejected while historical payments stay readable.

### Tests for User Story 6 ⚠️ Write first, confirm they FAIL

- [X] T065 [P] [US6] Failing test in `internal/usecase/merchant_va_usecase_test.go`: delete-VA on an ACTIVE no-bill registration calls `UpdateVAAccountStatus(..., 'INACTIVE')` and returns `2003100` (US6 AS1, FR-019).
- [X] T066 [P] [US6] Failing test in `internal/usecase/merchant_va_usecase_test.go`: a repeat delete-VA returns `2003100` without a second state change (US6 AS4).
- [X] T067 [P] [US6] Failing test in `internal/usecase/va_usecase_test.go`: payment against an INACTIVE registration returns `4042519`, and inquiry returns `4042419` (US6 AS2, US3 AS3).
- [X] T068 [P] [US6] Failing test in `internal/usecase/va_usecase_test.go`: an expired registration returns `4042419`/`4042519` with the expiry reason, transitions the registration to `EXPIRED`, and enqueues exactly one `va.expired` callback across repeated calls (US3 AS4, FR-017).
- [X] T069 [P] [US6] Failing test in `internal/usecase/merchant_va_usecase_test.go`: deactivation performs zero writes against historical `va_transactions` rows (US6 AS3, FR-020).

### Implementation for User Story 6

- [X] T070 [US6] Add `markRegistrationExpiredAndNotify` to `internal/usecase/va_usecase.go`, mirroring `markExpiredAndNotify` at [va_usecase.go:490](../../internal/usecase/va_usecase.go#L490) but calling `UpdateVAAccountStatus(..., 'EXPIRED')` — relying on its `WHERE status='ACTIVE'` guard for exactly-once semantics, and reusing `deliveryRepo.ExistsByVirtualAccountNoAndEventType` for the belt-and-suspenders dedupe
- [X] T071 [US6] Wire the expired/inactive guards into the US3 inquiry branch: INACTIVE → `4042419`; past `expired_date` → `markRegistrationExpiredAndNotify` then `4042419` with the expired reason
- [X] T072 [US6] Wire the same guards into the US2 payment branch: INACTIVE → `4042519`; past `expired_date` → `markRegistrationExpiredAndNotify` then `4042519` with `paymentFlagStatus: "01"`
- [X] T073 [US6] In `MerchantVAUsecase.DeleteVA` (`internal/usecase/merchant_va_usecase.go`), branch to `UpdateVAAccountStatus(..., 'INACTIVE')` when the VA number resolves to a no-bill registration, returning `2003100` idempotently when already INACTIVE or EXPIRED; keep the existing transaction path for every other case
- [X] T074 [US6] Confirm the FR-005 registration upsert reactivates an INACTIVE registration (`status='ACTIVE'` in the `DO UPDATE` from T014) and add a test for it in `internal/usecase/merchant_va_usecase_test.go`
- [X] T075 [US6] Run quickstart Scenario 7

**Checkpoint**: Full lifecycle works. All six user stories complete.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Reporting correctness (FR-023), wiring, docs, and the quality gates

### Reporting split (FR-023, SC-007) — cross-cutting, owned by no single story

- [X] T076 [P] Write failing tests in `internal/infrastructure/database/va_repository_test.go` for `ListVAAccounts` and `ListVATransactions` filter/SQL construction, mirroring the existing `ListVA` test approach
- [X] T077 Implement `ListVAAccounts` in `internal/infrastructure/database/va_repository.go` reading from `va_accounts`, with `transactionCount` and `totalPaid` aggregates over that VA number's settled `va_transactions` rows, and `status` filtering on `ACTIVE`/`INACTIVE`/`EXPIRED`
- [X] T078 Implement `ListVATransactions` in `internal/infrastructure/database/va_repository.go` by moving the existing `ListVA` query body across unchanged, keeping its `00`/`02`/`03`/`04` status semantics
- [X] T079 Repoint `MerchantVAHandler.ListVA` in `internal/adapter/delivery/http/handler/merchant_va_handler.go` at `ListVAAccounts` and add a `ListTransactions` handler, per [contracts/merchant-list.md](./contracts/merchant-list.md)
- [X] T080 [P] Update `MockMerchantVAUsecase` and add handler tests in `internal/adapter/delivery/http/handler/merchant_va_handler_test.go` for both listings
- [X] T081 Register `POST /list-transactions` on `merchantGroup` in `cmd/api/main.go` alongside the existing `/list` at [main.go:431](../../cmd/api/main.go#L431)
- [X] T082 Run quickstart Scenario 8 to confirm 1 VA / 3 transactions

### Documentation

- [X] T083 [P] Update Swagger annotations for the listing change and regenerate `docs/`
- [X] T084 [P] Update `README.md` and the no-bill sections of `docs/guides/` to describe register-once / pay-many, replacing any "call create-va before each payment" guidance
- [X] T085 [P] Write a release note covering the `POST /list` response-shape change and pointing existing clients at `POST /list-transactions` (plan.md Risks)

### Quality gates (Constitution III, XI, and the Development Workflow section)

- [X] T086 Run `go test -race ./...` — must pass with zero errors
- [X] T087 Run `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` — must report ≥90% and must not regress against the T002 baseline
- [X] T088 Run `golangci-lint run` — must produce zero warnings
- [X] T089 Verify the multi-stage non-root Docker build still succeeds
- [X] T090 Run the complete [quickstart.md](./quickstart.md) — all ten scenarios, including Scenario 10's before/after stranded-VA check (SC-006)
- [X] T091 Verify migration rollback: `migrate down 1` drops `va_accounts` cleanly and the prior schema is restored exactly, then re-apply

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies
- **Phase 2 (Foundational)**: Depends on Phase 1 — **BLOCKS all user stories**
- **Phase 3–5 (US1, US2, US3)**: All depend on Phase 2. Mutually independent in code (three different branches in two different files) but **must ship together** — see [Deployment Constraint](#-deployment-constraint)
- **Phase 6 (US4)**: Depends on US2 (needs no-bill payment rows to query)
- **Phase 7 (US5)**: Depends on Phase 2; most meaningful once US1–US3 are in, since that is when regression risk peaks
- **Phase 8 (US6)**: Depends on US2 and US3 — the guards it adds live inside their branches
- **Phase 9 (Polish)**: Depends on all desired stories

### Within Each User Story

- Tests MUST be written and MUST FAIL before implementation (Constitution III)
- Domain types → repository → usecase → handler → routes
- Story complete and checkpoint validated before moving to the next

### Parallel Opportunities

- **Phase 2**: T009, T010, T011 are three independent additions to `internal/domain/va.go` — parallel in planning, but serialize the edits since they touch one file. T013 and T015–T017 are genuinely parallel across separate concerns.
- **Phase 3–5**: All test tasks within a story are `[P]` (independent test functions). Implementation tasks within a story are sequential — T024–T029 all edit `merchant_va_usecase.go`, T038–T045 all edit `va_usecase.go`.
- **Across stories**: US1 (`merchant_va_usecase.go`) and US2+US3 (`va_usecase.go`) touch different files, so two developers can run Phase 3 and Phases 4–5 concurrently after Phase 2.
- **Phase 9**: T083, T084, T085 are fully parallel. T076–T082 are sequential (shared files).

---

## Parallel Example: User Story 2

```bash
# Write all US2 tests together — independent test functions in one file:
Task: "T031 two distinct payments both settle in internal/usecase/va_usecase_test.go"
Task: "T032 persisted record field assertions in internal/usecase/va_usecase_test.go"
Task: "T033 holder inheritance from registration in internal/usecase/va_usecase_test.go"
Task: "T034 unregistered VA returns 4042519 in internal/usecase/va_usecase_test.go"
Task: "T035 repeated paymentRequestId replays in internal/usecase/va_usecase_test.go"
Task: "T036 ErrVAPaymentDuplicate race replays in internal/usecase/va_usecase_test.go"
Task: "T037 exactly one callback per payment in internal/usecase/va_usecase_test.go"

# Confirm ALL fail, then implement T038-T045 sequentially (single file).
```

## Parallel Example: Two developers after Phase 2

```bash
# Developer A — merchant_va_usecase.go
Phase 3 (US1): T020-T030

# Developer B — va_usecase.go
Phase 4 (US2): T031-T046
Phase 5 (US3): T047-T054

# Both converge before deploying; neither ships alone.
```

---

## Implementation Strategy

### MVP = US1 + US2 + US3 (not US1 alone)

1. Phase 1: Setup — baseline captured
2. Phase 2: Foundational — schema and repository ready, zero behavior change
3. Phases 3, 4, 5: US1, US2, US3
4. **STOP and VALIDATE**: quickstart Scenarios 1–6, plus Phase 7's regression suite
5. Deploy the three together

The usual "ship P1 story 1 as the MVP" shortcut does not apply here. US1 removes the transaction that US2 and US3 replace; shipping it alone leaves no-bill VAs unpayable.

### Incremental Delivery After MVP

1. MVP (US1+US2+US3) → deploy
2. US4 (per-payment status) → deploy
3. US6 (deactivation + registration expiry) → deploy
4. Phase 9 reporting split → deploy with the release note, since `POST /list` changes shape

US5 is not a deliverable — it is the regression gate run before each of the above.

### Rollback

The migration only creates a table; it alters no existing column or row. Rolling back means reverting the application code and running `migrate down 1`. Because no-bill inquiry and payment both fall through to the legacy path when no registration is found, a code-only rollback with `va_accounts` left in place is also safe.

---

## Notes

- `[P]` = different files or independent test functions, no dependencies
- `[Story]` maps each task to a user story for traceability
- Verify every test fails before implementing it (Constitution III)
- Commit after each task or logical group
- The two open decisions in [plan.md](./plan.md#open-decisions-for-confirmation) (A-002 registry for all managed types, A-003 repeat create-VA is an update) are baked into Phase 3. Changing either after T027 is expensive — confirm before starting Phase 3.
