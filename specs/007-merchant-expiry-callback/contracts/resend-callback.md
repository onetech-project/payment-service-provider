# Contract: Admin Resend Callback Endpoint

**Route**: `POST /admin/transactions/:virtualAccountNo/resend-callback`

**Auth**: `AdminAuthMiddleware` (existing, `X-Admin-API-Key` header, constant-time compare, fails closed if unconfigured). Registered on the existing `adminGroup` in `cmd/api/main.go`, alongside `/admin/clients*`.

## Request

No body required. Path parameter `virtualAccountNo` identifies the transaction.

## Responses

### 200 OK — resend succeeded

```json
{
  "virtualAccountNo": "...",
  "eventType": "va.expired",
  "resentAt": "2026-07-28T10:15:00Z",
  "deliveryStatus": "success"
}
```

`eventType` reflects the most recent event on record (`payment.received` or `va.expired`) — not necessarily expiry-related; this endpoint is generic across event types (FR-017).

### 404 Not Found — VA does not exist (FR-014)

```json
{ "error": "transaction not found" }
```

### 422 Unprocessable Entity — no prior callback to resend (FR-015)

```json
{ "error": "no callback delivery on record for this transaction" }
```

### 422 Unprocessable Entity — no notification URL registered (FR-016)

```json
{ "error": "transaction has no registered notification URL" }
```

### 401 Unauthorized — missing/invalid admin key (FR-013)

Standard `AdminAuthMiddleware` rejection (no response body change needed — reuse existing behavior).

## Behavior

1. Look up the VA by `virtualAccountNo`. 404 if absent.
2. Query `va_notification_deliveries` for the most recent row for this VA (any trigger). If none, 422 (FR-015).
3. Confirm the VA has a non-empty `notification_url`. If empty, 422 (FR-016).
4. Rebuild the notification payload from the VA's **current** DB state (not a stored historical payload — see research.md Decision 5) using the same `event_type` as the most recent delivery record.
5. Enqueue via the existing `NotificationEnqueuer` (same signing/delivery path as auto-triggered notifications).
6. Insert a new `va_notification_deliveries` row with `trigger = "manual"` recording the outcome (FR-018).
7. Do not modify `va_transactions.status` (FR-019).

Each call to this endpoint always performs a fresh delivery attempt — no deduplication/rate-limiting of rapid repeated calls (spec Edge Cases).

`deliveryStatus` is `queued` once the callback is handed to the queue, or `failed` if enqueueing failed. Whether the merchant was actually reached is recorded afterwards on the delivery row (see "Delivery outcome" below).

---

# Contract: Admin Bulk Resend Endpoint

**Route**: `POST /admin/callbacks/resend`

**Auth**: same `AdminAuthMiddleware` as above.

## Request

```json
{
  "startTime": "2026-10-27T00:00:00+07:00",
  "endTime": "2026-10-28T00:00:00+07:00",
  "virtualAccountNo": "7000108212221111",
  "status": ["failed"]
}
```

- `startTime` (inclusive) / `endTime` (exclusive): RFC 3339 with offset, required. Range ≤ 31 days. Matched against the original delivery's `attempted_at`.
- `virtualAccountNo`: optional, narrows to one VA. Leading spaces are ignored on both sides, so `"15975123123121"` and `"   15975123123121"` (BCA's partnerServiceId-padded form) match the same rows.
- `status`: optional, filters on the original delivery's current status (`queued`, `retrying`, `success`, `failed`, `duplicate`). Omitted = every status — both failed and successful callbacks are resent.

## Behavior

1. Select original callbacks (`trigger = "auto"`) in range, oldest first. Manual rows are never selected — they are themselves resends.
2. More than 500 matches → 422, nothing sent.
3. Each match is resent with the **payload it was originally enqueued with** (not rebuilt from current VA state — a reused VA's current cycle may not be the one the callback was about), as a new `trigger = "manual"` row.
4. Matches recorded before payloads were stored (migration 000018) are **rebuilt** from the transaction state written alongside that callback, and marked `"reconstructed": true`:
   - `payment.received`: the `va_payments` instalment (variable bill) or settled `va_transactions` row (single settlement, no-bill, reconcile) whose write time is within 5 minutes of the delivery's `attempted_at`, closest first. Matches the VA number with or without BCA's leading-space padding. Not persisted anywhere, so left empty on an instalment: `paidBills`, `paymentType`, `flagAdvise`; its `trxDateTime` is the instalment's `paid_at`.
   - `va.expired`: the `va_transactions` row moved to `02`, or the `va_accounts` registration moved to `EXPIRED`, within the same window.
   - Because the match is by time, a reused VA's newer cycle is never picked up. If that row has since been updated again (moving `updated_at` out of the window), nothing matches and the item is `skipped` with `error` explaining so — never guessed.
5. Matches without a notification URL are reported `skipped`.

## Response 200

```json
{
  "matched": 3, "queued": 1, "failed": 1, "skipped": 1,
  "resentAt": "2026-10-27T03:15:00Z",
  "items": [
    { "sourceDeliveryId": "...", "deliveryId": "...", "virtualAccountNo": "...", "eventType": "payment.received", "status": "queued" },
    { "sourceDeliveryId": "...", "deliveryId": "...", "virtualAccountNo": "...", "eventType": "va.expired", "status": "queued", "reconstructed": true },
    { "sourceDeliveryId": "...", "virtualAccountNo": "...", "eventType": "payment.received", "status": "skipped", "error": "original payload not recorded and no matching transaction found to rebuild it" },
    { "sourceDeliveryId": "...", "deliveryId": "...", "virtualAccountNo": "...", "eventType": "payment.received", "status": "failed", "error": "..." }
  ]
}
```

400 on malformed body, bad/missing times, reversed or > 31-day range, unknown status. 422 when > 500 callbacks match.

---

# Delivery outcome (va_notification_deliveries, migration 000018)

Every callback — auto, manual and reconcile-recovered — is recorded **before** enqueue as `queued`, with its full `payload`. The notification worker finds the row through the Asynq task header `delivery-id` and updates it after every attempt:

| status | meaning |
|---|---|
| `queued` | enqueued, not yet attempted |
| `retrying` | an attempt failed (timeout, merchant ≥ 400, connection error); Asynq retries remain |
| `success` | merchant answered < 400. Never overwritten by a later attempt |
| `failed` | enqueue failed, or the last retry failed |
| `duplicate` | auto-triggered enqueue rejected by Asynq's 5-minute uniqueness window as identical to one already queued |

`attempt_count`, `http_status` (NULL when the merchant never answered, e.g. timeout), `error_detail` and `last_attempt_at` describe the latest attempt. A timeout shows up as `error_detail` containing `Client.Timeout exceeded` / `context deadline exceeded`. Manual resends skip the uniqueness window. Rows written before migration 000018 keep their old meaning: `success` = enqueued.
