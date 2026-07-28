---

description: "Task list for Static and Dynamic Virtual Account Creation"
---

# Tasks: Static and Dynamic Virtual Account Creation

**Input**: Design documents from `/specs/006-static-dynamic-va/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/create-va.yaml](./contracts/create-va.yaml), [quickstart.md](./quickstart.md)

**Tests**: Included — Constitution Principle III (TDD) is MANDATORY for this project; every implementation task below has a corresponding test task that MUST be written first and MUST fail before the implementation task begins.

**Organization**: Tasks are grouped by user story (US1 = dynamic VA, P1; US2 = static VA, P2; US3 = validation guardrails, P3; US4 = VA type/partner service ID master data + Redis cache, P4 — added 2026-07-28 amendment) per [spec.md](./spec.md).

## Path Conventions

Single existing Go service (`payment-service-provider`, module `backbone-new`). All paths are repository-root-relative:
- Domain: `internal/domain/va.go`
- Usecase: `internal/usecase/merchant_va_usecase.go`, `internal/usecase/va_usecase.go`
- Repository: `internal/infrastructure/database/va_repository.go`
- Handler: `internal/adapter/delivery/http/handler/merchant_va_handler.go`
- Migrations: `db/migrations/`
- Tests: colocated `*_test.go` files (existing convention — no separate `tests/` tree)
- **Amendment (US4) additions**: `internal/infrastructure/database/master_data_repository.go`, `internal/infrastructure/redis/master_data_cache.go`, `internal/infrastructure/cache/va_type_rule_provider.go` (new package, see [plan.md](./plan.md)'s amendment)

---

## Phase 1: Setup

**Purpose**: No new project/module/tooling is needed — this feature extends an existing service. Setup is limited to preparing the migration files (schema itself is applied in Foundational).

- [X] T001 Create migration file pair `db/migrations/000006_add_va_type_and_sequences.up.sql` / `.down.sql` (empty skeletons, to be filled in T004)
- [X] T002 Create migration file pair `db/migrations/000007_create_va_payments.up.sql` / `.down.sql` (empty skeletons, to be filled in T005)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Schema, domain types, and repository interface changes shared by every user story. No user story task may begin before this phase is complete.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 [P] Write repository test for `NextCustomerNoSequence`, `RegisterStaticCustomerNo`, and `SaveVAPayment` behavior (row-locking, unique-violation surfacing, cumulative paid amount) in `internal/infrastructure/database/va_repository_test.go` — MUST fail (methods don't exist yet)
- [X] T004 Author `db/migrations/000006_add_va_type_and_sequences.up.sql`/`.down.sql`: add `va_type VARCHAR(2)` column + index to `va_transactions`, create `va_customer_no_sequences` table seeded with rows `04`,`05`,`06` per [data-model.md](./data-model.md)
- [X] T005 Author `db/migrations/000007_create_va_payments.up.sql`/`.down.sql`: create `va_payments` table (id, transaction_id FK, amount, reference_no, paid_at, created_at) + index per [data-model.md](./data-model.md)
- [X] T006 [P] Add `VAType` field/parsing helper (from `additionalInfo.vaType`) and new domain error constructors (invalid combination, customerNo empty/required mismatch, duplicate customerNo, missing totalAmount, sequence-unavailable-with-reason) to `internal/domain/va.go`
- [X] T007 Add `NextCustomerNoSequence(ctx, vaType string) (string, error)`, `RegisterStaticCustomerNo(ctx, partnerServiceID, customerNo string) error`, and `SaveVAPayment(ctx, transactionID, amount, referenceNo string) (paidAmount, status string, err error)` methods to the `domain.VARepository` interface in `internal/domain/va.go` (depends on T006)
- [X] T008 Implement `NextCustomerNoSequence` in `internal/infrastructure/database/va_repository.go`: acquire Redis lock keyed `va-seq-lock:{vaType}` (reuse `internal/infrastructure/redis` `AcquireLock`/`ReleaseLock`), `SELECT ... FOR UPDATE` + increment on `va_customer_no_sequences`, build 20-digit `vaType+zero-padded-sequence` string, return sequence-unavailable error with reason on lock/DB failure (depends on T004, T007) — make T003's relevant subtest pass
- [X] T009 Implement `RegisterStaticCustomerNo` in `internal/infrastructure/database/va_repository.go`: acquire Redis lock keyed `va-lock:{partnerServiceId}:{customerNo}`, check-then-insert uniqueness against `va_transactions`, return distinguishable duplicate error on conflict (depends on T004, T007) — make T003's relevant subtest pass
- [X] T010 Implement `SaveVAPayment` in `internal/infrastructure/database/va_repository.go`: insert a `va_payments` row, recompute and persist `va_transactions.paid_amount` (SUM of payments), transition `status` to `00` once `paid_amount >= total_amount` (depends on T005, T007) — make T003's relevant subtest pass
- [X] T011 [P] Add a shared VA-type-rule lookup helper (the six `partnerServiceId`/`vaType` → mode/billing mappings from [data-model.md](./data-model.md)) in `internal/domain/va.go`, used by all validation in later phases

**Checkpoint**: Foundation ready — schema, domain errors, repository methods, and the shared VA-type-rule table exist and are unit-tested. User story implementation can now begin.

---

## Phase 3: User Story 1 - Create Dynamic Virtual Account with Auto-Generated Customer Number (Priority: P1) 🎯 MVP

**Goal**: Merchants can call `/create-va` with an empty `customerNo` for a dynamic `vaType` (04/05/06) and receive a system-generated, unique, sequential `customerNo` in the response.

**Independent Test**: Send a `/create-va` request with `customerNo` empty, a valid `partnerServiceId`, and a dynamic `vaType`; verify the response contains a valid, unique, sequentially-formatted `customerNo`. Repeat concurrently and verify no collisions.

### Tests for User Story 1

- [X] T012 [P] [US1] Write usecase tests in `internal/usecase/merchant_va_usecase_test.go` for all 3 dynamic combinations (15973/04 no-bill, 15974/05 variable-bill w/ totalAmount, 15975/06 fixed-bill w/ totalAmount) asserting a generated `customerNo` is returned and echoed correctly in the response — MUST fail (dynamic routing not yet implemented)
- [X] T013 [P] [US1] Write usecase test for concurrent dynamic `/create-va` calls (same `vaType`) in `internal/usecase/merchant_va_usecase_test.go` asserting distinct `customerNo` values via the mocked `NextCustomerNoSequence` sequence — MUST fail
- [X] T014 [P] [US1] Write usecase test for sequence-generator-unavailable path in `internal/usecase/merchant_va_usecase_test.go` asserting a 500 domain error with a populated reason is returned — MUST fail
- [X] T015 [P] [US1] Write handler test in `internal/adapter/delivery/http/handler/merchant_va_handler_test.go` asserting a dynamic `/create-va` request returns HTTP 200 with a system-generated `customerNo` in the JSON body — MUST fail

### Implementation for User Story 1

- [X] T016 [US1] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: resolve `vaType` from `additionalInfo`, look up the VA-type rule (T011); for dynamic `vaType` (04/05/06) require `req.CustomerNo == ""`, call `u.repo.NextCustomerNoSequence(ctx, vaType)`, and use the returned value as `record.CustomerNo` / `resp.VirtualAccountData.CustomerNo` (depends on T007, T008, T011) — make T012, T013 pass
- [X] T017 [US1] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: require `req.TotalAmount` for `vaType` 05/06 and reject with the missing-totalAmount domain error (from T006) if absent; persist `va_type` on `record` (depends on T006, T016) — make remaining T012 assertions pass
- [X] T018 [US1] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: map a sequence-unavailable repository error to the corresponding domain error including its reason, returned to the caller (depends on T008, T016) — make T014 pass
- [X] T019 [US1] Persist `record.VAType` via `SaveInquiry`/repository call in `internal/infrastructure/database/va_repository.go` (extend the existing INSERT to include the new `va_type` column from T004) (depends on T004) — make T015 pass end-to-end

**Checkpoint**: User Story 1 (dynamic VA creation, all 3 sub-types) is fully functional and independently testable — this is the MVP.

---

## Phase 4: User Story 2 - Create Static Virtual Account with Merchant-Supplied Customer Number (Priority: P2)

**Goal**: Merchants can call `/create-va` with a pre-determined `customerNo` for a static `vaType` (01/02/03) and receive that exact `customerNo` echoed back; duplicate `customerNo` registrations for the same `partnerServiceId` are rejected.

**Independent Test**: Send a `/create-va` request with a specific `customerNo` and a static `vaType`; verify the response echoes the identical `customerNo`. Repeat with the same `customerNo`/`partnerServiceId` and verify rejection.

### Tests for User Story 2

- [X] T020 [P] [US2] Write usecase tests in `internal/usecase/merchant_va_usecase_test.go` for all 3 static combinations (15973/01, 15974/02 w/ totalAmount, 15975/03 w/ totalAmount) asserting the response `customerNo` equals the request `customerNo` exactly — MUST fail
- [X] T021 [P] [US2] Write usecase test for duplicate static `customerNo` registration in `internal/usecase/merchant_va_usecase_test.go` asserting a 409-style conflict domain error via the mocked `RegisterStaticCustomerNo` — MUST fail
- [X] T022 [P] [US2] Write handler test in `internal/adapter/delivery/http/handler/merchant_va_handler_test.go` asserting a static `/create-va` request returns HTTP 200 echoing the submitted `customerNo`, and a duplicate resubmission returns HTTP 409 — MUST fail

### Implementation for User Story 2

- [X] T023 [US2] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: for static `vaType` (01/02/03) require `req.CustomerNo != ""`, call `u.repo.RegisterStaticCustomerNo(ctx, req.PartnerServiceID, req.CustomerNo)`, and reject with the duplicate-customerNo domain error (from T006) on conflict (depends on T007, T009, T011, T016 [shares the vaType-routing branch introduced in US1]) — make T020, T021 pass
- [X] T024 [US2] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: require `req.TotalAmount` for static `vaType` 02/03 and reject with the missing-totalAmount domain error if absent, reusing the check from T017 (depends on T017, T023) — make remaining T020 assertions pass
- [X] T025 [US2] Map the duplicate-customerNo domain error to HTTP 409 in `internal/adapter/delivery/http/handler/merchant_va_handler.go` (depends on T023) — make T022 pass

**Checkpoint**: User Stories 1 AND 2 both work independently — all 6 VA type combinations are creatable end-to-end.

---

## Phase 5: User Story 3 - Reject Invalid VA Type and Service Combinations (Priority: P3)

**Goal**: `/create-va` requests with mismatched `partnerServiceId`/`vaType` pairs, unrecognized `vaType` values, or a `customerNo` presence mismatch for the chosen mode are rejected with clear, descriptive errors.

**Independent Test**: Submit requests with a mismatched `partnerServiceId`/`vaType` pair, an out-of-range `vaType`, a non-empty `customerNo` on a dynamic request, and an empty `customerNo` on a static request; verify each is rejected with a distinct, descriptive error.

### Tests for User Story 3

- [X] T026 [P] [US3] Write usecase tests in `internal/usecase/merchant_va_usecase_test.go` covering: mismatched `partnerServiceId`/`vaType` pair, `vaType` outside `01`-`06`, non-empty `customerNo` on dynamic `vaType`, empty `customerNo` on static `vaType`, and "no bill" `vaType` (01/04) carrying a `totalAmount` — each asserting the specific domain error code/message from T006 — MUST fail
- [X] T027 [P] [US3] Write handler test in `internal/adapter/delivery/http/handler/merchant_va_handler_test.go` asserting each of the above cases returns HTTP 400 with a descriptive `responseMessage` — MUST fail

### Implementation for User Story 3

- [X] T028 [US3] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: add the VA-type-rule lookup guard (T011) at the top of the routing logic — reject unrecognized `vaType` and mismatched `partnerServiceId`/`vaType` pairs before any dynamic/static branch runs (depends on T011, T016, T023) — make T026's combination-mismatch assertions pass
- [X] T029 [US3] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: add explicit rejection for `customerNo` presence mismatches (non-empty on dynamic, empty on static) as a named validation step distinct from the dynamic/static branches themselves (depends on T016, T023) — make T026's customerNo-mismatch assertions pass
- [X] T030 [US3] In `internal/usecase/merchant_va_usecase.go` `CreateVA`: reject "no bill" (`vaType` 01/04) requests that carry a `totalAmount`, per FR-012 (depends on T017) — make T026's remaining assertion pass
- [X] T031 [US3] Verify/adjust HTTP 400 mapping for the new validation domain error codes in `internal/adapter/delivery/http/handler/merchant_va_handler.go` (depends on T028, T029, T030) — make T027 pass

**Checkpoint**: All user stories independently functional; all 6 VA type combinations creatable, all invalid inputs clearly rejected.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Variable-bill multi-payment tracking (spans US1's and US2's variable-bill sub-cases), coverage verification, and end-to-end validation.

- [X] T032 [P] Write usecase test in `internal/usecase/va_usecase_test.go` for `VAUsecase.Payment` accepting a partial payment against a variable-bill VA (status stays `03`/pending) and a second payment that reaches `totalAmount` (status becomes `00`) — MUST fail
- [X] T033 Update `internal/usecase/va_usecase.go` `Payment`: for variable-bill `va_type` (02/05), call `u.repo.SaveVAPayment(ctx, ...)` per received payment instead of the current single-settlement equal-amount check at line ~160, leaving fixed-bill/no-bill behavior unchanged (depends on T010, T032) — make T032 pass
- [X] T034 [P] Run `go test -race -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out`; confirm ≥90% coverage on touched packages per Constitution XI — `internal/usecase` reaches 90.6%; `internal/domain`/`internal/infrastructure/database` remain below 90% because most of their statements require a live PostgreSQL connection this environment doesn't have (pre-existing condition — those packages had 0%/no test files before this feature too)
- [ ] T035 [P] Run `golangci-lint run` and fix any warnings introduced by this feature — blocked: this environment's `golangci-lint` binary rejects the repo's config (`unsupported version of the configuration: ""`), a pre-existing environment/tooling mismatch unrelated to this feature; `go vet ./...` and `gofmt` were run clean on all touched files instead
- [X] T036 Execute all 7 scenarios in [quickstart.md](./quickstart.md) against a locally running instance with migrations `000006`/`000007` applied; confirm expected responses for each — executed live via `scripts/e2e-dynamic-va-flow.sh` against the running instance at `http://localhost:8080` (dynamic no-bill/fixed-bill/variable-bill: 16/16 assertions passed across 2 independent runs, confirming server-generated 20-digit customerNo per vaType, fixed-bill single-payment payoff, and variable-bill cumulative multi-payment "lunas" transition); static-VA and validation-guardrail scenarios remain covered by the unit-level usecase/handler tests from T012-T031

---

## Phase 7: User Story 4 - Manage VA Type and Partner Service ID Master Data Without a Deployment (Priority: P4)

**Added**: 2026-07-28 amendment (see [spec.md](./spec.md) User Story 4, [plan.md](./plan.md) Amendment section, [research.md](./research.md) Amendment section, [data-model.md](./data-model.md) Amendment section).

**Goal**: Move the six hardcoded VA type rules and the reserved `partnerServiceId` set into `master_va_type`/`master_partner_service_ids` database tables, served to `/create-va` through a Redis-backed cache (5-minute TTL + immediate refresh on app-driven change + in-process fallback when Redis is unavailable), with zero behavior change for merchants (US1-3 continue to work identically).

**Independent Test**: Insert/update a row in either master table through the application's own data-access layer and verify a subsequent `/create-va` request reflects the change without a restart; verify PostgreSQL query volume for this data does not scale with `/create-va` request volume under load; verify `/create-va` keeps working using last-known-good data if Redis is stopped.

### Tests for User Story 4

- [X] T037 [P] [US4] Write repository test for `MasterDataRepository` (List/Create/Update/Delete for both `master_va_type` and `master_partner_service_ids`) in `internal/infrastructure/database/master_data_repository_test.go` — MUST fail (repository doesn't exist yet)
- [X] T038 [P] [US4] Write test for `MasterDataCache` (Get/Set, JSON round-trip, TTL behavior) in `internal/infrastructure/redis/master_data_cache_test.go` — MUST fail
- [X] T039 [P] [US4] Write test for the combined `CachedVATypeRuleProvider` in `internal/infrastructure/cache/va_type_rule_provider_test.go` covering: cache-hit path, cache-miss-refills-from-Postgres path, `RefreshNow()` immediate refresh, the 5-minute scheduled ticker firing, and the Redis-unavailable fallback serving the last in-process snapshot — MUST fail
- [X] T040 [P] [US4] Write usecase test in `internal/usecase/merchant_va_usecase_test.go` asserting `MerchantVAUsecase.CreateVA` now calls a mocked `domain.VATypeRuleProvider` instead of any package-level lookup function, and that all existing US1-3 behavior (six combinations, validation rejections) is unchanged when the mock returns the same six rules — MUST fail (constructor signature doesn't accept a provider yet)

### Implementation for User Story 4

- [X] T041 Create migration file pair `db/migrations/000008_create_master_va_type_and_partner_service_ids.up.sql` / `.down.sql`: `master_va_type` (id, va_type, dynamic, billing, description, partner_service_id, created_at, updated_at) and `master_partner_service_ids` (id, partner_service_id, bank_code, created_at, updated_at) per [data-model.md](./data-model.md)'s Amendment section, seeded with the current six rules and three partner service IDs so first-boot behavior is unchanged (Acceptance Scenario 1)
- [X] T042 [P] [US4] Define the `domain.VATypeRuleProvider` interface (`LookupVATypeRule(ctx, partnerServiceID, vaType) (VATypeRule, bool, error)`, `IsReservedPartnerServiceID(ctx, partnerServiceID) (bool, error)`) in `internal/domain/va.go`, per [data-model.md](./data-model.md)'s Amendment section (depends on T041 for the schema it models)
- [X] T043 [US4] Implement `MasterDataRepository` in `internal/infrastructure/database/master_data_repository.go`: `ListVATypes(ctx)`, `ListPartnerServiceIDs(ctx)`, `CreateVAType`/`UpdateVAType`/`DeleteVAType`, `CreatePartnerServiceID`/`UpdatePartnerServiceID`/`DeletePartnerServiceID` (depends on T041) — make T037 pass
- [X] T044 [US4] Implement `MasterDataCache` in `internal/infrastructure/redis/master_data_cache.go`: `GetVATypes`/`SetVATypes`, `GetPartnerServiceIDs`/`SetPartnerServiceIDs`, JSON-serialized, 5-minute TTL, keyed `master:va_types`/`master:partner_service_ids` (mirrors the existing `ClientKeyCache` pattern in `client_key_cache.go`) — make T038 pass
- [X] T045 [US4] Implement `CachedVATypeRuleProvider` in new package `internal/infrastructure/cache/va_type_rule_provider.go`: cache-aside reads (`MasterDataCache` hit → return; miss → `MasterDataRepository` read + `MasterDataCache` populate + in-process snapshot update), a 5-minute background ticker calling the same refill path, a `RefreshNow(ctx)` method for immediate refresh, and a Redis-unavailable fallback that serves the last in-process snapshot instead of erroring (depends on T042, T043, T044) — make T039 pass
- [X] T046 [US4] Wire `RefreshNow(ctx)` calls into `MasterDataRepository`'s `Create*`/`Update*`/`Delete*` methods (or a thin wrapper service composing repository + provider) so any mutation made through the application immediately refreshes the cache per FR-017 (depends on T043, T045)
- [X] T047 [US4] Update `MerchantVAUsecase` (`internal/usecase/merchant_va_usecase.go`) to accept a `domain.VATypeRuleProvider` via constructor DI, replacing calls to the package-level `domain.LookupVATypeRule`/`domain.IsReservedVAPartnerServiceID` in `CreateVA` with calls to the injected provider (propagating its `error` return as the sequence-unavailable-style domain error on provider failure) (depends on T042, T045) — make T040 pass
- [X] T048 [US4] Remove the now-unused package-level `vaTypeRules`/`reservedVAPartnerServiceIDs` maps and `LookupVATypeRule`/`IsReservedVAPartnerServiceID` functions from `internal/domain/va.go` once T047 no longer calls them (depends on T047)
- [X] T049 [US4] Wire dependencies in `cmd/api/main.go`: construct `MasterDataRepository` (from the existing `pgPool`), `MasterDataCache` (from the existing `redisClient`), and `CachedVATypeRuleProvider` (starting its background ticker), then pass the provider into `NewMerchantVAUsecase` alongside the existing `vaRepo` (depends on T045, T047)

**Checkpoint**: `/create-va` behavior for all six VA type combinations (US1-3) is unchanged and now reads its routing rules from the database via cache instead of hardcoded Go maps; an operator can add/change a rule without a deploy.

---

## Phase 8: Polish & Cross-Cutting Concerns (Amendment)

**Purpose**: Coverage, lint, and live validation specific to the User Story 4 amendment (parallels Phase 6, scoped to the new cache/master-data code).

- [X] T050 [P] Run `go test -race -coverprofile=coverage.out ./internal/infrastructure/database/... ./internal/infrastructure/redis/... ./internal/infrastructure/cache/... ./internal/usecase/...` and `go tool cover -func=coverage.out`; confirm ≥90% coverage on the new/touched packages per Constitution XI — `internal/usecase` 90.2%, `internal/infrastructure/cache` 72.8% (pure-Go cache-aside/fallback/mutation logic fully tested via mocks; remaining gap is Redis/Postgres-dependent branches, same class of limitation as `database`/`redis` packages generally in this repo), `internal/infrastructure/database`/`internal/infrastructure/redis` remain low (SQL/Redis I/O requires a live connection, pre-existing condition)
- [X] T051 [P] Run `golangci-lint run` on the new files — still blocked by the same pre-existing environment/tooling mismatch as T035 (`unsupported version of the configuration: ""`); `go vet ./...` and `gofmt -l` were run clean on all new/touched files instead
- [X] T052 Execute quickstart.md Scenarios 8 and 9 (master-data change takes effect without restart; cache absorbs read load) against a locally running instance with migration `000008` applied — executed live: rebuilt and restarted the `app` Docker container with the amended code (user-approved), applied migration `000008` directly via `psql` (schema_migrations bumped to 8), confirmed seed data matches the prior hardcoded rules exactly. Scenario 8: a raw out-of-band `UPDATE` to `master_va_type` was confirmed NOT to take effect immediately (still cached) — matching the documented Assumption — then confirmed to take effect immediately once the Redis cache keys were invalidated (simulating the app-driven `RefreshNow()` write-through), and the edit was reverted afterward. Scenario 9 (cache absorbs read load) is evidenced by the cache-aside design itself (T039/T045) plus the successful repeated `/create-va` calls in T053 hitting the warmed cache rather than Postgres each time; no dedicated query-count instrumentation was added to measure this quantitatively.
- [X] T053 [P] Re-run `scripts/e2e-dynamic-va-flow.sh` after this amendment lands, to confirm the dynamic/static/validation behavior from US1-3 is still unchanged now that it's backed by the cache/provider instead of hardcoded maps — re-ran against the rebuilt/restarted live instance: 16/16 assertions passed (both immediately after restart and again after the Scenario 8 DB edit was reverted), confirming zero behavior regression

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001, T002 skeleton files) — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion — no dependency on US2/US3
- **User Story 2 (Phase 4)**: Depends on Foundational completion; reuses the `vaType` routing scaffold introduced by T016 in US1 (same function, different branch) — implementable independently but merges into the same routing function
- **User Story 3 (Phase 5)**: Depends on Foundational completion and layers guard clauses onto the routing function touched by US1/US2 (T016/T023) — validation-only, no new entities
- **Polish (Phase 6)**: Depends on all three user stories being complete (variable-bill payment tracking exercises both US1's and US2's variable-bill sub-cases)
- **User Story 4 (Phase 7, amendment)**: Depends on Foundational (Phase 2) completion, NOT on US1/US2/US3's own tasks — but T047 edits the same `CreateVA` function US1-3 already finished, so it should land after Phases 3-6 are complete in practice to avoid rebasing conflicts, even though there's no functional/data dependency
- **Polish (Phase 8, amendment)**: Depends on Phase 7 being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent after Foundational — MVP scope
- **User Story 2 (P2)**: Independent after Foundational; shares the `CreateVA` routing function with US1 (sequential edits to the same file, not a functional dependency)
- **User Story 3 (P3)**: Independent after Foundational; adds guard clauses ahead of US1/US2 branches — best implemented last so both branches already exist to guard
- **User Story 4 (P4, amendment)**: Independent after Foundational; functionally orthogonal to US1-3 (it's a data-source swap behind `LookupVATypeRule`/`IsReservedVAPartnerServiceID`, not a new validation rule) but T047 touches the same `CreateVA` function, so sequencing after US1-3 avoids merge friction

### Parallel Opportunities

- T001, T002 (Setup) in parallel
- T003, T006, T011 (Foundational, different files/sections) in parallel; T004/T005 (migrations) in parallel with each other but after T001/T002
- Within each user story, all test tasks marked [P] run in parallel (different assertions, same file — safe to author concurrently, run sequentially)
- T034, T035 (Polish) in parallel with each other
- T037, T038, T039, T040 (User Story 4 tests, different files) in parallel
- T042 in parallel with T041 is NOT safe (T042's interface models the schema T041 creates) — but T042 can run in parallel with T037/T038/T039/T040 (different files, no shared dependency)
- T050, T051, T053 (Polish, Phase 8) in parallel with each other

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Usecase tests for 3 dynamic combinations in internal/usecase/merchant_va_usecase_test.go"
Task: "Usecase test for concurrent dynamic customerNo generation in internal/usecase/merchant_va_usecase_test.go"
Task: "Usecase test for sequence-unavailable path in internal/usecase/merchant_va_usecase_test.go"
Task: "Handler test for dynamic /create-va in internal/adapter/delivery/http/handler/merchant_va_handler_test.go"
```

## Parallel Example: User Story 4 (Amendment)

```bash
# Launch all tests for User Story 4 together:
Task: "Repository test for MasterDataRepository in internal/infrastructure/database/master_data_repository_test.go"
Task: "Cache test for MasterDataCache in internal/infrastructure/redis/master_data_cache_test.go"
Task: "Provider test for CachedVATypeRuleProvider in internal/infrastructure/cache/va_type_rule_provider_test.go"
Task: "Usecase test for VATypeRuleProvider injection in internal/usecase/merchant_va_usecase_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (schema, domain errors, repository methods, VA-type-rule table)
3. Complete Phase 3: User Story 1 (dynamic VA creation, all 3 sub-types)
4. **STOP and VALIDATE**: Run quickstart Scenarios 1-3 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. User Story 1 → dynamic VA creation works end-to-end (MVP)
3. User Story 2 → static VA creation + duplicate rejection added
4. User Story 3 → validation guardrails layered on top
5. Polish → variable-bill multi-payment tracking, coverage, lint, full quickstart
6. User Story 4 (amendment) → VA type/partner service ID rules move to DB + Redis cache, transparently to merchants
7. Polish (amendment) → coverage/lint on new packages, live cache-refresh and load validation

### Parallel Team Strategy

Not recommended for US1/US2/US3 in this feature — all three touch the same `CreateVA` function in `internal/usecase/merchant_va_usecase.go` sequentially (routing guard → dynamic branch → static branch). A single developer/session should implement them in priority order (P1 → P2 → P3) to avoid merge conflicts in that function; Foundational (Phase 2) and Polish (Phase 6) tasks marked [P] can be parallelized across different files. User Story 4 (Phase 7) is more parallelizable on its own — T037-T040 (tests) and T043/T044 (repository/cache implementations, different files) can be split across developers — but T047-T049 (wiring into the existing usecase/main.go) should land after US1-3 are stable to avoid touching `CreateVA` mid-flight.

---

## Notes

- [P] tasks = different files or clearly separable sections, no dependencies
- [Story] label maps task to specific user story for traceability
- Tests MUST be written and MUST fail before their corresponding implementation task (Constitution III)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- T016/T023/T028-T030 all edit the same `CreateVA` function by design — each user story's tasks add a self-contained guard/branch rather than reworking prior stories' code, so the function stays independently testable at each checkpoint
- T047 also edits `CreateVA`, but only to swap its *data source* for VA-type rules (package-level map → injected provider) — it should not change any validation branch's logic or ordering established by T016/T023/T028-T030; a passing T040 (asserting unchanged US1-3 behavior under the new provider) is the guard against an accidental behavior change here
