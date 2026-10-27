# Contract: Admin Resend Callback Endpoint

**Route**: `POST /admin/transactions/:virtualAccountNo/resend-callback`

**Auth**: `AdminAuthMiddleware` (existing, `X-Admin-API-Key` header, constant-time compare, fails closed if unconfigured). Registered on the existing `adminGroup` in `cmd/api/main.go`, alongside `/admin/clients*`.

## Request

Path parameter `virtualAccountNo` identifies the VA. The body is optional:

```json
{ "trxId": "TRX-20261027-001" }
```

`trx_id` is accepted as an alias for `trxId`. A `virtualAccountNo` is reusable across transaction cycles, so without `trxId` the resend targets the VA's current cycle (the one `GetVAByVirtualAccountNo` ranks first). With `trxId` it targets the cycle created with that trxId — e.g. a settled cycle whose callback the merchant missed after a new bill was opened on the same VA.

### 400 Bad Request — malformed JSON body

```json
{ "error": "invalid request payload" }
```

## Responses

### 200 OK — resend succeeded

```json
{
  "virtualAccountNo": "...",
  "trxId": "...",
  "eventType": "va.expired",
  "resentAt": "2026-07-28T10:15:00Z",
  "deliveryStatus": "success"
}
```

`eventType` reflects the most recent event on record (`payment.received` or `va.expired`) — not necessarily expiry-related; this endpoint is generic across event types (FR-017).

### 404 Not Found — VA does not exist, or has no cycle with the given `trxId` (FR-014)

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

1. Look up the VA by `virtualAccountNo` (and `trxId`, when given). 404 if absent.
2. Query `va_notification_deliveries` for the most recent row for this VA (any trigger). If none, 422 (FR-015).
3. Confirm the VA has a non-empty `notification_url`. If empty, 422 (FR-016).
4. Rebuild the notification payload from the VA's **current** DB state (not a stored historical payload — see research.md Decision 5) using the same `event_type` as the most recent delivery record. When `trxId` is given, delivery records carry no trxId and may belong to another cycle, so a cycle whose status is `00` is resent as `payment.received` and one whose status is `02` as `va.expired`; other statuses keep the most recent record's event type.
5. Enqueue via the existing `NotificationEnqueuer` (same signing/delivery path as auto-triggered notifications).
6. Insert a new `va_notification_deliveries` row with `trigger = "manual"` recording the outcome (FR-018).
7. Do not modify `va_transactions.status` (FR-019).

Each call to this endpoint always performs a fresh delivery attempt — no deduplication/rate-limiting of rapid repeated calls (spec Edge Cases).
