# Contract: Create VA — No-Bill Types (`01`, `04`) Register Only

**Endpoint**: Existing merchant create-VA endpoint (`POST /create-va` → `MerchantVAHandler.CreateVA` → `MerchantVAUsecase.CreateVA`)

**Change**: When the resolved `VATypeRule.Billing == domain.VATypeBillingNone` (VA types `01` and `04`), the usecase MUST persist a `va_accounts` registration and MUST NOT call `SaveInquiry`. The request/response wire format is unchanged.

## Request (unchanged)

```json
{
  "partnerServiceId": "   15973",
  "customerNo": "000000000000000001",
  "virtualAccountNo": "   15973000000000000000001",
  "virtualAccountName": "John Doe",
  "virtualAccountEmail": "john@example.com",
  "virtualAccountPhone": "628123456789",
  "trxId": "MERCHANT-TRX-001",
  "additionalInfo": {
    "vaType": "01",
    "dbUrlProcess": "https://merchant.example.com/callback"
  }
}
```

For `vaType: "04"`, `customerNo` MUST be empty and `virtualAccountNo` MAY be omitted (both are derived).

## Response (unchanged wire format)

```json
{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "   15973",
    "customerNo": "000000000000000001",
    "virtualAccountNo": "   15973000000000000000001",
    "virtualAccountName": "John Doe",
    "virtualAccountEmail": "john@example.com",
    "virtualAccountPhone": "628123456789",
    "trxId": "MERCHANT-TRX-001",
    "additionalInfo": { "vaType": "01", "dbUrlProcess": "https://merchant.example.com/callback" }
  }
}
```

`totalAmount` is absent from the response because a no-bill VA has no bill.

## Side effects

1. **Exactly one** `va_accounts` row is upserted (`ON CONFLICT (virtual_account_no) DO UPDATE`), setting `customer_name`, `customer_email`, `customer_phone`, `trx_id`, `notification_url`, `expired_date`, `status = 'ACTIVE'`, `updated_at = NOW()`.
2. **Zero** `va_transactions` rows are written. (FR-001, SC-002)
3. **Zero** `va_bill_details` rows are written — `billDetails` on a no-bill request is ignored.
4. For `vaType: "04"` only: one `va_customer_no_sequences.next_seq` increment, unchanged from today.

## Behavior on repeat call (FR-005)

| Scenario | Result |
|---|---|
| Same `virtualAccountNo`, `vaType: "01"`, changed `virtualAccountName` | `2002700`; registration's holder details updated; still zero transactions. |
| Same `virtualAccountNo`, `vaType: "01"`, identical payload | `2002700`; idempotent no-op update. |
| `vaType: "04"` called twice | Two distinct registrations with two distinct generated `customerNo` values — repeat-on-same-number is unreachable. |

Note the deliberate asymmetry with static **bill-bearing** types (`02`, `03`), which still return `4092701` on a repeat call — see `research.md` R-002 and FR-021.

## Error responses (existing codes, unchanged)

| Condition | Code |
|---|---|
| `totalAmount` present on a no-bill request | `4002706` Invalid Field Format [totalAmount must not be set for no-bill vaType] |
| `customerNo` non-empty for `vaType: "04"` | `4002703` |
| `customerNo` empty for `vaType: "01"` | `4002704` |
| `virtualAccountNo` ≠ `partnerServiceId + customerNo` for `vaType: "01"` | `4002707` |
| `virtualAccountName` / `trxId` missing | `4002701` |
| Master data unreachable | `5002702` System Unavailable [VA type master data: …] |
| Sequence generator unreachable (`04`) | `5002702` System Unavailable [sequence generator: …] |
| Registration persist failure | `5002700` Internal Server Error |

## Non-goals

- Bill-bearing types (`02`, `03`, `05`, `06`) keep calling `SaveInquiry` in addition to writing their registration. Their conflict-on-pending-transaction guard is untouched.
- Unmanaged/legacy requests (no `vaType`, non-reserved `partnerServiceId`) write no registration and behave exactly as today.
