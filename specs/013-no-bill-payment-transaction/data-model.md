# Phase 1 Data Model: No-Bill VA — Transaction Created at Payment

**Feature**: `013-no-bill-payment-transaction`
**Date**: 2026-08-05

---

## Overview

The change splits one overloaded table into two clearly-scoped ones:

```
BEFORE                              AFTER

va_transactions                     va_accounts            (NEW — VA identity, 1 row per VA number)
  = VA identity                       │
  + transaction                       │ 1
                                      │
                                      │ N
                                    va_transactions        (unchanged shape — 1 row per payment/inquiry event)
                                      │
                                      │ N
                                    va_bill_details, va_payments  (unchanged)
```

---

## New Entity: `va_accounts`

One row per virtual account number. Created by `/create-va` for managed VA types (`01`–`06`). Never created by inquiry or payment.

### Columns

| Column | Type | Null | Notes |
|---|---|---|---|
| `id` | `VARCHAR(36)` | no | PK, app-generated UUID (`google/uuid`), matching the convention in `va_transactions.id` and `master_va_type.id`. |
| `partner_service_id` | `VARCHAR(8)` | no | |
| `customer_no` | `VARCHAR(20)` | no | Merchant-supplied for static, generated for dynamic. |
| `virtual_account_no` | `VARCHAR(28)` | no | **UNIQUE.** Equals `partner_service_id \|\| customer_no` for managed types. |
| `va_type` | `VARCHAR(2)` | no | `01`–`06`. FK-by-convention to `master_va_type.va_type` (no hard FK — master data is operator-editable). |
| `customer_name` | `VARCHAR(255)` | no | VA holder name; the name returned by inquiry. |
| `customer_email` | `VARCHAR(255)` | yes | |
| `customer_phone` | `VARCHAR(30)` | yes | |
| `trx_id` | `VARCHAR(64)` | no | Merchant's own reference from the registering `/create-va`. |
| `notification_url` | `VARCHAR(512)` | yes | From `additionalInfo.dbUrlProcess`. Empty means no callback. |
| `status` | `VARCHAR(10)` | no | `ACTIVE` \| `INACTIVE` \| `EXPIRED`. Default `ACTIVE`. |
| `expired_date` | `TIMESTAMPTZ` | yes | `NULL` = never expires (spec A-004). |
| `created_at` | `TIMESTAMPTZ` | no | Default `NOW()`. |
| `updated_at` | `TIMESTAMPTZ` | no | Default `NOW()`. |

### Constraints and Indexes

| Name | Definition | Why |
|---|---|---|
| `va_accounts_pkey` | `PRIMARY KEY (id)` | |
| `va_accounts_virtual_account_no_key` | `UNIQUE (virtual_account_no)` | The `ON CONFLICT` target for registration upsert (FR-005); makes the payment/inquiry lookup exact. |
| `uq_va_accounts_partner_customer` | `UNIQUE (partner_service_id, customer_no)` | Replaces the check-then-act in `RegisterStaticCustomerNo` with a real constraint (FR-007). |
| `idx_va_accounts_partner_service_id` | `INDEX (partner_service_id)` | Merchant dashboard listing (FR-023). |
| `chk_va_accounts_status` | `CHECK (status IN ('ACTIVE','INACTIVE','EXPIRED'))` | Mirrors the `billing` CHECK style already used on `master_va_type`. |

### State Transitions

```
                    /create-va
                        │
                        ▼
                     ACTIVE ──────── delete-va ──────▶ INACTIVE
                        │                                 │
                        │ expired_date passes,             │ delete-va (idempotent no-op)
                        │ detected at inquiry/payment      │
                        ▼                                 ▼
                     EXPIRED ◀───────────────────────  (terminal)
```

- `ACTIVE → EXPIRED` is applied with a `WHERE status = 'ACTIVE'` guard so the expiry callback fires exactly once (R-005, FR-017).
- `ACTIVE → INACTIVE` via delete-VA (FR-019); repeating it returns success without a second update.
- No transition ever deletes or modifies historical `va_transactions` rows (FR-020).

### Validation Rules

| Rule | Source |
|---|---|
| `virtual_account_no` must equal `partner_service_id + customer_no` for static types. | FR-004 (existing `vaNoMatchesPartnerAndCustomer`) |
| `customer_no` must be empty in the request for dynamic types, non-empty for static. | Existing `merchant_va_usecase.go:88-93` |
| No-bill types (`billing = 'none'`) must not carry `totalAmount`. | FR-006 (existing check at `merchant_va_usecase.go:98`) |
| `customer_name` (`virtualAccountName`) is mandatory. | Existing `merchant_va_usecase.go:67` |

---

## Modified Entity: `va_transactions`

**No schema change.** Only its semantics narrow, and how rows are produced changes.

| Aspect | Before | After |
|---|---|---|
| Row per no-bill `/create-va` | 1 pending (`status='03'`) row | **none** (FR-001) |
| Row per no-bill payment | upsert onto the pending row | **1 new settled (`status='00'`) row** (FR-008) |
| Row per no-bill inquiry | 1 inquiry row when no VA record exists | **none** (FR-016) |
| `inquiry_request_id` for a no-bill payment | fallback chain `inquiryRequestId → trxId → paymentRequestId` | **always `paymentRequestId`** (R-003) |
| `total_amount` for a no-bill payment | vendor `totalAmount` else `paidAmount` | **`paidAmount`** (FR-009) |
| Row per bill-bearing VA (`02`,`03`,`05`,`06`) | unchanged | unchanged (FR-021) |

Existing rows are not migrated or rewritten (FR-022, R-006).

---

## Domain Types (`internal/domain/va.go`)

### New

```
VAAccount
  ID, PartnerServiceID, CustomerNo, VirtualAccountNo, VAType,
  CustomerName, CustomerEmail, CustomerPhone,
  TrxID, NotificationURL, Status, ExpiredDate, CreatedAt, UpdatedAt

VAAccountStatus constants: VAAccountStatusActive / Inactive / Expired

VAAccountListItem  — dashboard projection (VA number, customer no, name, va type,
                     status, expired date, created at, transaction count, total paid)
```

### Extended: `VARepository` interface

```
+ SaveVAAccount(ctx, *VAAccount) error                    // upsert on virtual_account_no
+ GetVAAccount(ctx, virtualAccountNo string) (*VAAccount, error)
+ GetVAAccountByPartnerAndCustomer(ctx, partnerServiceID, customerNo string) (*VAAccount, error)
+ UpdateVAAccountStatus(ctx, virtualAccountNo, status string) error   // guarded on ACTIVE
+ ListVAAccounts(ctx, *VAAccountListFilter) ([]VAAccountListItem, int, error)
+ SaveNoBillPayment(ctx, *VAPaymentRecord) error          // plain INSERT; conflict ⇒ ErrVAPaymentDuplicate
```

`SaveVAPayment`, `NextCustomerNoSequence`, and the rest of the interface are unchanged. `RegisterStaticCustomerNo` is retained but reimplemented against `va_accounts`.

### New Domain Errors

| Error | Meaning | Mapped response |
|---|---|---|
| `ErrVAAccountNotFound` | No registration for this VA number. | `4042419` (inquiry) / `4042519` (payment) |
| `ErrVAAccountInactive` | Registration deactivated. | `4042419` / `4042519` |
| `ErrVAPaymentDuplicate` | `paymentRequestId` already recorded. | Replayed success (not surfaced) |

---

## Relationships

| From | To | Cardinality | Key |
|---|---|---|---|
| `va_accounts` | `va_transactions` | 1 : N | `virtual_account_no` (soft link, no FK — legacy transactions predate the registry) |
| `va_transactions` | `va_bill_details` | 1 : N | `transaction_id` (existing FK) |
| `va_transactions` | `va_payments` | 1 : N | `transaction_id` (existing FK, variable-bill only) |
| `va_accounts` | `master_va_type` | N : 1 | `va_type` (soft link — master data is operator-editable) |

No hard foreign key is placed from `va_transactions` to `va_accounts`: transactions created before this feature have no registration, and adding the constraint would fail the migration on any existing dataset.

---

## Migration

`db/migrations/000013_create_va_accounts.up.sql` / `.down.sql`

1. `CREATE TABLE IF NOT EXISTS va_accounts (...)` with the constraints above.
2. Backfill one row per distinct `virtual_account_no` from `va_transactions WHERE va_type IN ('01','02','03','04','05','06')`, taking holder fields from the most recent row per VA number (`DISTINCT ON (virtual_account_no) ... ORDER BY virtual_account_no, created_at DESC`), with status derived from that latest row: `'04'` (deleted) → `INACTIVE`, `'02'` (expired) → `EXPIRED`, anything else → `ACTIVE`. Mapping `'02'` to `EXPIRED` rather than `ACTIVE` avoids briefly resurrecting an already-expired VA; `expired_date` is copied across too, so the inline detection would reach the same conclusion on the next inquiry either way.
3. `ON CONFLICT DO NOTHING`, untargeted so it covers both unique constraints, making re-runs safe and tolerating any legacy row pair that would violate `uq_va_accounts_partner_customer`.

Down: `DROP TABLE IF EXISTS va_accounts;` — no existing table or row is altered by the up migration, so rollback is exact.
