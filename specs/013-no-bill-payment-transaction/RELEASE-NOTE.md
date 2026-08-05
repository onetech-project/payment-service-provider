# Release Note: No-Bill VA — Register Once, Pay Many Times

**Feature**: `013-no-bill-payment-transaction`
**Migration**: `000013_create_va_accounts`

## What changed

No-bill Virtual Accounts (`vaType` `01` static, `04` dynamic) could previously be paid **exactly once**. `/create-va` created a pending transaction, the first payment consumed it, and every payment after that was rejected with `4092500` "already paid or inactive" — forcing merchants to re-register the VA before each payment.

A no-bill VA is now a durable payment address, like an e-wallet top-up number:

| | Before | After |
|---|---|---|
| `/create-va` for `01`/`04` | creates a pending transaction | registers the VA only — **zero** transactions |
| 2nd payment to the same VA | `4092500` rejected | succeeds, as its own transaction |
| `/create-va` again on the same number | `4092700` / `4092701` conflict | `2002700`, updates holder details |
| `/delete-va` for `01`/`04` | cancels the pending transaction | deactivates the registration |

VA identity moved into a new `va_accounts` table; `va_transactions` now means one row per payment/inquiry event.

**Bill-bearing types (`02`, `03`, `05`, `06`) are unchanged** — same transaction-at-create-VA behavior, same conflict rules, same cumulative settlement.

## Breaking change: `POST /openapi/v1.0/transfer-va/list`

This endpoint now returns **one entry per registered VA number** instead of one per transaction. Listing transactions there was wrong once a VA could hold many payments: a no-bill VA paid ten times rendered as ten separate VAs.

**Response item shape changed:**

```diff
- { virtualAccountNo, customerNo, customerName, totalAmount, paidAmount,
-   status, expiredDate, createdAt, transactionDate }
+ { virtualAccountNo, customerNo, customerName, vaType, status, expiredDate,
+   createdAt, transactionCount, totalPaid }
```

**`status` filter semantics changed** — it now filters registration state (`ACTIVE` / `INACTIVE` / `EXPIRED`), not transaction state (`00` / `02` / `03` / `04`).

**Migration path for existing clients:** `POST /openapi/v1.0/transfer-va/list-transactions` is new and returns the per-payment view with the same filters and the old transaction `status` semantics. A client that needs the previous data should point at that endpoint. Its item shape adds `paymentRequestId` and `referenceNo`:

```json
{ "virtualAccountNo": "...", "customerNo": "...", "customerName": "...",
  "paymentRequestId": "PAY-1", "referenceNo": "REF1",
  "paidAmount": {...}, "totalAmount": {...}, "status": "00",
  "transactionDate": "...", "createdAt": "..." }
```

**Unchanged:** authentication, signing, pagination defaults (`pageSize` clamped 1–100, default 20), and the response envelope (`responseCode` / `responseMessage` / `data` / `pagination`).

## Vendor-facing API

No request or response schema changed on `/inquiry`, `/payment`, `/status`, or `/create-va`. This is a change to *when* records are written and *what blocks a payment*.

One behavioral note for no-bill VAs: `totalAmount` in the response now echoes the payment amount, and the vendor's `totalAmount` is no longer compared against `paidAmount` (a no-bill VA has no bill amount to match against). A payment of zero or less is now rejected with `4002501`.

## Deployment

The migration only creates a table — it alters no existing column and rewrites no existing row. It backfills one registration per distinct managed `virtual_account_no` from `va_transactions`, so VAs created before this release keep working.

Deploy the migration and the application together. If no registration is found, both the inquiry and payment paths fall through to the previous transaction-based behavior, so a partial rollout degrades to the old behavior rather than to a hard failure.

**Rollback:** revert the application, then `migrate down 1`. Because of the fall-through above, reverting the application while leaving `va_accounts` in place is also safe.
