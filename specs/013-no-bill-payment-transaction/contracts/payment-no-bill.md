# Contract: Payment Notification — No-Bill VA Creates a New Transaction

**Endpoint**: Existing SNAP payment-notification endpoint (`VAHandler.Payment` → `VAUsecase.Payment`)

**Change**: A new branch in `VAUsecase.Payment`, inserted after the `GetPayment` idempotency short-circuit and before the existing `GetVAByVirtualAccountNo` transaction lookup. When `GetVAAccount(virtualAccountNo)` resolves an ACTIVE registration whose VA type is no-bill (`01`/`04`), the payment creates a brand-new settled transaction instead of settling a pending one.

## Request (unchanged)

```json
{
  "partnerServiceId": "   15973",
  "customerNo": "000000000000000001",
  "virtualAccountNo": "   15973000000000000000001",
  "virtualAccountName": "John Doe",
  "paymentRequestId": "PAY-20260805-0001",
  "paidAmount": { "value": "50000.00", "currency": "IDR" },
  "trxDateTime": "2026-08-05T10:15:00+07:00",
  "referenceNo": "REF00000001"
}
```

## Response

```json
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "   15973",
    "customerNo": "000000000000000001",
    "virtualAccountNo": "   15973000000000000000001",
    "virtualAccountName": "John Doe",
    "paymentRequestId": "PAY-20260805-0001",
    "paidAmount": { "value": "50000.00", "currency": "IDR" },
    "totalAmount": { "value": "50000.00", "currency": "IDR" },
    "trxDateTime": "2026-08-05T10:15:00+07:00",
    "referenceNo": "REF00000001",
    "paymentFlagStatus": "00",
    "paymentFlagReason": { "english": "Success", "indonesia": "Sukses" }
  }
}
```

`paymentFlagStatus` is always `"00"` (settled) — a no-bill payment has no cumulative target, so the `"03"` pending status used by the variable-bill path never applies here.

## Side effects

1. **One new** `va_transactions` row per payment:
   - `inquiry_request_id = paymentRequestId` (unconditionally — the vendor's `inquiryRequestId`/`trxId` are ignored for this key; see `research.md` R-003)
   - `payment_request_id = paymentRequestId`
   - `status = '00'`, `paid_amount = total_amount = paidAmount.value`
   - `va_type` copied from the registration
   - `customer_name`, `customer_email`, `customer_phone`, `notification_url`, `trx_id` inherited from the registration when the request does not supply them (FR-013)
2. **Zero** modifications to any prior transaction row for this VA number. (FR-020)
3. **Zero** modifications to the `va_accounts` registration — a payment never deactivates or settles the VA.
4. Exactly one `va.payment.received` callback enqueued when the registration has a `notification_url`, plus its `va_notification_deliveries` audit row. (FR-014)

## Repeat / concurrency behavior

| Scenario | Result |
|---|---|
| Same `paymentRequestId` retried | `GetPayment` short-circuit returns the original response; zero new rows. (FR-012) |
| Same `paymentRequestId` retried concurrently, both past the short-circuit | The `UNIQUE (inquiry_request_id)` index rejects the second insert with `ErrVAPaymentDuplicate`; the usecase re-reads and replays the original response. Zero duplicate rows, zero duplicate callbacks. |
| Different `paymentRequestId`, same VA number, sequential | Two independent settled rows. (FR-010, US2 AS2) |
| Different `paymentRequestId`, same VA number, concurrent | Two independent settled rows; no lock needed — no shared mutable state. (R-008) |
| 10 payments into one registration | 10 rows, 10 callbacks, 1 registration. (SC-001, SC-007) |

## Error responses

| Condition | Code | Notes |
|---|---|---|
| No `va_accounts` row for this `virtualAccountNo` | `4042519` Invalid Bill/Virtual Account | FR-011. Falls through to the existing legacy path first, so pre-feature VAs still work. |
| Registration `status = 'INACTIVE'` | `4042519` Invalid Bill/Virtual Account | FR-019 / US6 AS2 |
| Registration past `expired_date` | `4042519` Invalid Bill/Virtual Account, `paymentFlagStatus: "01"`, reason `expired transaction` | Side effects per `expiry-no-bill.md` |
| `paidAmount` missing | `4002502` Invalid Mandatory Field [paidAmount] | existing |
| `paidAmount.value` ≤ 0 | `4002501` Invalid Field Format [paidAmount] | new guard; no transaction created |
| Persist failure | `5002500` Internal Server Error | |

`totalAmount` sent by the vendor is **not** compared against `paidAmount` for a no-bill VA — the existing mismatch check at `va_usecase.go:323` is bypassed on this branch, because no bill amount exists to match against.

## Non-goals

- Variable-bill (`02`, `05`) keeps its `SaveVAPayment` cumulative path unchanged.
- Fixed-bill (`03`, `06`) and unmanaged VAs keep the single-settlement upsert path unchanged.
