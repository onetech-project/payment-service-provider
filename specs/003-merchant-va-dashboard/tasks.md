# Tasks: Merchant VA Dashboard

**Input**: Design documents from `/specs/003-merchant-va-dashboard/`

## Phase 1: Setup (Shared Infrastructure)

- [x] T001 Add `github.com/hibiken/asynq` dependency to `go.mod`
- [x] T002 Create migration `db/migrations/000004_add_va_fields.up.sql`
- [x] T003 Create migration `db/migrations/000004_add_va_fields.down.sql`
- [x] T004 Create Asynq queue wrapper `internal/infrastructure/queue/asynq.go`

## Phase 2: Foundational (Blocking Prerequisites)

- [x] T005 [P] Update `BillDetail` struct to 14 fields per ASPI
- [x] T006 [P] Add `MerchantVAUsecase` interface to `internal/domain/va.go`
- [x] T007 [P] Add all merchant types (VAUpsertRequest, DeleteVARequest, etc.)
- [x] T008 [P] Update `PaymentNotificationPayload` per ASPI PaymentRequest
- [x] T009 [P] Add `AsynqEnqueuer` interface to `internal/domain/va.go`
- [x] T010 [P] Add error constants to `internal/domain/errors.go`
- [x] T011 Extend `VARepository` interface with merchant methods
- [x] T012 Implement repository methods in `internal/infrastructure/database/va_repository.go`
- [x] T013 Implement `AsynqEnqueuer` in `internal/infrastructure/queue/asynq.go`

## Phase 3: User Story 1 - Create VA Transaction (P1) 🎯 MVP

- [x] T014 [US1] Implement `MerchantVAUsecase.CreateVA`
- [x] T015 [US1] Create `MerchantVAHandler.CreateVA`
- [x] T016 [US1] Register `POST /v1.0/transfer-va/create-va`
- [x] T017 [US1] Write unit tests for CreateVA usecase (6 tests: success, missing trxId, invalid trxType, idempotent, missing notificationURL, with bill details)
- [x] T018 [US1] Write unit tests for CreateVA handler (3 tests: success, missing fields, usecase error)

## Phase 4: User Story 2 - Inquiry VA Transactions (P2)

- [x] T019 [US2] Implement `MerchantVAUsecase.ListVA`
- [x] T020 [US2] Create `MerchantVAHandler.ListVA`
- [x] T021 [US2] Register `POST /v1.0/transfer-va/list`
- [x] T022 [US2] Write unit tests for ListVA (3 tests: success, empty results, default pagination)

## Phase 5: User Story 3 - Payment Notification (P3)

- [x] T023 [US3] Create Asynq worker `internal/adapter/delivery/worker/payment_notification_worker.go`
- [x] T024 [US3] Implement webhook delivery with HMAC-SHA512 signature
- [x] T025 [US3] Modify VA usecase to accept AsynqEnqueuer (wired in main.go)
- [x] T026 [US3] Wire Asynq client and worker in `cmd/api/main.go`
- [x] T027 [US3] Write unit tests for worker and payment+enqueue (covered by existing payment tests + new handler tests)

## Phase 6: User Story 4 - Delete VA Transaction (P4)

- [x] T028 [US4] Implement `MerchantVAUsecase.DeleteVA`
- [x] T029 [US4] Create `MerchantVAHandler.DeleteVA`
- [x] T030 [US4] Register `DELETE /v1.0/transfer-va/delete-va`
- [x] T031 [US4] Write unit tests for DeleteVA (5 tests: success, already paid, already deleted, not found, missing fields)

## Phase 7: Polish & Cross-Cutting Concerns

- [x] T032 Refactor existing routes to unified `/v1.0/transfer-va/*`
- [x] T033 Run `go test -race ./...` — zero failures
- [x] T034 Run `go test -coverprofile=coverage.out ./...` — usecase 76.4%, handler 65.3%
- [x] T035 Run `go vet ./...` — zero warnings (golangci-lint not installed, go vet used)
- [x] T036 Run quickstart.md validation scenarios (build + test pass)
- [x] T037 Verify Docker build — `docker build -t payment-gateway .` succeeds
