# Quickstart: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Feature**: `007-merchant-expiry-callback`

## Prerequisites

- Local stack running (PostgreSQL, Redis, Asynq worker, API server) per repo's existing `README`/dev scripts.
- A test merchant client with a registered VA whose `notification_url` points to a reachable test receiver (e.g. `webhook.site` or a local HTTP echo server).
- `ADMIN_API_KEY` set in the environment for the resend endpoint checks.

## Scenario 1: Expiry detected via bill inquiry (User Story 1, FR-001/FR-001a)

1. Create a VA (via existing merchant VA creation flow) with `expiredDate` set a few seconds in the future.
2. Wait until `expiredDate` has passed.
3. Call the SNAP bill inquiry endpoint for that `virtualAccountNo`.
4. **Expect**: HTTP 404, body `responseCode: "4042419"`, `inquiryStatus: "01"`, `inquiryReason.indonesia: "transaksi kadaluarsa"` (see [contracts/inquiry-expired.md](contracts/inquiry-expired.md)).
5. **Expect**: within a few seconds, the test receiver gets a `va.expired` callback with valid `X-Signature`/`X-Timestamp` headers.
6. Repeat step 3 immediately. **Expect**: same 404 expired response again, but the test receiver does NOT get a second callback (dedupe, FR-005).

## Scenario 2: Expiry detected via payment notification (User Story 1, FR-001b)

1. Create a VA with a short `expiredDate`, do not pay it, let it expire.
2. Call the SNAP payment-notification (vendor notify) endpoint for that VA as if a payment arrived.
3. **Expect**: HTTP 404, `responseCode: "4042519"`, `paymentFlagStatus: "01"`, `paymentFlagReason.indonesia: "transaksi kadaluarsa"` (see [contracts/notify-expired.md](contracts/notify-expired.md)).
4. **Expect**: exactly one `va.expired` callback delivered (same dedupe guarantee as Scenario 1 — if Scenario 1 already ran for this VA, no second callback here).

## Scenario 3: Payment beats expiry (race, FR-010)

1. Create a VA with a short `expiredDate`.
2. Concurrently: (a) submit a valid payment notification just before `expiredDate`, and (b) call inquiry just after `expiredDate`.
3. **Expect**: the VA ends up `paid`, not `expired`; inquiry in (b) reflects the paid state, not the expired-response contract; only a `payment.received` callback is delivered, never `va.expired`.

## Scenario 4: No notification URL registered (edge case, FR-009)

1. Create a VA with no `notification_url` (or merchant configured without one) and let it expire.
2. Trigger detection via inquiry (as in Scenario 1).
3. **Expect**: the 404 expired response is still returned and the VA status becomes `"02"` in the database, but no callback is attempted (verify no delivery-attempt row is created, or one is created with a "skipped" reason per implementation choice).

## Scenario 5: Manual resend (User Story 2, FR-011–FR-019)

1. Using a VA from Scenario 1 or 2 (which now has a `va.expired` delivery-attempt row on record), call:
   ```
   POST /admin/transactions/{virtualAccountNo}/resend-callback
   X-Admin-API-Key: <configured key>
   ```
2. **Expect**: HTTP 200, body echoes `eventType: "va.expired"` and `deliveryStatus: "success"`.
3. **Expect**: the test receiver gets a second `va.expired` callback (identical event data, new delivery).
4. **Expect**: querying delivery history for this VA now shows two rows for `va.expired` — one `trigger: "auto"`, one `trigger: "manual"` (FR-018/SC-007).
5. Call the same endpoint again immediately. **Expect**: another successful resend (no dedup/rate-limit — see contracts/resend-callback.md).
6. Call resend for a VA that has never had any callback (e.g., still active/unpaid, never inquired/notified). **Expect**: HTTP 422, "no callback delivery on record."
7. Call resend for a non-existent `virtualAccountNo`. **Expect**: HTTP 404.
8. Call resend without the `X-Admin-API-Key` header. **Expect**: HTTP 401/403 per existing `AdminAuthMiddleware` behavior, and confirm no callback was sent.

## Validation checklist

- [ ] All SNAP response codes/messages match [contracts/inquiry-expired.md](contracts/inquiry-expired.md) and [contracts/notify-expired.md](contracts/notify-expired.md) exactly.
- [ ] `va_transactions.status` transitions `"03"` → `"02"` exactly once per VA, verified across both inquiry and notify trigger paths.
- [ ] Callback signing (HMAC-SHA512, `X-Timestamp`/`X-Signature`) on `va.expired` events matches the existing `payment.received` verification steps merchants already use.
- [ ] Resend endpoint is unreachable without a valid admin key, and reachable with one.
- [ ] `go test -race ./...` passes; coverage on touched packages (`internal/usecase`, `internal/adapter/delivery/http/handler`, `internal/adapter/delivery/worker`, `internal/infrastructure/database`) remains ≥ 90% per constitution Principle XI.
