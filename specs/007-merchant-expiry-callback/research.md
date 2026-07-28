# Research: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Feature**: `007-merchant-expiry-callback` | **Date**: 2026-07-28

All unknowns from the Technical Context were resolved by reading the existing codebase rather than external research, since this feature extends established patterns (VA domain, SNAP handlers, Asynq notification worker, admin auth). No NEEDS CLARIFICATION items remain.

## Decision 1: Expiry detection mechanism

**Decision**: Evaluate expiry inline, on-access, inside `VAUsecase.Inquiry` and `VAUsecase.Payment` — no periodic/background scanner.

**Rationale**: Confirmed via clarification with the user (spec.md Clarifications, 2026-07-28) and via code research: no scheduler/cron/ticker exists anywhere in the repo. Adding one would be a new operational component (deployment, monitoring, idempotent-run guarantees) that the user explicitly ruled out in favor of the simpler on-access check, which reuses the request path that already loads the VA row.

**Alternatives considered**:
- Periodic background job (original spec assumption) — rejected by user; adds infra (ticker/cron, singleton-run guarantee across replicas) not currently present in the codebase, and violates YAGNI (constitution Principle I) given the simpler alternative satisfies all acceptance scenarios.
- Redis-based expiry-triggered event (e.g., keyspace notification) — rejected: overengineered for the requirement; nothing in the spec needs sub-request-cycle latency, and it would introduce a new failure mode (missed keyspace events) requiring monitoring.

## Decision 2: Expiry status write path

**Decision**: Reuse the existing `domain.MerchantVARepository.UpdateVAStatus(ctx, virtualAccountNo, status)` (`internal/domain/va.go:205`, impl `internal/infrastructure/database/va_repository.go:508`) to set status `"02"`.

**Rationale**: The existing SQL is `UPDATE va_transactions SET status = $2, updated_at = NOW() WHERE virtual_account_no = $1 AND status = '03'` — scoped to rows currently pending (`"03"`), which is exactly the precondition for "unpaid VA transitioning to expired." No repository change is required; the same guard that prevents corrupting reused VA numbers protects this new call for free.

**Alternatives considered**:
- New dedicated `MarkVAExpired` repository method — rejected: would duplicate the same guarded SQL for no behavioral difference (YAGNI/DRY, constitution Principle I).

## Decision 3: Callback delivery mechanism for the expiry event

**Decision**: Extend the existing `NotificationEnqueuer` / Asynq worker pattern with a distinct event rather than building new delivery infrastructure. Concretely: add an `EventType` discriminator to the notification payload path (or a sibling `EnqueueExpiredNotification` method + task type) so the existing `payment_notification_worker.go` HMAC-SHA512 signing, retry, and delivery code is reused unchanged for the `va.expired` event, following the same shape as `payment.received`.

**Rationale**: Constitution Principle IX mandates non-blocking async delivery via Asynq with retry policies for notifications — already implemented for `payment.received`. Constitution Principle I (DRY/YAGNI) argues against a parallel delivery mechanism. The worker's signing/retry/HTTP-delivery code is event-type agnostic; only the payload's `eventType` field and enqueue call site differ.

**Alternatives considered**:
- Synchronous callback delivery inline in the inquiry/notify request path — rejected: violates FR-010a (must not add unacceptable latency to the inquiry/notify response) and Principle IX (async processing mandatory for notifications).
- A wholly new queue/task type unrelated to `TaskPaymentNotify` — evaluated as acceptable if payload shapes diverge significantly (expiry payload has no paid-amount fields); final call deferred to Phase 1 data-model design of the payload schema, but delivery mechanics (client, worker registration pattern, signing) are unchanged either way.

## Decision 4: Resend endpoint placement, auth, and semantics

**Decision**: Add `POST /admin/transactions/:trxId/resend-callback` inside the existing `adminGroup` (`cmd/api/main.go`), protected by the existing `AdminAuthMiddleware` (`internal/adapter/delivery/http/middleware/admin_auth.go`, `X-Admin-API-Key` header, constant-time compare, fails closed).

**Rationale**: The repo already has exactly one internal/admin authentication pattern, applied to `/admin/clients*` routes. Reusing it satisfies FR-013 (reject non-operator callers) with zero new auth code, consistent with Principle I (avoid speculative abstractions) and Principle VII (no new secret-handling surface — reuses the existing `ADMIN_API_KEY`-sourced middleware).

**Alternatives considered**:
- New JWT-based operator auth — rejected: no such mechanism exists yet in the repo; introducing one is out of scope and unjustified when a working admin API-key pattern already exists.
- Exposing resend on the merchant-facing SNAP API surface — rejected by spec Assumptions (internal/admin-only capability).

## Decision 5: What "resend" redelivers

**Decision**: Resend redelivers the transaction's *current* status/event (payment-received or expired), computed fresh from the VA's current DB state at resend time — not a byte-for-byte replay of a specific historical delivery attempt.

**Rationale**: Matches spec Assumptions ("redelivering the most recent relevant event... rather than replaying an arbitrary historical delivery by ID"). This avoids needing to persist/version full historical payloads (no existing delivery-attempt-payload storage exists in the schema today), keeping scope minimal per YAGNI.

**Alternatives considered**:
- Per-attempt replay by delivery-attempt ID — explicitly deferred in the spec as a possible follow-up; not implemented now since no attempt-history payload store exists.

## Decision 6: Delivery-attempt audit trail (FR-018)

**Decision**: Persist notification delivery attempts (including manual resends) in a new `va_notification_deliveries` table (or equivalent), recording `virtual_account_no`, `event_type`, `trigger` (`auto` | `manual`), `status` (`success`/`failed`), and `attempted_at`. This is additive — it does not modify `va_transactions`.

**Rationale**: FR-008/FR-018 require recording each resend attempt distinctly, and FR-006/SC-007 require the audit record to be retrievable afterward. No existing table stores delivery attempts today (only the outbound HTTP call happens in the worker); a minimal audit table is the smallest schema addition that satisfies these requirements without repurposing `va_transactions` (Constitution Principle IX: PostgreSQL is the source of truth for transactional data; this is exactly that kind of record).

**Alternatives considered**:
- Log-only audit trail (OTel logs, no DB row) — rejected: FR-018/SC-007 require the attempt to be "retrievable afterward as a distinct record," which implies queryable persistence, not just log lines (logs remain useful for tracing per Principle VIII but are not a substitute for the durable record here).
