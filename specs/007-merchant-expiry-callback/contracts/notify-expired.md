# Contract: Payment Notification (Vendor Notify) — Expired VA Response

**Endpoint**: Existing SNAP payment-notification endpoint (`VAHandler.Payment` → `VAUsecase.Payment`)

**New behavior**: In `VAUsecase.Payment` (`internal/usecase/va_usecase.go`), the existing guard at the "non-pending VA" check (`if merchantVA.Status != "03" { return 409 ... }`) MUST special-case `merchantVA.Status == "02"` OR (`merchantVA.Status == "03"` AND `current_time > expired_date`) to return the expired-specific response below instead of falling through to the generic `4092500` conflict response.

## Response

```json
{
  "responseCode": "4042519",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    }
  }
}
```

## Side effects (same as inquiry-expired.md)

1. `UpdateVAStatus(ctx, virtualAccountNo, "02")` — no-op if already transitioned (e.g., a prior Inquiry call already detected it).
2. Enqueue `va.expired` notification only if not already delivered (dedupe check against `va_notification_deliveries`).
3. Skip notification (but still transition status) if no `notification_url` registered.
4. MUST NOT block/delay this response (FR-010a) — enqueue is async.

## Race with concurrent payment (FR-010, edge case)

If a payment for the same VA is concurrently accepted (state already transitioned to paid before this check runs), the expired-response path MUST NOT fire — the existing paid-VA handling takes precedence. This is naturally satisfied because `UpdateVAStatus`'s `status = '03'` guard means a VA that has already moved to a paid status is no longer eligible for the `"03"` → `"02"` transition.
