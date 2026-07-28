# Contract: Bill Inquiry — Expired VA Response

**Endpoint**: Existing SNAP bill inquiry endpoint (`VAHandler.Inquiry` → `VAUsecase.Inquiry`)

**New behavior**: When the resolved VA is unpaid (`status == "03"`) and `current_time > expired_date`, the usecase MUST return a `DomainError` mapped to the response below instead of the normal inquiry success response. This is a new branch inserted after the existing `GetVAByVirtualAccountNo` lookup, before the success path.

## Response (HTTP status derived from `mapSNAPCodeToHTTP`, first 3 digits `404` → `404 Not Found`)

```json
{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    }
  }
}
```

## Side effects (must occur exactly once per VA, before or alongside the response)

1. `UpdateVAStatus(ctx, virtualAccountNo, "02")` — transitions VA to expired (no-op if already non-`"03"`).
2. If the status transition actually applied (i.e., this call is the first to detect the expiry) AND no prior `va.expired` delivery-attempt row exists: enqueue a `va.expired` notification via `NotificationEnqueuer` (async, non-blocking — must not delay this response, FR-010a).
3. If the VA has no `notification_url`, skip step 2 but still perform step 1 (FR-009).

## Non-goals

- Paid VAs (`status != "03"`) that happen to be past `expired_date` are unaffected by this contract — existing status-conflict handling applies unchanged.
