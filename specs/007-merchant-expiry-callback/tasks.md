---

description: "Task list for feature implementation"
---

# Tasks: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Input**: Design documents from `/specs/007-merchant-expiry-callback/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (all present)

**Tests**: Included and REQUIRED — constitution Principle III mandates TDD (failing test → minimal implementation → refactor) for all feature work in this repo.

**Organization**: Tasks are grouped by user story (US1 = expiry detection & callback, P1; US2 = resend endpoint, P2) to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 or US2
- File paths are exact and relative to repo root

---

## Phase 1: Setup

**Purpose**: Confirm the working environment before touching code — no new dependencies are required (per plan.md, all libraries already in `go.mod`).

- [ ] T001 Verify local dev stack (PostgreSQL, Redis, Asynq worker, API server) runs per existing repo scripts, and confirm `ADMIN_API_KEY` and a test merchant `notification_url` receiver are configured, per specs/007-merchant-expiry-callback/quickstart.md Prerequisites

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared data model and domain contracts required by BOTH user stories (expiry dedupe check in US1 and delivery-history lookup in US2 both read/write the same audit table).

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 Create migration `db/migrations/000009_create_va_notification_deliveries.up.sql` and `.down.sql` for new table `va_notification_deliveries` (columns: `id`, `virtual_account_no`, `event_type`, `trigger`, `status`, `attempted_at`, `error_detail`) per specs/007-merchant-expiry-callback/data-model.md
- [X] T003 [P] Write failing repository test for `VANotificationDeliveryRepository` (Create + GetLatestByVirtualAccountNo + ExistsByVirtualAccountNoAndEventType) in internal/infrastructure/database/va_repository_test.go
- [X] T004 [P] Add `NotificationDelivery` domain entity and `VANotificationDeliveryRepository` interface (`Create`, `GetLatestByVirtualAccountNo`, `ExistsByVirtualAccountNoAndEventType`) to internal/domain/va.go
- [X] T005 [P] Extend `PaymentNotificationPayload` in internal/domain/va.go with `EventType string` and `ExpiredAt string` fields (`omitempty` where applicable) per specs/007-merchant-expiry-callback/data-model.md
- [X] T006 [P] Add SNAP error constants for expired inquiry/notification (`ErrVAExpiredInquiry` snapCode `4042419`, `ErrVAExpiredPayment` snapCode `4042519`) alongside existing `ErrMerchantVAExpired` in internal/domain/errors.go
- [X] T007 Implement `VANotificationDeliveryRepository` in internal/infrastructure/database/va_repository.go (depends on T002, T003, T004) — makes T003 pass

**Checkpoint**: Foundation ready — `va_notification_deliveries` table and domain contracts exist; US1 and US2 implementation can now proceed (US2 depends on US1's dedupe/enqueue plumbing existing, but both can start once this phase is done since they touch different files).

---

## Phase 3: User Story 1 - Merchant is notified when a Virtual Account transaction expires (Priority: P1) 🎯 MVP

**Goal**: Detect expiry inline on bill-inquiry and payment-notification calls, transition the VA to `"02"` (expired), return the SNAP-specified expired responses, and deliver exactly one `va.expired` merchant callback.

**Independent Test**: Create a VA with a short expiry window, let it pass without payment, call the SNAP inquiry endpoint, and verify the `4042419` response, the DB status transition to `"02"`, and a single signed `va.expired` callback delivered to the merchant's registered URL (per specs/007-merchant-expiry-callback/quickstart.md Scenarios 1–4).

### Tests for User Story 1 ⚠️ (write first, confirm they FAIL before implementation)

- [X] T008 [P] [US1] Failing test: `VAUsecase.Inquiry` returns `4042419`/`inquiryStatus: "01"`/`inquiryReason` per contracts/inquiry-expired.md, transitions status to `"02"`, and enqueues one `va.expired` notification, in internal/usecase/va_usecase_test.go
- [X] T009 [P] [US1] Failing test: `VAUsecase.Payment` returns `4042519`/`paymentFlagStatus: "01"`/`paymentFlagReason` per contracts/notify-expired.md, transitions status to `"02"`, and enqueues one `va.expired` notification, in internal/usecase/va_usecase_test.go
- [X] T010 [P] [US1] Failing test: repeated inquiry/notify calls on an already-expired VA do NOT enqueue a second `va.expired` notification (dedupe via `ExistsByVirtualAccountNoAndEventType`), in internal/usecase/va_usecase_test.go
- [X] T011 [P] [US1] Failing test: a VA with no `notification_url` still transitions to `"02"` but no notification is enqueued, in internal/usecase/va_usecase_test.go
- [X] T012 [P] [US1] Failing test: a VA paid concurrently before expiry detection is NOT transitioned to expired and receives no `va.expired` callback (race precedence per FR-010), in internal/usecase/va_usecase_test.go
- [X] T013 [P] [US1] Failing test: `payment_notification_worker` correctly signs and delivers a `va.expired`-typed payload (HMAC-SHA512, `X-Timestamp`/`X-Signature`) with no paid-amount fields present, in internal/adapter/delivery/worker/payment_notification_worker_test.go

### Implementation for User Story 1

- [X] T014 [US1] Add expiry-check branch in `VAUsecase.Inquiry` (internal/usecase/va_usecase.go): when `merchantVA.Status == "03"` and `time.Now().After(*merchantVA.ExpiredDate)`, return `domain.NewDomainError("4042419", ...)` per contracts/inquiry-expired.md — depends on T008
- [X] T015 [US1] Add expiry-check branch in `VAUsecase.Payment` (internal/usecase/va_usecase.go), replacing/extending the existing `status != "03"` conflict guard to special-case expired VAs with `4042519` per contracts/notify-expired.md — depends on T009
- [X] T016 [US1] Implement shared `markExpiredAndNotify(ctx, merchantVA)` helper in internal/usecase/va_usecase.go: calls `UpdateVAStatus(ctx, virtualAccountNo, "02")`, checks `ExistsByVirtualAccountNoAndEventType` for dedupe, and — if VA has a `notification_url` — calls `notifier.EnqueuePaymentNotification` with `EventType: "va.expired"` and inserts a `va_notification_deliveries` row via `Create` (`trigger: "auto"`); called from both T014 and T015 — depends on T010, T011, T012
- [X] T017 [US1] Extend `payment_notification_worker.HandlePaymentNotification` in internal/adapter/delivery/worker/payment_notification_worker.go to branch on `payload.EventType` (default `"payment.received"` for backward compatibility) and build the outbound JSON body accordingly for `"va.expired"` — depends on T013
- [X] T018 [US1] Add OTel span + structured log fields (`virtual_account_no`, `event_type`) around the new expiry-detection and notification-enqueue code paths in internal/usecase/va_usecase.go, consistent with existing `Inquiry`/`Payment` instrumentation (constitution Principle VIII)

**Checkpoint**: User Story 1 is fully functional and independently testable — expired VAs are correctly rejected on inquiry/notify, transitioned in the DB, and merchants receive exactly one signed callback.

---

## Phase 4: User Story 2 - Operator manually resends a failed or missed merchant callback (Priority: P2)

**Goal**: Provide an admin-only endpoint to redeliver the most recent callback event for a transaction on demand, with a full audit trail.

**Independent Test**: Take a VA that already has a delivery record from US1 (or an existing `payment.received` delivery), call `POST /admin/transactions/:virtualAccountNo/resend-callback` with a valid admin key, and verify a second signed callback is delivered and a new `trigger: "manual"` row is recorded (per specs/007-merchant-expiry-callback/quickstart.md Scenario 5).

### Tests for User Story 2 ⚠️ (write first, confirm they FAIL before implementation)

- [X] T019 [P] [US2] Failing test: resend usecase succeeds for a VA with a prior delivery record, redelivers the current event via `NotificationEnqueuer`, and records a new `trigger: "manual"` row, in internal/usecase/resend_callback_usecase_test.go
- [X] T020 [P] [US2] Failing test: resend usecase returns not-found for a non-existent `virtualAccountNo`, in internal/usecase/resend_callback_usecase_test.go
- [X] T021 [P] [US2] Failing test: resend usecase returns a distinct error when no prior delivery record exists for the VA (FR-015), in internal/usecase/resend_callback_usecase_test.go
- [X] T022 [P] [US2] Failing test: resend usecase returns a distinct error when the VA has no `notification_url` (FR-016), in internal/usecase/resend_callback_usecase_test.go
- [X] T023 [P] [US2] Failing test: resend usecase never calls `UpdateVAStatus` (transaction state unchanged, FR-019), in internal/usecase/resend_callback_usecase_test.go
- [X] T024 [P] [US2] Failing HTTP contract test: `POST /admin/transactions/:virtualAccountNo/resend-callback` returns 200/404/422/401 per contracts/resend-callback.md, including rejection without `X-Admin-API-Key`, in internal/adapter/delivery/http/handler/admin_resend_handler_test.go

### Implementation for User Story 2

- [X] T025 [US2] Add `ResendCallbackUsecase` interface (`Resend(ctx, virtualAccountNo string) (*domain.ResendCallbackResult, error)`) and `ResendCallbackResult` struct to internal/domain/va.go
- [X] T026 [US2] Implement `resendCallbackUsecase` in internal/usecase/resend_callback_usecase.go: look up VA (404 if absent), fetch latest delivery record via `GetLatestByVirtualAccountNo` (422 if none), verify `notification_url` present (422 if absent), rebuild payload from current VA state with the recorded `event_type`, call `notifier.EnqueuePaymentNotification`, and `Create` a new `trigger: "manual"` delivery row — depends on T019, T020, T021, T022, T023
- [X] T027 [US2] Implement `AdminResendHandler.Resend` in internal/adapter/delivery/http/handler/admin_resend_handler.go mapping usecase results/errors to the HTTP responses in contracts/resend-callback.md — depends on T024, T026
- [X] T028 [US2] Wire `adminGroup.POST("/transactions/:virtualAccountNo/resend-callback", adminResendHandler.Resend)` into the existing `adminGroup` (already behind `AdminAuthMiddleware`) in cmd/api/main.go, and construct `adminResendHandler` via DI alongside the existing admin/VA handler wiring — depends on T027
- [X] T029 [US2] Add OTel span + structured log fields (`virtual_account_no`, `event_type`, `trigger: "manual"`) around the resend usecase, consistent with constitution Principle VIII

**Checkpoint**: User Stories 1 AND 2 both work independently — expiry callbacks fire automatically, and operators can resend any transaction's most recent callback on demand.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Validate the full feature end-to-end and close out constitution quality gates.

- [X] T030 [P] Update Swagger/OpenAPI annotations (swaggo) for the new admin resend endpoint and the expired-response shapes on the existing inquiry/notify endpoints, in internal/adapter/delivery/http/handler/va_handler.go and the new admin_resend_handler.go
- [X] T031 Run `go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` and confirm ≥90% coverage on touched packages (internal/usecase, internal/adapter/delivery/http/handler, internal/adapter/delivery/worker, internal/infrastructure/database) per constitution Principle XI
- [X] T032 Run `golangci-lint run` and resolve all warnings across changed files
- [ ] T033 Execute all 5 scenarios in specs/007-merchant-expiry-callback/quickstart.md end-to-end against the local dev stack and check off the Validation checklist

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS both user stories (T002 migration must run before T007 repo implementation; T004/T005/T006 domain changes are prerequisites for all US1/US2 test and implementation tasks)
- **User Story 1 (Phase 3)**: Depends on Foundational completion. No dependency on US2.
- **User Story 2 (Phase 4)**: Depends on Foundational completion. Reuses the `va_notification_deliveries` repository from Phase 2 and the `EventType`-aware payload/worker from US1 (T017) to redeliver `va.expired` events correctly — implement US1 first for a fully working resend of expiry events, though US2's own tests (T019–T024) can be written in parallel with US1.
- **Polish (Phase 5)**: Depends on both user stories being complete.

### Within Each User Story

- Tests (T008–T013, T019–T024) MUST be written and confirmed failing before their corresponding implementation tasks
- Domain/entity changes (Phase 2) before usecase logic before handler/worker wiring
- Story complete and checkpoint-validated before moving to the next priority

### Parallel Opportunities

- T003, T004, T005, T006 (Phase 2) can run in parallel — different files/concerns
- T008–T013 (US1 tests) can all run in parallel — same test file but independent test functions with no shared mutable state
- T019–T024 (US2 tests) can all run in parallel
- Once Phase 2 is complete, US1 (Phase 3) and US2 test-writing (T019–T024) can proceed in parallel; US2 implementation (T025–T029) is best sequenced after US1's T017 lands so the resend path exercises a real `va.expired`-capable worker

---

## Parallel Example: User Story 1

```bash
# Launch all US1 tests together (after Phase 2 is complete):
Task: "Failing test: VAUsecase.Inquiry returns 4042419 when expired, in internal/usecase/va_usecase_test.go"
Task: "Failing test: VAUsecase.Payment returns 4042519 when expired, in internal/usecase/va_usecase_test.go"
Task: "Failing test: dedupe prevents a second va.expired notification, in internal/usecase/va_usecase_test.go"
Task: "Failing test: no notification_url skips notification but still marks expired, in internal/usecase/va_usecase_test.go"
Task: "Failing test: concurrent payment takes precedence over expiry, in internal/usecase/va_usecase_test.go"
Task: "Failing test: worker signs and delivers va.expired payload correctly, in internal/adapter/delivery/worker/payment_notification_worker_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks both stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run quickstart.md Scenarios 1–4 independently
5. Deploy/demo if ready — merchants already get expiry callbacks at this point, even without the resend endpoint

### Incremental Delivery

1. Complete Setup + Foundational → `va_notification_deliveries` table and domain contracts ready
2. Add User Story 1 → validate Scenarios 1–4 → deploy (MVP — automatic expiry callbacks live)
3. Add User Story 2 → validate Scenario 5 → deploy (operators can now resend on demand)
4. Phase 5 Polish closes out coverage/lint/docs gates before final release

---

## Notes

- [P] tasks = different files or independent test functions, no shared dependency
- [Story] label maps task to specific user story for traceability
- Tests are mandatory here per constitution Principle III (TDD) — do not skip the "confirm it fails" step
- Commit after each task or logical group
- Stop at either checkpoint (end of Phase 3, end of Phase 4) to validate independently
- No background scheduler/cron is introduced anywhere in this task list — expiry detection is intentionally on-access only, per research.md Decision 1
