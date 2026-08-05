# Contract: Expiry and Deactivation of a No-Bill Registration

**Endpoints**: `VAUsecase.Inquiry`, `VAUsecase.Payment` (expiry detection), `MerchantVAUsecase.DeleteVA` (deactivation)

**Change**: Expiry for a no-bill VA is detected on the **registration** (`va_accounts`) rather than on a pending transaction, because after this feature no pending transaction exists. The existing exactly-once callback guarantee from feature `007-merchant-expiry-callback` is preserved by moving the guard one level up.

---

## Part 1 — Expiry detection

**Trigger**: `va_accounts.expired_date IS NOT NULL AND NOW() > expired_date`, evaluated inline during inquiry or payment. No background scanner (unchanged design from `007`).

`expired_date IS NULL` means the registration never expires (spec A-004).

### Response — inquiry

```json
{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "inquiryStatus": "01",
    "inquiryReason": { "english": "expired transaction", "indonesia": "transaksi kadaluarsa" }
  }
}
```

### Response — payment

```json
{
  "responseCode": "4042519",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": { "english": "expired transaction", "indonesia": "transaksi kadaluarsa" }
  }
}
```

### Side effects (exactly once per registration)

1. `UpdateVAAccountStatus(ctx, virtualAccountNo, 'EXPIRED')`, executed as `UPDATE va_accounts SET status='EXPIRED' WHERE virtual_account_no=$1 AND status='ACTIVE'`. Zero rows affected ⇒ another call already transitioned it ⇒ **stop here, enqueue nothing**. This mirrors the `WHERE status='03'` guard that `UpdateVAStatus` uses today.
2. If the transition applied AND the registration has a `notification_url` AND no prior auto-triggered `va.expired` delivery row exists for this VA number (`ExistsByVirtualAccountNoAndEventType`, reused unchanged): enqueue one `va.expired` notification.
3. Record the delivery attempt in `va_notification_deliveries`.
4. **Zero** `va_transactions` rows created or modified.
5. Enqueue is asynchronous and best-effort — it must never delay or fail the SNAP response.

### Already-EXPIRED registration

Subsequent inquiries and payments keep returning the same expired response above, and step 1's guard makes steps 2–3 no-ops. No duplicate callback is ever sent.

---

## Part 2 — Deactivation via delete-VA

**Endpoint**: `POST /delete-va` → `MerchantVAUsecase.DeleteVA`

**Change**: When the target VA number resolves to a no-bill `va_accounts` registration, delete-VA deactivates the **registration** instead of cancelling a pending transaction (FR-019).

### Response

```json
{
  "responseCode": "2003100",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "   15973",
    "customerNo": "000000000000000001",
    "virtualAccountNo": "   15973000000000000000001",
    "trxId": "MERCHANT-TRX-001"
  }
}
```

### Behavior matrix

| Registration state | Result |
|---|---|
| `ACTIVE` | → `INACTIVE`, `2003100`. (US6 AS1) |
| `INACTIVE` | `2003100`, idempotent no-op. (US6 AS4) |
| `EXPIRED` | `2003100`, no state change — already terminal. |
| No registration (legacy VA) | Existing transaction-based delete path applies unchanged. |

### Side effects

1. `va_accounts.status → 'INACTIVE'`, `updated_at = NOW()`.
2. **Zero** modifications to historical `va_transactions` rows — prior settled payments remain readable and unchanged, including via status queries. (FR-020, US6 AS3)
3. No callback is enqueued — deactivation is merchant-initiated, so the merchant already knows.

### Post-deactivation

| Request | Result |
|---|---|
| Payment for this VA number | `4042519` Invalid Bill/Virtual Account. (US6 AS2) |
| Inquiry for this VA number | `4042419` Invalid Bill/Virtual Account. (US3 AS3) |
| Status query for a prior payment | Unchanged — returns that payment's original data. (US6 AS3) |
| `/create-va` for this VA number again | Reactivates the registration (`status → 'ACTIVE'`) via the FR-005 upsert. |

## Non-goals

- Bill-bearing VA types keep the existing transaction-level expiry and delete behavior from feature `007-merchant-expiry-callback`, unchanged. (FR-021)
- No new event types are introduced; `va.expired` and `va.payment.received` are reused as-is.
