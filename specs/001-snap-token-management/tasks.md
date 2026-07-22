---
description: "Task list for SNAP Token Management Service & Endpoint implementation"
---

# Tasks: SNAP Token Management Service & Endpoint

**Input**: Design documents from `/specs/001-snap-token-management/`

**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/snap-token-b2b.json, quickstart.md

**Tests**: Test-Driven Development (TDD) is MANDATORY per Constitution Principle III. All test tasks MUST be written and verified failing before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Explicit file paths included in all descriptions.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, Go module configuration, and container layout

- [x] T001 Initialize Go module dependencies and create `go.mod`
- [x] T002 [P] Create Clean Architecture directory layout in `cmd/api/`, `internal/domain/`, `internal/usecase/`, `internal/adapter/`, `internal/infrastructure/`, and `db/migrations/`
- [x] T003 [P] Configure multi-stage non-root Docker build in `Dockerfile` and `docker-compose.yml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core database migrations, shared domain errors, infrastructure drivers, and Echo middleware

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Create PostgreSQL migration scripts in `db/migrations/000001_create_client_apps.up.sql` and `000002_create_client_keys.up.sql`
- [x] T005 [P] Define domain error types and interfaces in `internal/domain/errors.go`
- [x] T006 [P] Implement PostgreSQL connection pool in `internal/infrastructure/database/postgres.go`
- [x] T007 [P] Implement Redis client and distributed locking helper in `internal/infrastructure/redis/redis.go`
- [x] T008 [P] Implement OpenTelemetry tracer and OTLP exporter in `internal/infrastructure/telemetry/otel.go`
- [x] T009 [P] Implement Echo Idempotency-Key middleware with Redis SETNX locking in `internal/adapter/delivery/http/middleware/idempotency.go`
- [x] T010 [P] Implement Echo OTel tracing and structured JSON logger middleware in `internal/adapter/delivery/http/middleware/telemetry.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Request B2B Access Token via SNAP API (Priority: P1) 🎯 MVP

**Goal**: Expose SNAP-compliant B2B token endpoint (`POST /v1.0/access-token/b2b`) with RSA-SHA256 signature verification and signed JWT token issuance.

**Independent Test**: Execute `curl -X POST /v1.0/access-token/b2b` with valid `X-SIGNATURE`, `X-TIMESTAMP`, and `X-CLIENT-KEY` headers; verify HTTP 200 OK with `accessToken`, `tokenType: "Bearer"`, and `expiresIn: "900"`.

### Tests for User Story 1 (TDD Mandatory) ⚠️

- [x] T011 [P] [US1] Unit test for RSA-SHA256 signature verifier in `internal/infrastructure/crypto/rsa_verifier_test.go`
- [x] T012 [P] [US1] Unit test for JWT token issuer in `internal/infrastructure/crypto/jwt_issuer_test.go`
- [x] T013 [P] [US1] Unit test for Token Usecase in `internal/usecase/token_usecase_test.go`
- [x] T014 [P] [US1] Contract & Integration test for SNAP B2B Token HTTP handler in `internal/adapter/delivery/http/handler/token_handler_test.go`

### Implementation for User Story 1

- [x] T015 [P] [US1] Define Client & Token domain models and interfaces in `internal/domain/client.go` and `internal/domain/token.go`
- [x] T016 [P] [US1] Implement RSA-SHA256 signature verifier in `internal/infrastructure/crypto/rsa_verifier.go`
- [x] T017 [P] [US1] Implement RSA signed JWT token issuer in `internal/infrastructure/crypto/jwt_issuer.go`
- [x] T018 [US1] Implement PostgreSQL ClientApp and ClientKey repository queries in `internal/infrastructure/database/client_repository.go`
- [x] T019 [US1] Implement `GenerateB2BToken` use case in `internal/usecase/token_usecase.go`
- [x] T020 [US1] Implement Echo HTTP handler for `POST /v1.0/access-token/b2b` in `internal/adapter/delivery/http/handler/token_handler.go`
- [x] T021 [US1] Wire HTTP routes, middleware, and dependencies in `cmd/api/main.go`

**Checkpoint**: At this point, User Story 1 (MVP) is fully functional and independently testable!

---

## Phase 4: User Story 2 - Client Credential & Public Key Management (Priority: P2)

**Goal**: Provision and manage partner client credentials and RSA public keys with Redis caching.

**Independent Test**: Register a new client public key and verify immediate verification capability during token generation requests.

### Tests for User Story 2 (TDD Mandatory) ⚠️

- [x] T022 [P] [US2] Unit test for Client Usecase in `internal/usecase/client_usecase_test.go`
- [x] T023 [P] [US2] Integration test for Client Repository in `internal/infrastructure/database/client_repository_test.go`

### Implementation for User Story 2

- [x] T024 [US2] Implement `RegisterClient` and `RevokeClientKey` in `internal/usecase/client_usecase.go`
- [x] T025 [US2] Implement Redis client public key cache store in `internal/infrastructure/redis/client_key_cache.go`

**Checkpoint**: User Stories 1 AND 2 operate independently and seamlessly together

---

## Phase 5: User Story 3 - Access Token Validation & Cache Storage (Priority: P3)

**Goal**: Rapid sub-millisecond validation of active JWT tokens against Redis session cache.

**Independent Test**: Validate active JWT token against Redis cache store and verify invalidation upon expiration or revocation.

### Tests for User Story 3 (TDD Mandatory) ⚠️

- [x] T026 [P] [US3] Unit test for Token Validation Usecase in `internal/usecase/token_validation_test.go`
- [x] T027 [P] [US3] Integration test for Redis Token Session Store in `internal/infrastructure/redis/token_session_store_test.go`

### Implementation for User Story 3

- [x] T028 [US3] Implement Redis token session store in `internal/infrastructure/redis/token_session_store.go`
- [x] T029 [US3] Implement `ValidateToken` in `internal/usecase/token_usecase.go`

**Checkpoint**: All user stories are fully functional and independently testable

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Quality checks, linting, validation scripts, and final verification

- [x] T030 [P] Configure golangci-lint in `.golangci.yml`
- [x] T031 [P] Create end-to-end validation script in `scripts/validate-snap-token.sh`
- [x] T032 Run full test suite (`go test -race ./...`) and verify clean non-root Docker build execution

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Completed ✅
- **Foundational (Phase 2)**: Completed ✅
- **User Story 1 (Phase 3)**: Completed ✅
- **User Story 2 (Phase 4)**: Completed ✅
- **User Story 3 (Phase 5)**: Completed ✅
- **Polish (Phase 6)**: Completed ✅
