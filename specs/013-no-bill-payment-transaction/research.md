# Phase 0 Research: No-Bill VA — Transaction Created at Payment

**Feature**: `013-no-bill-payment-transaction`
**Date**: 2026-08-05

This document records the design decisions taken before implementation, each grounded in the current codebase state.

---

## R-001: Where does VA identity live?

**Decision**: Introduce a new table `va_accounts` as the VA registry. It holds one row per `virtual_account_no`. `va_transactions` keeps its current meaning — one row per payment/inquiry event — and gains no new identity responsibility.

**Rationale**:

- Today `va_transactions` is doing two jobs at once: it *is* the VA (identity, holder name, notification URL, expiry) and it *is* the transaction (amount, payment request ID, settlement status). Every defect in the spec's Problem Statement traces back to that conflation.
- `internal/infrastructure/database/va_repository.go:610` (`GetVAByVirtualAccountNo`) already has to work around it with `ORDER BY created_at DESC LIMIT 1`, and its own comment admits multiple rows share a `virtual_account_no`. A registry makes that lookup exact instead of heuristic.
- A separate table lets the registry carry a real `UNIQUE` constraint on `virtual_account_no` and on `(partner_service_id, customer_no)`, which is what `RegisterStaticCustomerNo` (`va_repository.go:783`) currently emulates with a Redis lock plus a `COUNT(*)` — a check-then-act race that a unique index closes properly.

**Alternatives considered**:

- *Add an `is_registration BOOLEAN` flag to `va_transactions`.* Rejected: every existing query (`ListVA`, `GetVAByVirtualAccountNo`, `GetInquiry`, `GetPayment`) would need a filter added, and forgetting one silently reintroduces the bug. Also cannot express the identity uniqueness constraint.
- *Keep identity in `va_transactions` and relax the pending-transaction guard for no-bill only.* Rejected: it fixes symptom 1 (second payment rejected) but leaves symptoms 2 and 3 (re-registration required, phantom transactions) untouched, and leaves reporting unable to answer "how many VAs do I have".

---

## R-002: Which VA types get a registry row?

**Decision**: All managed VA types (`01`–`06`) get a `va_accounts` row on `/create-va`. Only no-bill types (`01`, `04`) change *when* a `va_transactions` row is created. Unmanaged/legacy requests (no `vaType`, non-reserved `partnerServiceId`) are unchanged and get no registry row.

**Rationale**: Spec assumption A-002. Identity is identity regardless of billing mode; splitting the registry by VA type would mean two lookup paths in `Inquiry` and `Payment`. Bill-bearing types simply keep their extra `SaveInquiry` call on top of the registration.

**Consequence for `RegisterStaticCustomerNo`**: The uniqueness check moves onto `va_accounts`, but its *strictness* is deliberately kept type-dependent so FR-021 holds:

| VA type | Repeat `/create-va` on the same registered VA number |
|---|---|
| `01`, `04` (no bill) | Upsert holder details, return `2002700`. No transaction. (FR-005) |
| `02`, `03` (static bill) | Reject `4092701` exactly as today. |
| `05`, `06` (dynamic bill) | Not reachable — each call generates a fresh `customerNo`. |

**Alternatives considered**: Relaxing the static-bill duplicate check at the same time. Rejected — it is a real behavior change for `02`/`03`, it is not required by any FR, and FR-021 explicitly demands those types stay put. Worth doing later as its own change.

---

## R-003: How is a per-payment transaction row kept unique?

**Decision**: For no-bill payments, set `va_transactions.inquiry_request_id = req.PaymentRequestID`, unconditionally — ignoring any `inquiryRequestId` or `trxId` the vendor sends.

**Rationale**:

- `inquiry_request_id` is `UNIQUE NOT NULL` (`db/migrations/000003_create_va_transactions.up.sql`) and is the `ON CONFLICT` key for both `SaveInquiry` and `SavePayment` (`va_repository.go:92`, `va_repository.go:169`). It is already the de-facto row identity.
- `paymentRequestId` is Mandatory and unique per payment in the ASPI spec; `inquiryRequestId` is not a field of `PaymentRequest` at all and `trxId` is only Conditional. The existing fallback chain at `internal/usecase/va_usecase.go:227-233` already lands on `paymentRequestId` as its final, guaranteed-unique option.
- Using it directly means N payments produce N rows automatically, and a retried `paymentRequestId` collides on the existing unique index — so FR-012 idempotency comes from the same mechanism, with the `GetPayment` short-circuit at `va_usecase.go:183` as the fast path in front of it.

**Alternatives considered**: Adding a `UNIQUE` index on `payment_request_id` and a second `ON CONFLICT` target. Rejected as redundant — it would need a new `SavePayment` variant and a partial index (the column is nullable for inquiry-only rows), for no behavior the above does not already give.

---

## R-004: How does inquiry resolve a VA that has no transaction?

**Decision**: In `VAUsecase.Inquiry`, look up `va_accounts` by `virtual_account_no` *before* the `GetVAByVirtualAccountNo` transaction lookup. If a no-bill registration is found and is active, build the response from the registration and return — never calling `SaveInquiry`.

**Rationale**: FR-016 forbids creating a transaction at inquiry time. The current code path at `va_usecase.go:128-151` unconditionally inserts an inquiry-only row when no merchant VA is found, which after this change would be *every* first inquiry on a no-bill VA — reintroducing phantom rows through a different door.

**Amount echoed**: `totalAmount` is echoed from the inquiry request's own `amount` (spec A-005), not asserted from the registration, because a no-bill VA has no bill.

**Ordering note**: The registry lookup must come after the `GetInquiry` idempotency short-circuit (`va_usecase.go:60`) so retried inquiry request IDs keep their existing behavior.

---

## R-005: Expiry and deactivation for a registration

**Decision**: `va_accounts` carries `expired_date` and `status` (`ACTIVE` / `INACTIVE`). Expiry detection for no-bill VAs runs against the registration, and `markExpiredAndNotify` gains a registration-scoped sibling that flips `va_accounts.status` under a `WHERE status = 'ACTIVE'` guard.

**Rationale**: The existing `markExpiredAndNotify` (`va_usecase.go:490`) relies on `UpdateVAStatus`'s `WHERE status = '03'` clause to guarantee exactly-once callback delivery. A no-bill VA has no `'03'` row to guard on, so the same trick is applied one level up on the registration row. The `deliveryRepo.ExistsByVirtualAccountNoAndEventType` dedupe from feature `007-merchant-expiry-callback` is reused unchanged.

**No expiry date = never expires** (spec A-004), matching the current `merchantVA.ExpiredDate != nil` guard.

---

## R-006: Backfill of existing data

**Decision**: Migration `000014` creates `va_accounts` and backfills one row per distinct `virtual_account_no` from `va_transactions` where `va_type` is one of `01`–`06`, taking the holder fields from the most recent row. Existing `va_transactions` rows are left untouched.

**Rationale**: FR-022 / SC-006 — no VA may be stranded. Backfilling identity means an in-flight no-bill VA created under the old flow resolves through the new registry lookup on its next inquiry or payment. Its old pending (`'03'`) row remains in place and remains payable through the existing path.

**Handling of in-flight pending no-bill rows**: Left as-is. The no-bill payment path creates a *new* row keyed by `paymentRequestId` rather than settling the old pending one, so the stale `'03'` row simply ages out via the normal expiry path. Documented in `quickstart.md` as a post-deploy check rather than a data fix.

**Rollback**: `000014_*.down.sql` drops `va_accounts` only. Since no existing column or row is modified, rolling back the migration restores the prior schema exactly; the application code must be rolled back with it.

---

## R-007: Reporting — VAs vs transactions

**Decision**: `ListVA` (`va_repository.go:343`) is split. The merchant dashboard's VA list reads `va_accounts` (one row per VA number). A transaction list reading `va_transactions` is exposed alongside it, filterable by `virtual_account_no`.

**Rationale**: FR-023 / SC-007. Today `ListVA` selects straight from `va_transactions`, so a no-bill VA with 10 payments would render as 10 VAs. Splitting the two is the only way the counts can be right.

---

## R-008: Concurrency

**Decision**:

- Registration: rely on the `UNIQUE` index on `va_accounts(virtual_account_no)` with `INSERT ... ON CONFLICT DO UPDATE`, keeping the existing Redis `withLock` wrapper (`va_repository.go:61`) around the static-customerNo path for the `4092701` conflict determination.
- Payment: no lock. Two concurrent payments carry distinct `paymentRequestId`s and therefore write distinct rows with no shared mutable state — this is precisely what removing the "settle the one pending row" model buys.

**Rationale**: The current variable-bill path needs `SaveVAPayment`'s transaction + `SUM()` because payments mutate a shared cumulative total. No-bill payments have no shared total, so the contention disappears rather than needing to be managed.

---

## R-009: Sequence generation for dynamic no-bill (`04`)

**Decision**: Unchanged. `NextCustomerNoSequence` (`va_repository.go:737`) still runs on every `/create-va` for vaType `04`.

**Rationale**: Each `/create-va` for a dynamic VA type is a request for a *new* VA number, so consuming a sequence value per call is correct. The change is only that the newly numbered VA lands in `va_accounts` instead of `va_transactions`. FR-005's "repeat call updates the registration" is unreachable for `04` since the merchant never supplies the `customerNo`.
