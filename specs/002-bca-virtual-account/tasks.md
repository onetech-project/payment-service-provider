# Tasks: BCA Virtual Account Integration

**Input**: Design documents from `/specs/002-bca-virtual-account/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are included per Constitution Principle III (TDD) and Principle XI (90% coverage).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and configuration

- [x] T001 Create BCA VA domain entities and interfaces in internal/domain/va.go
- [x] T002 [P] Add BCA VA error constants to internal/domain/errors.go
- [x] T003 [P] Create BCA VA configuration loader in internal/infrastructure/config/bca_va_config.go
- [x] T004 [P] Create HMAC-SHA512 signer in internal/infrastructure/crypto/hmac.go
- [x] T005 [P] Create database migration 000003_create_va_transactions.up.sql in db/migrations/
- [x] T006 [P] Create database migration 000003_create_va_transactions.down.sql in db/migrations/

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T007 Implement VA repository interface in internal/infrastructure/database/va_repository.go
- [x] T008 [P] Create BCA VA HTTP handler in internal/adapter/delivery/http/handler/va_handler.go
- [x] T009 [P] Create generic SNAP auth middleware in internal/adapter/delivery/http/middleware/snap_auth.go
- [x] T010 Create VA usecase in internal/usecase/va_usecase.go
- [x] T011 [P] Write unit tests for HMAC signer in internal/infrastructure/crypto/hmac_test.go
- [x] T012 [P] Write unit tests for BCA VA config loader in internal/infrastructure/config/bca_va_config_test.go
- [x] T013 [P] Write unit tests for VA repository in internal/infrastructure/database/va_repository_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin

### Additional Foundation Tasks (Vendor Configuration)

- [x] T037 [P] Create generic VendorConfig loader in internal/infrastructure/config/vendor_config.go
- [x] T038 [P] Create .env and .env.example for global configuration
- [x] T039 [P] Create .env.vendor.channel.example template
- [x] T040 [P] Update .gitignore to exclude .env files but keep .example files
- [x] T041 Update main.go to use generic vendor config and middleware
- [x] T042 [P] Create generic SNAP gateway client in internal/adapter/gateway/snap/client.go
- [x] T043 Remove BCA-specific files (bca_va_config.go, bca_auth.go, bca/ folder, .env.bca.va.example)

---

## Phase 3: User Story 1 - VA Bill Inquiry (Priority: P1) MVP

**Goal**: Receive and process Virtual Account inquiry requests from BCA for bill presentment

**Independent Test**: Send mock inquiry request with valid VA number and verify response contains correct bill details, amounts, and customer information

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T014 [P] [US1] Write unit tests for VA inquiry usecase in internal/usecase/va_usecase_test.go
- [x] T015 [P] [US1] Write unit tests for VA handler inquiry endpoint in internal/adapter/delivery/http/handler/va_handler_test.go

### Implementation for User Story 1

- [x] T016 [US1] Implement VAInquiry method in internal/usecase/va_usecase.go
- [x] T017 [US1] Implement GetB2BAccessToken handler method for VA inquiry in internal/adapter/delivery/http/handler/va_handler.go
- [x] T018 [US1] Wire VA routes in cmd/api/main.go
- [x] T019 [US1] Validate inquiry acceptance scenarios pass (valid VA, invalid VA, paid bill)

**Checkpoint**: VA Bill Inquiry is fully functional and testable independently

---

## Phase 4: User Story 2 - VA Payment Notification (Priority: P2)

**Goal**: Receive and process payment notifications from BCA to record payments

**Independent Test**: Send mock payment notification with valid transaction data and verify system acknowledges payment correctly

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T020 [P] [US2] Write unit tests for VA payment usecase in internal/usecase/va_usecase_test.go
- [x] T021 [P] [US2] Write unit tests for VA handler payment endpoint in internal/adapter/delivery/http/handler/va_handler_test.go

### Implementation for User Story 2

- [x] T022 [US2] Implement VAPayment method in internal/usecase/va_usecase.go
- [x] T023 [US2] Implement GetB2BAccessToken handler method for VA payment in internal/adapter/delivery/http/handler/va_handler.go
- [x] T024 [US2] Wire VA payment route in cmd/api/main.go
- [x] T025 [US2] Validate payment acceptance scenarios pass (success, duplicate, amount mismatch)

**Checkpoint**: VA Payment Notification is fully functional and testable independently

---

## Phase 5: User Story 3 - VA Payment Status Inquiry (Priority: P3)

**Goal**: Check payment status of VA transactions for reconciliation

**Independent Test**: Send mock status inquiry request and verify response contains correct payment flag status

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T026 [P] [US3] Write unit tests for VA status usecase in internal/usecase/va_usecase_test.go
- [x] T027 [P] [US3] Write unit tests for VA handler status endpoint in internal/adapter/delivery/http/handler/va_handler_test.go

### Implementation for User Story 3

- [x] T028 [US3] Implement VAStatus method in internal/usecase/va_usecase.go
- [x] T029 [US3] Implement GetB2BAccessToken handler method for VA status in internal/adapter/delivery/http/handler/va_handler.go
- [x] T030 [US3] Wire VA status route in cmd/api/main.go
- [x] T031 [US3] Validate status inquiry acceptance scenarios pass (success, pending, reject)

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T032 [P] Verify test coverage meets 90% threshold via go test -coverprofile
- [x] T033 [P] Run golangci-lint and fix any warnings
- [x] T034 Verify Docker build succeeds with multi-stage non-root container
- [x] T035 Verify quickstart.md validation scenarios pass end-to-end
- [x] T036 Update README.md with BCA VA integration section

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 -> P2 -> P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- UseCase before Handler
- Handler before Routes
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Write unit tests for VA inquiry usecase in internal/usecase/va_usecase_test.go"
Task: "Write unit tests for VA handler inquiry endpoint in internal/adapter/delivery/http/handler/va_handler_test.go"

# Then implementation:
Task: "Implement VAInquiry method in internal/usecase/va_usecase.go"
Task: "Implement GetB2BAccessToken handler method for VA inquiry in internal/adapter/delivery/http/handler/va_handler.go"
Task: "Wire VA routes in cmd/api/main.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Deploy/Demo
4. Add User Story 3 -> Test independently -> Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Coverage target: 90%+ per Constitution Principle XI
