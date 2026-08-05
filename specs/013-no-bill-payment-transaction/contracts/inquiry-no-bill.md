# Contract: Bill Inquiry — No-Bill VA Resolves From the Registration

**Endpoint**: Existing SNAP bill inquiry endpoint (`VAHandler.Inquiry` → `VAUsecase.Inquiry`)

**Change**: A new branch in `VAUsecase.Inquiry`, inserted **after** the existing `GetInquiry` idempotency short-circuit and **before** the `GetVAByVirtualAccountNo` lookup. When `GetVAAccount(virtualAccountNo)` resolves an ACTIVE no-bill registration, the response is built from the registration and `SaveInquiry` is **not** called.

This branch is what makes a first-ever payment possible: after this feature, a freshly registered no-bill VA has no transaction row for the old lookup to find.

## Request (unchanged)

```json
{
  "partnerServiceId": "   15973",
  "customerNo": "000000000000000001",
  "virtualAccountNo": "   15973000000000000000001",
  "inquiryRequestId": "INQ-20260805-0001",
  "amount": { "value": "50000.00", "currency": "IDR" }
}
```

## Response

```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "inquiryStatus": "00",
    "inquiryReason": { "english": "Success", "indonesia": "Sukses" },
    "partnerServiceId": "   15973",
    "customerNo": "000000000000000001",
    "virtualAccountNo": "   15973000000000000000001",
    "virtualAccountName": "John Doe",
    "inquiryRequestId": "INQ-20260805-0001",
    "totalAmount": { "value": "50000.00", "currency": "IDR" },
    "subCompany": "00000"
  }
}
```

- `virtualAccountName` comes from `va_accounts.customer_name` — the registered holder (FR-016).
- `totalAmount` echoes the **request's own `amount`** (spec A-005). A no-bill VA asserts no bill; the customer chose the amount at the channel. When the request carries no `amount`, `"0.00"` is echoed.
- `billDetails` is absent — a no-bill VA has none.

## Side effects

**None.** Specifically:

1. **Zero** `va_transactions` rows written — the `SaveInquiry` call at `va_usecase.go:149` is not reached on this branch. (FR-016, SC-002)
2. **Zero** `va_accounts` mutations.
3. No callback enqueued.

## Behavior matrix

| Scenario | Result |
|---|---|
| Registered, never paid | `2002400` with registered holder name. (US3 AS1) |
| Registered, has N settled payments | `2002400` — prior settled payments never block an inquiry. (US3 AS2, FR-015) |
| Same `inquiryRequestId` retried | Existing `GetInquiry` short-circuit applies; since this branch wrote no row, the retry re-enters this branch and returns the same response. No rows created either time. (US3 AS5) |
| Registration `INACTIVE` | `4042419` Invalid Bill/Virtual Account. (US3 AS3) |
| Registration past `expired_date` | `4042419` with expiry side effects — see `expiry-no-bill.md`. (US3 AS4, FR-017) |
| No registration at all | Falls through to the existing legacy path (`GetVAByVirtualAccountNo`, then ad-hoc inquiry record), unchanged — this is what keeps pre-feature VAs working. (FR-022) |

## Non-goals

- Bill-bearing registrations (`02`, `03`, `05`, `06`) are **not** served by this branch; they continue through `GetVAByVirtualAccountNo` so their bill amount, bill details, and expiry behavior stay exactly as today. (FR-021)
- The ad-hoc "inquiry with no prior record" path at `va_usecase.go:128-168` is untouched.
