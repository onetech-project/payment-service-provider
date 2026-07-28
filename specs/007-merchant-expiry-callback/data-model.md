# Data Model: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Feature**: `007-merchant-expiry-callback` | **Date**: 2026-07-28

## Entities

### Virtual Account / Transaction (existing — extended usage, no schema change)

Table: `va_transactions`. No new columns required.

| Field | Type | Notes |
|---|---|---|
| `virtual_account_no` | text | existing PK/identifier |
| `status` | text | existing status code column. `"02"` (expired) is an already-recognized value in code comments but never written today — this feature is the first writer. |
| `expired_date` | `TIMESTAMPTZ` | existing column (`db/migrations/000004_add_va_fields.up.sql`), nullable, already supports time-of-day precision. |
| `notification_url` | text (via `additional_info`/dedicated column per existing schema) | existing merchant callback destination, reused unchanged for expiry events. |

**State transition added by this feature**:

```
"03" (pending) --[current_time > expired_date, detected on inquiry or notify]--> "02" (expired)
```

- Precondition: current status MUST be `"03"`. Enforced by reusing `UpdateVAStatus`'s existing `WHERE ... AND status = '03'` guard — no other status (paid, deleted, already-expired) is overwritten.
- Trigger points: `VAUsecase.Inquiry` and `VAUsecase.Payment`, evaluated inline (no background job — see research.md Decision 1).
- Idempotent: if status is already `"02"` when checked again, the transition is a no-op (row doesn't match `status = '03'` guard) and no duplicate callback is enqueued (see Notification Delivery Attempt entity below for the dedupe mechanism).

### Notification / Callback Event (existing concept, extended payload)

`PaymentNotificationPayload` (`internal/domain/va.go:478`) is extended with an event-type discriminator so the same enqueue/worker/signing path serves both `payment.received` (existing) and the new `va.expired` event.

| Field | Type | Notes |
|---|---|---|
| `EventType` | string | **new**. `"payment.received"` (existing behavior, now explicit) or `"va.expired"` (new). |
| `PartnerServiceID`, `CustomerNo`, `VirtualAccountNo`, `TrxID`, `PaymentRequestID`, `ReferenceNo`, `NotificationURL` | existing | reused as-is for identifying the VA in the expiry event. |
| `ExpiredAt` | string (ISO 8601 timestamp) | **new**, populated only for `va.expired` events; carries the expiry timestamp (FR-003). |
| `PaidAmount`, `CumulativePaymentAmount`, etc. | existing | left empty/omitted (`omitempty`) for `va.expired` events — not applicable. |

Delivery mechanics (HMAC-SHA512 signing, `X-Timestamp`/`X-Signature` headers, Asynq retry policy) are unchanged and reused verbatim from `payment_notification_worker.go`.

### Notification Delivery Attempt (new entity — new table)

New table `va_notification_deliveries`, purely additive (does not alter `va_transactions`). Provides the audit trail required by FR-006/FR-018/SC-007, and the dedupe check required by FR-005 (no duplicate expiry callbacks).

| Column | Type | Notes |
|---|---|---|
| `id` | UUID / bigserial (PK) | |
| `virtual_account_no` | text | FK-by-value to `va_transactions.virtual_account_no` |
| `event_type` | text | `"payment.received"` \| `"va.expired"` |
| `trigger` | text | `"auto"` (from Inquiry/Payment flow) \| `"manual"` (from resend endpoint) |
| `status` | text | `"success"` \| `"failed"` — outcome of this specific delivery attempt |
| `attempted_at` | `TIMESTAMPTZ` | when this attempt was made |
| `error_detail` | text, nullable | populated when `status = "failed"` |

**Uniqueness/dedupe rule (FR-005)**: before enqueuing a `va.expired` auto-triggered notification, the system checks whether a row with `(virtual_account_no, event_type = 'va.expired', trigger = 'auto')` already exists. If so, skip enqueueing again. This check, combined with the `status = '03'` guard on `UpdateVAStatus`, gives two independent layers preventing duplicate expiry callbacks (belt-and-suspenders, since the status guard alone already prevents re-entry after the first successful transition).

**Resend query (FR-011/FR-015)**: the resend endpoint looks up the most recent delivery-attempt row for the given `virtual_account_no` (any trigger) to determine the current/most-recent event type to redeliver; if none exists, returns the "no callback to resend" error (FR-015).

## Validation Rules Summary

- A VA is expired iff `current_time > expired_date` AND `status == "03"` (unpaid/pending). (Clarified 2026-07-28 — corrects the initially-reversed comparison direction.)
- An expiry callback is sent at most once per VA (enforced via `UpdateVAStatus` guard + delivery-attempt dedupe check).
- A resend request requires: (a) the VA exists, (b) at least one prior delivery-attempt row exists for it, (c) the VA has a non-empty `notification_url`. Any failing precondition returns the corresponding FR-014/FR-015/FR-016 error and does **not** create a delivery-attempt row.
- Resending never mutates `va_transactions.status` (FR-019) — it only reads current state and inserts a new `va_notification_deliveries` row.
