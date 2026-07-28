# Implementation Plan: Static and Dynamic Virtual Account Creation

**Branch**: `006-static-dynamic-va` | **Date**: 2026-07-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-static-dynamic-va/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Extend the existing `/create-va` (ASPI `VAUpsertRequest`) endpoint so that the `partnerServiceId` + `additionalInfo.vaType` pair routes each request into one of six VA types (static/dynamic × no-bill/variable-bill/fixed-bill). Dynamic types (04/05/06) require an empty `customerNo` and get a system-generated, uniquely-sequenced 20-digit `customerNo` (`vaType` + 18-digit sequence) via a locked/queued counter; static types (01/02/03) require a merchant-supplied `customerNo` that is registered under the same lock to reject duplicates. "Fixed bill" and "variable bill" types require `totalAmount`; "variable bill" additionally accepts multiple individually-recorded payments against the same VA until cumulative payments reach `totalAmount`.

## Technical Context

**Language/Version**: Go 1.26.5 (existing `payment-service-provider` / module `backbone-new`)

**Primary Dependencies**: Echo v4 (HTTP delivery), `jackc/pgx/v5` (PostgreSQL driver, hand-written SQL, no ORM), `redis/go-redis/v9` (distributed locks, reused from idempotency middleware pattern), `stretchr/testify` (tests)

**Storage**: PostgreSQL — extends `va_transactions`/`va_bill_details` (`db/migrations/000003`-`000005`); adds a new migration for `va_type`, a per-`vaType` sequence counter, and a `va_payments` table for individually-recorded variable-bill payments

**Testing**: Standard Go `testing` + `testify` (`mock`, `assert`), colocated `_test.go` files per Constitution III/XI (TDD, >90% coverage)

**Target Platform**: Linux server (containerized, existing deployment)

**Project Type**: Web service (single Go backend, Clean Architecture layering per constitution)

**Performance Goals**: Consistent with existing VA endpoints — no new dedicated latency target beyond SC-001..SC-007 in the spec (correctness/uniqueness focused, not throughput-focused)

**Constraints**: Dynamic `customerNo` generation and static `customerNo` registration MUST be race-free under concurrent requests (FR-005, FR-008); `customerNo` MUST NOT exceed 20 digits (FR-005a)

**Scale/Scope**: Single new endpoint-extension (no new endpoint), 6 VA type combinations, additive schema change — no scale-driven unknowns

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Applicability | Assessment |
|---|---|---|
| I. Clean Architecture | Yes | New validation/routing logic stays in `internal/usecase/merchant_va_usecase.go`; new counter/lock access goes through a `domain.VARepository`-style interface implemented in `internal/infrastructure/database`; no framework leakage into `internal/domain`. PASS |
| II. Configuration-Driven Integrations | No | This feature is about internal VA-type routing/numbering, not a new external payment provider; the six `partnerServiceId`/`vaType` combinations are fixed business rules from the spec, not provider config. N/A |
| III. TDD & Testability | Yes | Plan follows write-test-first for usecase validation branches and repository sequence/lock logic; all new I/O (Postgres, Redis lock) goes behind interfaces for mocking. PASS (enforced in tasks.md /speckit-tasks phase) |
| IV. Context-Aware Operations | Yes | New repository methods (sequence increment, static customerNo registration, payment insert) MUST accept `ctx context.Context` first param, consistent with existing `VARepository` methods. PASS |
| V/VI. Docker/Container | No | No container or deployment topology changes. N/A |
| VII. Credential Store | No | No new secrets/credentials introduced. N/A |
| VIII. Observability | Yes | Reuse existing OTel instrumentation already wrapping `CreateVA`/repository calls; no new uninstrumented I/O path. PASS |
| IX. Async/State (Redis/Asynq) | Partial | Uses Redis distributed lock (existing `internal/infrastructure/redis` `AcquireLock`/`ReleaseLock` pattern) for sequence/registration atomicity; this is synchronous request-path locking, not an Asynq background task — no new async workflow needed. PASS |
| X. Idempotency | Yes | Existing `X-EXTERNAL-ID` idempotency middleware already wraps all mutating endpoints including `/create-va`; unaffected by this feature. PASS |
| XI. Coverage > 90% | Yes | New usecase/repository code must ship with tests reaching the same threshold; verified at task/implementation time. PASS (deferred to CI gate) |

No violations requiring the Complexity Tracking table.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   └── va.go                              # add VAType, TotalAmount(fixed/variable), sequence/lock domain errors
├── usecase/
│   ├── merchant_va_usecase.go             # add vaType/partnerServiceId validation + dynamic/static customerNo routing
│   └── merchant_va_usecase_test.go        # new test cases for all 6 combinations + validation failures
├── adapter/delivery/http/handler/
│   ├── merchant_va_handler.go             # minor: surface new domain error codes on /create-va
│   └── merchant_va_handler_test.go
└── infrastructure/
    ├── database/
    │   ├── va_repository.go               # add: NextCustomerNoSequence, RegisterStaticCustomerNo, SaveVAPayment
    │   └── va_repository_test.go          # new: sequence/lock concurrency + duplicate-customerNo tests
    └── redis/
        └── redis.go                        # reuse existing AcquireLock/ReleaseLock (no change expected)

db/migrations/
├── 000006_add_va_type_and_sequences.up.sql    # va_transactions.va_type column; va_customer_no_sequences table
└── 000006_add_va_type_and_sequences.down.sql

db/migrations/
├── 000007_create_va_payments.up.sql           # va_payments table (per-payment records for variable bill)
└── 000007_create_va_payments.down.sql
```

**Structure Decision**: Single existing Go web-service project (Clean Architecture per constitution: `domain` → `usecase` → `adapter/delivery` → `infrastructure`). No new project/module is introduced; this feature extends the current `merchant_va_usecase.go` / `va_repository.go` / `va.go` trio plus two additive migrations, following the same layering as the existing VA feature (002-bca-virtual-account).

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations — see Amendment section below for the additional Constitution Check covering User Story 4.

---

## Amendment (2026-07-28): VA Type / Partner Service ID Master Data + Redis Cache

**Scope added to spec**: User Story 4, FR-015–FR-019, SC-008/SC-009 (see [spec.md](./spec.md)). Everything above this line describes the already-implemented User Stories 1-3 (merchant-facing `/create-va` behavior). This amendment covers moving the previously hardcoded six VA type rules (`internal/domain/va.go`'s `vaTypeRules` map) and reserved `partnerServiceId` set (`reservedVAPartnerServiceIDs` map) into database-backed master data, cached in Redis, so operators can change them without a deployment.

### Technical Context (amendment)

**Language/Version**: Go 1.26.5 (unchanged)

**Primary Dependencies**: adds no new external dependency — reuses `jackc/pgx/v5` (new tables) and `redis/go-redis/v9` (new cache keys), consistent with the rest of the codebase

**Storage**: PostgreSQL — new migration `000008_create_master_va_type_and_partner_service_ids` adding `master_va_type` and `master_partner_service_ids` tables, seeded with the current six rules / three partner IDs so first-boot behavior is unchanged (Acceptance Scenario 1). Redis — new cache keys (`master:va_types`, `master:partner_service_ids`) with a 5-minute TTL, refreshed on schedule and on-demand via the write path.

**Testing**: Standard Go `testing` + `testify`, same conventions as the rest of this feature

**Target Platform**: unchanged

**Project Type**: unchanged

**Performance Goals**: `/create-va`'s VA-type-rule/partner-service-ID lookup MUST NOT issue a PostgreSQL query per request under normal operation (SC-009) — served from the Redis cache (or an in-process last-known-good snapshot if Redis is briefly unavailable, FR-018)

**Constraints**: Cache refresh-on-change MUST NOT expose a torn/partial read to concurrent `/create-va` requests (edge case in spec.md); "immediate refresh on change" only applies to changes made through the application's own data-access layer, not out-of-band DB edits (documented Assumption) — those are picked up on the next scheduled 5-minute refresh

**Scale/Scope**: Two new small, low-cardinality tables (6 rows / 3 rows expected); no scale-driven unknowns

### Constitution Check (amendment)

| Principle | Applicability | Assessment |
|---|---|---|
| I. Clean Architecture | Yes | New `domain.VATypeRuleProvider` interface (consumer-defined, per Constitution I) replaces the current package-level `domain.LookupVATypeRule`/`IsReservedVAPartnerServiceID` functions; its implementation (Postgres read + Redis cache + in-memory fallback) lives entirely in `internal/infrastructure`, injected into `MerchantVAUsecase` via constructor DI — no framework/driver leakage into `internal/domain`. PASS |
| II. Configuration-Driven Integrations | Yes | This is precisely the "zero-code-modification configuration" principle applied to VA type/partner routing rules: they move from hardcoded Go maps to operator-editable database rows. PASS (this amendment is what makes this principle applicable to feature 006, where it was previously N/A) |
| III. TDD & Testability | Yes | The new provider is an interface — trivially mockable in `merchant_va_usecase_test.go` (no behavior change needed in existing tests, since the interface's default/seeded data matches the old hardcoded map exactly). New repository/cache code gets its own tests before implementation. PASS |
| IV. Context-Aware Operations | Yes | All new repository/cache methods (`ListVATypeRules(ctx)`, `ListReservedPartnerServiceIDs(ctx)`, `RefreshNow(ctx)`) accept `ctx context.Context` first, consistent with existing methods. PASS |
| V/VI. Docker/Container | No | No container/deployment changes. N/A |
| VII. Credential Store | No | No new secrets. N/A |
| VIII. Observability | Yes | Cache refresh (scheduled and on-demand) and cache-miss/Redis-unavailable fallback paths get the same OTel trace/log treatment as other Redis/Postgres calls in this codebase. PASS |
| IX. Async/State (Redis/Asynq) | Yes | Redis used as a cache (its documented role per Constitution IX), TTL-based + explicit invalidation — no new Asynq task needed since the refresh-on-change is a synchronous cache write following the same request that mutated the master data. PASS |
| X. Idempotency | No | Master data mutation is out of scope for this feature (see spec.md Assumptions — no CRUD API is being added here); nothing new to gate on Idempotency-Key. N/A |
| XI. Coverage > 90% | Yes | New cache/provider code ships with tests to the same bar as the rest of this feature. PASS (deferred to CI gate) |

No violations requiring the Complexity Tracking table.

### Project Structure (amendment)

```text
internal/
├── domain/
│   └── va.go                                   # replace vaTypeRules/reservedVAPartnerServiceIDs maps
│                                                #   + LookupVATypeRule/IsReservedVAPartnerServiceID funcs
│                                                #   with a VATypeRuleProvider interface (consumer-defined)
├── usecase/
│   └── merchant_va_usecase.go                  # depend on domain.VATypeRuleProvider instead of the
│                                                #   package-level domain functions (constructor DI)
└── infrastructure/
    ├── database/
    │   ├── master_data_repository.go           # new: CRUD + List for master_va_type / master_partner_service_ids
    │   └── master_data_repository_test.go
    └── redis/
        ├── master_data_cache.go                # new: Get/Set cached rule lists, 5-min TTL, RefreshNow()
        └── master_data_cache_test.go

internal/infrastructure/                         # new combined provider (Postgres + Redis + in-memory fallback)
└── cache/
    └── va_type_rule_provider.go                 # implements domain.VATypeRuleProvider; owns the 5-min
                                                  #   ticker and the RefreshNow() call triggered by mutations
    └── va_type_rule_provider_test.go

db/migrations/
├── 000008_create_master_va_type_and_partner_service_ids.up.sql   # both tables + seed data (6 rules, 3 IDs)
└── 000008_create_master_va_type_and_partner_service_ids.down.sql
```

**Structure Decision (amendment)**: Same single-project Clean Architecture layering as the rest of this feature. The provider combining Postgres + Redis + fallback is placed in a new `internal/infrastructure/cache` package (rather than inside `database` or `redis` directly) since it orchestrates both and owns background-refresh lifecycle — keeping `database`/`redis` packages as thin, single-responsibility clients per Constitution I.

### Complexity Tracking (amendment)

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| New `internal/infrastructure/cache` package (a 4th infrastructure subpackage beyond database/redis/queue) | Combining a Postgres-backed repository with a Redis cache and an in-memory fallback, plus owning a background refresh ticker, is a distinct responsibility from either raw client wrapper | Putting this logic inside `database/` or `redis/` directly would make one of those packages depend on the other (violates single-responsibility) or force `merchant_va_usecase.go` to orchestrate Postgres+Redis+ticker itself (leaks infrastructure concerns into the usecase layer, violating Constitution I) |
