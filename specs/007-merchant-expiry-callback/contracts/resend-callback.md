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
