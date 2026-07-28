# Implementation Plan: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Branch**: `007-merchant-expiry-callback` | **Date**: 2026-07-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/007-merchant-expiry-callback/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Detect Virtual Account expiry inline (on-access) inside the existing SNAP bill-inquiry and payment-notification usecases — a VA is expired when `current_time > expired_date` and its status is still `"03"` (pending). On detection, transition the VA to status `"02"` (reusing the existing guarded `UpdateVAStatus` SQL), return the SNAP-specified expired responses (`4042419` for inquiry, `4042519` for payment notification), and enqueue a `va.expired` merchant callback exactly once via the existing Asynq/`NotificationEnqueuer` HMAC-signed delivery path (extended with an event-type discriminator). A new admin-only endpoint (`POST /admin/transactions/:virtualAccountNo/resend-callback`, protected by the existing `AdminAuthMiddleware`) lets operators redeliver the most recent callback event for a transaction on demand, recording every attempt (auto or manual) in a new audit table.

No new infrastructure (no scheduler, no new queue) is introduced — this is a targeted extension of existing usecase, worker, and admin-route patterns.

## Technical Context

**Language/Version**: Go 1.26.5 (per `go.mod`)

**Primary Dependencies**: Echo v4 (HTTP), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), hibiken/asynq (queue), stretchr/testify (tests) — all existing, no new dependencies required.

**Storage**: PostgreSQL — extends existing `va_transactions` usage (no new columns); adds one new table `va_notification_deliveries` for the delivery-attempt audit trail.

**Testing**: Standard Go `testing` + `testify`, following existing suite conventions in `internal/usecase`, `internal/adapter/delivery/*`, `internal/infrastructure/database`.

**Target Platform**: Linux server (existing containerized deployment, unchanged).

**Project Type**: Web service (single Go backend, Clean Architecture layout per constitution).

**Performance Goals**: Expiry check adds O(1) comparison + at most one guarded UPDATE to the existing inquiry/notify request path; must not add unacceptable latency (FR-010a) — callback enqueue is async (non-blocking), consistent with existing `payment.received` behavior.

**Constraints**: Must not introduce a periodic/background scanning component (explicit user decision, spec Clarifications). Must not duplicate callback for the same VA (FR-005). Must not mutate transaction state on resend (FR-019).

**Scale/Scope**: Same request volume as existing SNAP inquiry/notify endpoints; one new low-traffic admin endpoint (manual, operator-invoked).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment |
|---|---|
| I. Clean Architecture | PASS. New logic lives in `internal/usecase` (expiry check, resend orchestration); new HTTP handler in `internal/adapter/delivery/http/handler`; new repository method(s) in `internal/infrastructure/database`; domain interfaces (`MerchantVARepository`, `NotificationEnqueuer`) extended, not bypassed. No new inheritance; reuses composition via existing constructor-injected interfaces. |
| II. Configuration-Driven Integrations | N/A — this feature doesn't add a new external provider; it extends internal notification logic. No violation. |
| III. TDD | PASS (process requirement) — tasks.md (next phase) must sequence failing tests before implementation for: expiry-detection branch in `Inquiry`/`Payment`, `UpdateVAStatus` reuse, notification payload extension, resend handler, dedupe logic. |
| IV. Context propagation | PASS — new code paths (expiry check, resend usecase, new repo queries) all accept `ctx context.Context` as first param, following existing method signatures. |
| V/VI. Docker/non-root | N/A — no new deployment artifacts; existing multi-stage non-root image covers the new code automatically. |
| VII. Zero secrets in code | PASS — resend endpoint reuses existing `ADMIN_API_KEY`-sourced middleware; no new secret introduced. |
| VIII. OTel Observability | PASS (process requirement) — new usecase/handler/worker code must create spans and structured logs consistent with existing `Inquiry`/`Payment`/`payment_notification_worker` instrumentation; tasks.md must include this. |
| IX. Async processing (PostgreSQL/Redis/Asynq) | PASS — `va_notification_deliveries` is a new PostgreSQL table (source of truth for the audit record); callback delivery stays on Asynq with existing retry policy; no synchronous external calls added to the request path. |
| X. Idempotency | PASS — this feature does not add new state-mutating merchant-facing endpoints requiring `Idempotency-Key` (inquiry/notify already have their own idempotency handling; the new admin resend endpoint is an internal operator action, not a merchant ingress mutation, and explicitly has no dedupe requirement per spec Edge Cases — each call is a deliberate resend). Existing Idempotency-Key enforcement on inquiry/notify is untouched. |
| XI. Coverage ≥ 90% | PASS (process requirement) — tasks.md must include unit/integration tests for all new branches (expired-inquiry, expired-notify, dedupe, resend success/404/422/401) to keep touched packages at/above threshold. |

**Result**: No violations requiring justification. Complexity Tracking section left empty.

## Project Structure

### Documentation (this feature)

```text
specs/007-merchant-expiry-callback/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── inquiry-expired.md
│   ├── notify-expired.md
│   └── resend-callback.md
└── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

### Source Code (repository root)

Existing single-service Clean Architecture layout (no new top-level directories):

```text
internal/
├── domain/
│   └── va.go                          # extend: PaymentNotificationPayload (EventType, ExpiredAt);
│                                       #         new NotificationDelivery entity/interface methods
├── usecase/
│   └── va_usecase.go                  # extend: Inquiry() and Payment() add expiry-check branch;
│                                       #         new ResendCallbackUsecase (or method) for admin resend
├── adapter/
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── handler/
│   │   │   │   └── admin_resend_handler.go   # new: POST /admin/transactions/:id/resend-callback
│   │   │   └── middleware/            # reused as-is: admin_auth.go
│   │   └── worker/
│   │       └── payment_notification_worker.go  # extend: handle EventType discriminator (va.expired)
└── infrastructure/
    └── database/
        └── va_repository.go           # extend: query/insert against new va_notification_deliveries table
                                        #         (UpdateVAStatus reused unmodified)

db/migrations/
└── 000009_create_va_notification_deliveries.up.sql / .down.sql   # new table

cmd/api/main.go                        # extend: wire new resend route into existing adminGroup
```

**Structure Decision**: No new project/module boundary — this feature is entirely additive within the existing single Go service, following the established `domain → usecase → adapter/infrastructure` dependency direction. All new code sits in existing packages (`internal/usecase`, `internal/adapter/delivery/http/handler`, `internal/adapter/delivery/worker`, `internal/infrastructure/database`), consistent with constitution Principle I and the Directory Layout Convention.

## Complexity Tracking

*No Constitution Check violations — table intentionally left empty.*
