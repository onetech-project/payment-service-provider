# Implementation Plan: No-Bill VA — Transaction Created at Payment, Not at Create-VA

**Branch**: `013-no-bill-payment-transaction` | **Date**: 2026-08-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/013-no-bill-payment-transaction/spec.md`

## Summary

No-bill Virtual Accounts (`01` static, `04` dynamic) currently create a *pending transaction* at `/create-va` time, which makes the VA payable exactly once and forces the merchant to re-register before every payment. The fix separates VA **identity** from VA **transactions**: a new `va_accounts` table holds one row per virtual account number, written once at `/create-va`; a `va_transactions` row is then created per payment. A no-bill VA behaves like an e-wallet top-up address — registered once, payable indefinitely.

Bill-bearing types (`02`, `03`, `05`, `06`) keep create-VA-time transaction creation untouched. They gain a `va_accounts` row for identity, but their payment, inquiry, expiry, and conflict behavior is unchanged.

Full design rationale in [research.md](./research.md); schema in [data-model.md](./data-model.md); wire behavior in [contracts/](./contracts/).

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — module `backbone-new`

**Primary Dependencies**: Echo (HTTP delivery), pgx/v5 (PostgreSQL driver + pool), Redis (distributed locks, master-data cache), Asynq (merchant callback queue), OpenTelemetry Go SDK, `google/uuid`, testify

**Storage**: PostgreSQL — new table `va_accounts`; existing `va_transactions`, `va_bill_details`, `va_payments`, `va_customer_no_sequences`, `master_va_type`, `master_partner_service_ids`, `va_notification_deliveries` unchanged. Migrations via `migrate/migrate` under `db/migrations/`, next sequence number `000014`.

**Testing**: Go `testing` + testify, following this repo's established split:
- `internal/usecase/` — unit tests against the in-package `MockVARepository` / `MockMerchantVARepository` / `MockVATypeRuleProvider` doubles. This is where all branch logic is proven.
- `internal/infrastructure/database/va_repository_test.go` — pure-Go logic only (error mapping, lock behavior, filter/SQL building). Methods requiring a live PostgreSQL connection are **not** unit-tested here, matching the existing convention documented in that file for `NextCustomerNoSequence`, `RegisterStaticCustomerNo`, and `SaveVAPayment`.
- SQL correctness for the new repository methods is validated by [quickstart.md](./quickstart.md) integration scenarios against the Docker Compose stack, same as feature `006`.

**Target Platform**: Linux server, non-root distroless container

**Project Type**: Web service (SNAP/ASPI-compliant payment integration gateway)

**Performance Goals**: No regression on the existing vendor-facing endpoints. The no-bill payment path removes a lock and a `SUM()` relative to the variable-bill path, and adds one indexed point lookup (`va_accounts` by unique `virtual_account_no`) to inquiry and payment.

**Constraints**:
- Vendor-facing request/response schemas MUST NOT change (spec A-008) — this is a change to *when* rows are written and *what blocks a payment*.
- Existing no-bill VAs created under the old flow MUST keep working (FR-022, SC-006).
- Callback exactly-once semantics from feature `007-merchant-expiry-callback` MUST be preserved.
- ≥90% test coverage (Constitution XI); `go test -race`, `golangci-lint run` clean.

**Scale/Scope**: 1 new migration, ~5 new domain types + 6 new repository methods, 3 usecase branches (create/inquiry/payment), 1 delete-VA branch, 1 repository listing split, 1 new merchant route. No new services, no new infrastructure.

## Constitution Check

*GATE: evaluated before Phase 0 and re-evaluated after Phase 1 design. Both passes clean.*

| Principle | Assessment | Verdict |
|---|---|---|
| **I. Clean Architecture** | `VAAccount` and `VAAccountStatus` are plain structs/constants in `internal/domain/va.go` with no driver imports. `SaveVAAccount`/`GetVAAccount`/… are added to the existing consumer-defined `domain.VARepository` interface. Usecases depend only on that interface; SQL stays in `internal/infrastructure/database/`. No new global state. | ✅ PASS |
| **II. Configuration-Driven Integrations** | VA type classification continues to come from `master_va_type` master data via the Redis-cached `VATypeRuleProvider`. The no-bill branch keys off `rule.Billing == VATypeBillingNone`, **not** a hardcoded `vaType == "01" \|\| "04"` check — so an operator adding a seventh no-bill VA type gets the new flow with no code change. | ✅ PASS |
| **III. TDD** | Every task below is ordered test-first. The three defects in the spec's Problem Statement are each reproduced as a failing test before any production code is written (T-006, T-013, T-023). | ✅ PASS |
| **IV. Context Propagation** | All new repository and usecase methods take `ctx context.Context` first, matching every existing signature in `VARepository`. No new goroutine escapes its caller's context. | ✅ PASS |
| **V/VI. Docker & Non-Root** | No Dockerfile change. | ✅ N/A |
| **VII. Zero Secrets** | No new credentials, keys, or config values. | ✅ N/A |
| **VIII. OTel Observability** | New repository methods inherit tracing from the instrumented pgx pool; new usecase branches inherit the Echo/OTel middleware span. Structured log lines for `va_account_registered` and `va_nobill_payment_recorded` follow the existing `event=… key=value` form used by `markExpiredAndNotify`. | ✅ PASS |
| **IX. Async & State Management** | PostgreSQL remains the source of truth. Schema change is a versioned `migrate/migrate` pair under `db/migrations/`. Callbacks continue through the existing Asynq pipeline; no new queue or task type. | ✅ PASS |
| **X. Idempotency** | Ingress `Idempotency-Key` middleware is untouched. Business-level idempotency is *strengthened*: the no-bill payment row is keyed on the `UNIQUE (inquiry_request_id)` index set to `paymentRequestId`, so a duplicate that races past the `GetPayment` short-circuit is rejected by the database rather than silently upserting over a settled row (research.md R-003). Registration upsert is idempotent by construction (FR-005). | ✅ PASS |
| **XI. Coverage >90%** | New code lands in `internal/domain/`, `internal/usecase/`, `internal/infrastructure/database/`, `internal/adapter/delivery/http/handler/` — all in-scope packages, all covered by the test tasks below. | ✅ PASS |

**No violations. Complexity Tracking section is empty by design.**

## Project Structure

### Documentation (this feature)

```text
specs/013-no-bill-payment-transaction/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 — design decisions R-001..R-009
├── data-model.md        # Phase 1 — va_accounts schema, domain types, migration
├── quickstart.md        # Phase 1 — runnable end-to-end validation guide
├── contracts/
│   ├── create-va-no-bill.md   # /create-va registers only, no transaction
│   ├── payment-no-bill.md     # each payment creates a new transaction
│   ├── inquiry-no-bill.md     # inquiry resolves from the registration
│   ├── expiry-no-bill.md      # registration-level expiry + delete-VA
│   └── merchant-list.md       # VA listing vs transaction listing
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 — created by /speckit-tasks, NOT by this command
```

### Source Code (repository root)

```text
db/migrations/
├── 000014_create_va_accounts.up.sql      # NEW — table, constraints, backfill
└── 000014_create_va_accounts.down.sql    # NEW — DROP TABLE only

internal/
├── domain/
│   └── va.go                             # MODIFY — VAAccount, VAAccountStatus,
│                                         #   VAAccountListItem, VAAccountListFilter,
│                                         #   VATransactionListItem, 6 VARepository
│                                         #   methods, 3 new sentinel errors
├── usecase/
│   ├── merchant_va_usecase.go            # MODIFY — CreateVA: register-only branch for
│   │                                     #   no-bill; registration write for all managed
│   │                                     #   types; DeleteVA: registration deactivation
│   └── va_usecase.go                     # MODIFY — Inquiry: registry branch (no SaveInquiry);
│                                         #   Payment: no-bill new-transaction branch;
│                                         #   markRegistrationExpiredAndNotify
├── infrastructure/database/
│   └── va_repository.go                  # MODIFY — SaveVAAccount, GetVAAccount,
│                                         #   GetVAAccountByPartnerAndCustomer,
│                                         #   UpdateVAAccountStatus, ListVAAccounts,
│                                         #   SaveNoBillPayment, ListVATransactions;
│                                         #   RegisterStaticCustomerNo repointed
└── adapter/delivery/http/handler/
    └── merchant_va_handler.go            # MODIFY — ListVA response shape,
                                          #   new ListTransactions handler

cmd/api/main.go                           # MODIFY — register POST /list-transactions

docs/                                     # MODIFY — regenerate Swagger for the
                                          #   listing change
```

**Structure Decision**: The standard Clean Architecture layout already established by the constitution's Directory Layout Convention and used by features `006`–`012`. This feature introduces no new package — every change lands in an existing file in its correct layer. Tests live beside their subjects in the existing `*_test.go` files.

## Implementation Phases

### Phase A — Schema and domain (no behavior change yet)

| # | Task | File |
|---|---|---|
| T-001 | Write `000014_create_va_accounts.up.sql`: table + `UNIQUE (virtual_account_no)` + `UNIQUE (partner_service_id, customer_no)` + status CHECK + partner index. | `db/migrations/` |
| T-002 | Add the backfill INSERT (`DISTINCT ON (virtual_account_no) … ORDER BY virtual_account_no, created_at DESC`, `ON CONFLICT DO NOTHING`) covering `va_type IN ('01'..'06')`, deriving `INACTIVE` where the latest row is `'04'`. | `db/migrations/` |
| T-003 | Write `000014_create_va_accounts.down.sql` (`DROP TABLE IF EXISTS va_accounts`). | `db/migrations/` |
| T-004 | Add `VAAccount`, `VAAccountStatus` constants, `VAAccountListFilter`, `VAAccountListItem`, `VATransactionListItem`, and the three sentinel errors. | `internal/domain/va.go` |
| T-005 | Extend the `domain.VARepository` interface with the six new methods. Compilation now fails for every fake — that failure is the checkpoint. | `internal/domain/va.go` |

Apply with `docker compose up -d migrate`; verify the backfill row count matches `SELECT COUNT(DISTINCT virtual_account_no) FROM va_transactions WHERE va_type IS NOT NULL`.

### Phase B — Repository (TDD)

| # | Task | File |
|---|---|---|
| T-006 | **Failing test first**: error-mapping and SQL-building tests for the new methods (`pgx.ErrNoRows` → `ErrVAAccountNotFound`, unique-violation → `ErrVAPaymentDuplicate`, `ListVAAccounts` filter construction). Live-DB behavior is proven by quickstart Scenario 1/3/10, per this repo's convention. | `va_repository_test.go` |
| T-007 | Implement `SaveVAAccount` (`INSERT … ON CONFLICT (virtual_account_no) DO UPDATE`, `RETURNING id` read back — same pattern and same reason as `SaveInquiry`). | `va_repository.go` |
| T-008 | Implement + test `GetVAAccount` and `GetVAAccountByPartnerAndCustomer`, returning `ErrVAAccountNotFound` on `pgx.ErrNoRows` and the driver error verbatim otherwise. | both |
| T-009 | Implement + test `UpdateVAAccountStatus` with the `WHERE status='ACTIVE'` guard; zero rows affected ⇒ `ErrVAAccountNotFound`. | both |
| T-010 | Implement + test `SaveNoBillPayment` — plain `INSERT` (**not** upsert), mapping a unique-violation on `inquiry_request_id` to `ErrVAPaymentDuplicate`. | both |
| T-011 | Repoint `RegisterStaticCustomerNo` at `va_accounts` (keeping the Redis `withLock` wrapper and the `ErrVACustomerNoAlreadyRegistered` result); test that a static **bill** type still conflicts on a repeat. | both |
| T-012 | Split `ListVA` → `ListVAAccounts` (from `va_accounts`, with `transactionCount`/`totalPaid` aggregates) and `ListVATransactions` (the existing query). Test both. | both |

### Phase C — Create-VA (TDD)

| # | Task | File |
|---|---|---|
| T-013 | **Failing test first**: `/create-va` with `vaType: "01"` writes zero `va_transactions` rows. Reproduces defect 3. | `merchant_va_usecase_test.go` |
| T-014 | In `CreateVA`, after rule resolution, branch on `vaTypeRule.Billing == domain.VATypeBillingNone`: build the `VAAccount` and call `SaveVAAccount`; skip `SaveInquiry`, skip `SaveBillDetails`, skip the pending-transaction conflict check. | `merchant_va_usecase.go` |
| T-015 | For bill-bearing managed types, call `SaveVAAccount` **in addition to** the existing `SaveInquiry`, leaving every existing guard in place. Unmanaged/legacy requests write no registration. | `merchant_va_usecase.go` |
| T-016 | Test FR-005: a repeat `/create-va` on a registered no-bill VA returns `2002700` with updated holder details and still zero transactions — while a repeat on a static **bill** VA still returns `4092701` (the deliberate asymmetry, research.md R-002). | `merchant_va_usecase_test.go` |
| T-017 | Test that `vaType: "04"` still consumes one sequence value per call and produces two distinct registrations across two calls. | `merchant_va_usecase_test.go` |
| T-018 | In `DeleteVA`, branch to `UpdateVAAccountStatus(…, 'INACTIVE')` when a no-bill registration resolves; keep the transaction path for everything else. Test the idempotent repeat and that historical transactions are untouched. | `merchant_va_usecase.go` + test |

### Phase D — Inquiry (TDD)

| # | Task | File |
|---|---|---|
| T-019 | **Failing test first**: inquiry against a registered, never-paid no-bill VA returns `2002400` with the registered holder name and writes zero rows. | `va_usecase_test.go` |
| T-020 | Insert the registry branch in `Inquiry` after the `GetInquiry` short-circuit and before `GetVAByVirtualAccountNo`; build the response from the registration, report `totalAmount` as `0.00`, and return without `SaveInquiry`. | `va_usecase.go` |
| T-021 | Add the expiry/inactive guards on the registration, delegating to `markRegistrationExpiredAndNotify`. Test `4042419` for both, and exactly-once callback on repeated expired inquiries. | both |
| T-022 | Test that a VA with no registration still falls through to the unchanged legacy path (FR-022). | `va_usecase_test.go` |

### Phase E — Payment (TDD)

| # | Task | File |
|---|---|---|
| T-023 | **Failing test first**: two payments with distinct `paymentRequestId` into one registered no-bill VA both succeed and produce two settled transactions. Reproduces defects 1 and 2. | `va_usecase_test.go` |
| T-024 | Insert the no-bill branch in `Payment` after the `GetPayment` short-circuit and before `GetVAByVirtualAccountNo`: resolve the registration, apply the expired/inactive guards, then build a `VAPaymentRecord` with `InquiryRequestID = req.PaymentRequestID`, `Status = "00"`, `TotalAmount = PaidAmount`, and holder fields inherited from the registration; persist via `SaveNoBillPayment`. | `va_usecase.go` |
| T-025 | Bypass the `req.TotalAmount != nil && paidAmount != totalAmount` mismatch check on this branch (no bill exists to match), and add a `paidAmount <= 0` rejection returning `4002501` with no row written. | `va_usecase.go` + test |
| T-026 | Route the callback through the existing `notifyMerchantWithVA`, sourcing `notificationURL` and `trxID` from the registration. Test exactly one callback per settled payment and none when `notification_url` is empty. | both |
| T-027 | Test `ErrVAPaymentDuplicate` handling: a racing duplicate `paymentRequestId` re-reads and replays the original response, creating no second row and no second callback. | `va_usecase_test.go` |
| T-028 | Test rejection with `4042519` when no registration exists, when it is `INACTIVE`, and when it is past `expired_date`. | `va_usecase_test.go` |

### Phase F — Reporting, wiring, docs

| # | Task | File |
|---|---|---|
| T-029 | Repoint `MerchantVAHandler.ListVA` at `ListVAAccounts` and add `ListTransactions`. | `merchant_va_handler.go` + test |
| T-030 | Register `POST /list-transactions` on `merchantGroup` alongside the existing `/list`. | `cmd/api/main.go` |
| T-031 | Update Swagger annotations and regenerate `docs/`. | `docs/` |
| T-032 | Update `README.md` and the no-bill sections of `docs/guides/` to describe register-once/pay-many. | docs |

### Phase G — Verification

| # | Task |
|---|---|
| T-033 | Run the full [quickstart.md](./quickstart.md) scenarios end-to-end against the Docker Compose stack. |
| T-034 | Regression: re-run every existing test for VA types `02`, `03`, `05`, `06` unchanged (SC-005). |
| T-035 | `go test -race ./...`, `go test -coverprofile` ≥90%, `golangci-lint run` clean, Docker build succeeds. |

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Backfill misses a VA number, stranding an in-flight VA | High — a live VA stops accepting payments | Both the inquiry and payment branches fall through to the existing transaction-based path when no registration is found, so a missed backfill degrades to today's behavior rather than to a hard failure. T-022 tests this fall-through explicitly. |
| `ListVA` response shape change breaks an existing merchant dashboard client | Medium | The change is required by FR-023 and is the point of the feature. `/list-transactions` returns the old item shape, so a client needing the old data has a direct migration target. Call out in the release note. |
| Static **bill** types (`02`, `03`) keep the strict duplicate-`customerNo` rejection, which is arguably its own bug | Low | Deliberate — FR-021 requires those types stay put. Recorded in research.md R-002 as follow-up work rather than smuggled into this change. |
| Two `va_transactions` rows for one VA number confuse the `GetVAByVirtualAccountNo` `ORDER BY created_at DESC LIMIT 1` heuristic | Medium | No-bill flows no longer call it — they resolve through the exact `va_accounts` unique-index lookup. It remains only on the legacy and bill-bearing paths, where its existing semantics are correct. |
| Migration rollback | Low | The up migration only creates a table; it alters no existing column or row. `DROP TABLE va_accounts` restores the prior schema exactly, provided application code is rolled back with it. |

## Open Decisions for Confirmation

Both are informed defaults carried from the spec and implemented as described above. Correcting either is cheap now and expensive after Phase C.

1. **A-002** — a `va_accounts` row is written for *all* managed VA types (`01`–`06`), not only no-bill. The alternative (no-bill only) means two lookup paths in inquiry and payment.
2. **A-003** — a repeat `/create-va` on a registered **no-bill** VA updates the holder details and returns success rather than conflicting. Static bill types keep conflicting.

## Complexity Tracking

No constitution violations. Section intentionally empty.
