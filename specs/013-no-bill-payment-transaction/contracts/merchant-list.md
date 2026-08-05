# Contract: Merchant Listing — VAs and Transactions Are Separate

**Endpoints**: `POST /list` (existing, `MerchantVAHandler.ListVA`) and a new transaction listing

**Change**: `ListVA` currently selects straight from `va_transactions` (`va_repository.go:343`), so a no-bill VA with 10 payments would render as 10 VAs. It is repointed at `va_accounts`, and a separate transaction listing is added for per-payment reconciliation. (FR-023, SC-007)

---

## `POST /list` — VA registrations

One row per registered virtual account number.

### Request

```json
{
  "partnerServiceId": "   15973",
  "virtualAccountNo": "",
  "status": "ACTIVE",
  "fromDate": "2026-08-01T00:00:00+07:00",
  "toDate": "2026-08-31T23:59:59+07:00",
  "page": 1,
  "pageSize": 20
}
```

`status` now filters on registration state (`ACTIVE` / `INACTIVE` / `EXPIRED`) rather than transaction state (`00`/`02`/`03`/`04`). `fromDate`/`toDate` filter on registration `created_at`.

### Response

```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "data": [
    {
      "virtualAccountNo": "   15973000000000000000001",
      "customerNo": "000000000000000001",
      "customerName": "John Doe",
      "vaType": "01",
      "status": "ACTIVE",
      "expiredDate": null,
      "createdAt": "2026-08-05T09:00:00+07:00",
      "transactionCount": 10,
      "totalPaid": { "value": "500000.00", "currency": "IDR" }
    }
  ],
  "pagination": { "page": 1, "pageSize": 20, "totalRows": 1, "totalPages": 1 }
}
```

`transactionCount` and `totalPaid` are aggregates over that VA number's settled `va_transactions` rows, so a merchant sees top-up activity without a second call.

### Coverage of legacy VAs

Rows backfilled by migration `000014` appear here identically to newly registered ones. VA numbers with no registration (unmanaged/legacy, no `va_type`) do **not** appear in this listing — they are reachable through the transaction listing below.

---

## `POST /list-transactions` — individual payments

One row per payment/transaction event. This is where a no-bill VA's N top-ups are visible.

### Request

```json
{
  "partnerServiceId": "   15973",
  "virtualAccountNo": "   15973000000000000000001",
  "status": "00",
  "fromDate": "2026-08-01T00:00:00+07:00",
  "toDate": "2026-08-31T23:59:59+07:00",
  "page": 1,
  "pageSize": 20
}
```

`status` keeps its existing transaction semantics (`00` paid, `02` expired, `03` pending, `04` deleted).

### Response

```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "data": [
    {
      "virtualAccountNo": "   15973000000000000000001",
      "customerNo": "000000000000000001",
      "customerName": "John Doe",
      "paymentRequestId": "PAY-20260805-0001",
      "referenceNo": "REF00000001",
      "paidAmount": { "value": "50000.00", "currency": "IDR" },
      "totalAmount": { "value": "50000.00", "currency": "IDR" },
      "status": "00",
      "transactionDate": "2026-08-05T10:15:00+07:00",
      "createdAt": "2026-08-05T10:15:01+07:00"
    }
  ],
  "pagination": { "page": 1, "pageSize": 20, "totalRows": 10, "totalPages": 1 }
}
```

Behavior is the existing `ListVA` query, unchanged in substance — only the route and the response item shape are new.

---

## Verification

| Setup | `/list` | `/list-transactions?virtualAccountNo=X` |
|---|---|---|
| 1 no-bill VA, 10 payments | 1 row, `transactionCount: 10` | 10 rows |
| 1 fixed-bill VA, 1 payment | 1 row, `transactionCount: 1` | 1 row |
| 1 no-bill VA, 0 payments | 1 row, `transactionCount: 0`, `totalPaid: "0.00"` | 0 rows |

## Non-goals

- No change to merchant authentication, pagination defaults (`pageSize` clamped to 1–100, default 20), or response envelope.
- No new dashboard UI; this is the API contract only.
