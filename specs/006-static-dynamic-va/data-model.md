# Phase 1 Data Model: Static and Dynamic Virtual Account Creation

## Entity: VA Type Rule (originally static/hardcoded; now backed by `master_va_type` + `master_partner_service_ids`, see Amendment section below)

| partnerServiceId | additionalInfo.vaType | Mode | Billing | `customerNo` required from merchant? | `totalAmount` required? |
|---|---|---|---|---|---|
| 15973 | 01 | static | no bill | Yes | No |
| 15974 | 02 | static | variable bill | Yes | Yes (cumulative target) |
| 15975 | 03 | static | fixed bill | Yes | Yes (fixed) |
| 15973 | 04 | dynamic | no bill | No (must be empty) | No |
| 15974 | 05 | dynamic | variable bill | No (must be empty) | Yes (cumulative target) |
| 15975 | 06 | dynamic | fixed bill | No (must be empty) | Yes (fixed) |

Validation rules (→ FR-002, FR-003, FR-009, FR-011, FR-012):
- Reject if `(partnerServiceId, vaType)` is not one of the six rows above.
- Reject if mode is dynamic and `customerNo` is non-empty; reject if mode is static and `customerNo` is empty.
- Reject if billing is "fixed bill" (03/06) or "variable bill" (02/05) and `totalAmount` is missing.
- "No bill" (01/04) requests MUST NOT carry a `totalAmount`; if present, reject (validation policy consistent with FR-012).

## Entity: `va_transactions` (existing table — extended)

Extends `db/migrations/000003_create_va_transactions.up.sql`.

| Column | Type | Notes |
|---|---|---|
| `id` | VARCHAR(36) PK | existing |
| `partner_service_id` | VARCHAR(8) | existing |
| `customer_no` | VARCHAR(20) | existing; now also holds system-generated dynamic values (vaType+sequence) |
| `virtual_account_no` | VARCHAR(28) | existing |
| `va_type` | VARCHAR(2) | **new** — one of `01`..`06`; persists the resolved VA type classification (FR-010) |
| `total_amount` | NUMERIC(16,2) | existing; now also used as the cumulative target for variable-bill VAs |
| `paid_amount` | NUMERIC(16,2) | existing; kept as a denormalized running total for fast inquiry (== SUM of `va_payments.amount`) |
| `status` | VARCHAR(2) | existing (`03` pending / `00` paid / `02` expired / `04` deleted); reaches `00` when `paid_amount >= total_amount` for variable-bill types |
| ...remaining existing columns unchanged | | |

Migration: `db/migrations/000006_add_va_type_and_sequences.up.sql`
```sql
ALTER TABLE va_transactions ADD COLUMN IF NOT EXISTS va_type VARCHAR(2);
CREATE INDEX IF NOT EXISTS idx_va_transactions_customer_no ON va_transactions(partner_service_id, customer_no);
```

## Entity: `va_customer_no_sequences` (new table)

Backs dynamic `customerNo` generation (research.md §1).

| Column | Type | Notes |
|---|---|---|
| `va_type` | VARCHAR(2) PK | one of `04`, `05`, `06` |
| `next_seq` | BIGINT NOT NULL DEFAULT 1 | next value to assign; incremented under `SELECT ... FOR UPDATE` |
| `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |

Seed rows for `04`, `05`, `06` inserted by the migration (`next_seq = 1`).

Generation algorithm:
1. Acquire Redis lock keyed `va-seq-lock:{vaType}`.
2. `BEGIN`; `SELECT next_seq FROM va_customer_no_sequences WHERE va_type = $1 FOR UPDATE`.
3. If no row / DB unreachable / lock acquisition fails → return system-unavailable error with reason (FR-014).
4. `UPDATE va_customer_no_sequences SET next_seq = next_seq + 1, updated_at = NOW() WHERE va_type = $1`.
5. `customerNo := vaType + zeroPad(next_seq, 18)` (FR-005a). Reject before commit if the padded sequence would exceed 18 digits (edge case: exhausted range → system-unavailable error, not silent wraparound).
6. `COMMIT`; release lock.

## Entity: `va_payments` (new table)

Records each individual payment against a VA (variable-bill tracking, FR-013, SC-006).

| Column | Type | Notes |
|---|---|---|
| `id` | VARCHAR(36) PK | |
| `transaction_id` | VARCHAR(36) NOT NULL REFERENCES `va_transactions(id)` | |
| `amount` | NUMERIC(16,2) NOT NULL | individual payment amount |
| `reference_no` | VARCHAR(11) | payment/settlement reference, mirrors `va_transactions.reference_no` convention |
| `paid_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| `created_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |

Migration: `db/migrations/000007_create_va_payments.up.sql`
```sql
CREATE TABLE IF NOT EXISTS va_payments (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES va_transactions(id),
    amount NUMERIC(16,2) NOT NULL,
    reference_no VARCHAR(11),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_va_payments_transaction ON va_payments(transaction_id);
```

State transition (variable-bill VA, `va_transactions.status`):
`03` (pending, `paid_amount` < `total_amount`) → on each `va_payments` insert, `paid_amount` is recalculated → once `paid_amount >= total_amount`, `status` transitions to `00` (lunas/paid). This reuses the existing status enum — no new status values needed.

## Domain layer changes (`internal/domain/va.go`)

- `MerchantCreateVARequest`: confirm/add `VAType` (parsed from `additionalInfo.vaType`) as a derived field used internally for routing; `TotalAmount *Amount` already exists.
- `MerchantCreateVAResponse` / `MerchantVAData`: no shape change — `CustomerNo` already a plain string field; will simply carry the system-generated value for dynamic requests.
- New domain errors (`domain.NewDomainError`, following the existing `"400XXXX"`/`"500XXXX"` code convention already used in `merchant_va_usecase.go`):
  - Invalid `partnerServiceId`/`vaType` combination (400).
  - `customerNo` must be empty for dynamic `vaType` (400).
  - `customerNo` is required for static `vaType` (400).
  - `customerNo` already registered for `partnerServiceId` (409, mirrors existing `4092700` conflict pattern).
  - Missing `totalAmount` for fixed/variable bill `vaType` (400).
  - Sequence generator unavailable, with a `Reason` field/message describing the cause (500).
- New repository interface methods on `domain.VARepository`:
  - `NextCustomerNoSequence(ctx context.Context, vaType string) (string, error)`
  - `RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error` (returns a distinguishable "already exists" error)
  - `SaveVAPayment(ctx context.Context, transactionID string, amount string, referenceNo string) (paidAmount string, status string, err error)`

---

## Amendment (2026-07-28): VA Type / Partner Service ID Master Data + Cache

### Entity: `master_va_type` (new table)

Replaces the hardcoded `vaTypeRules` map in `internal/domain/va.go` as the source of truth for the six VA type rules (User Story 4, FR-015).

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | `gen_random_uuid()` default (or app-generated, consistent with existing `VARCHAR(36)` UUID-as-string convention used elsewhere in this schema — pick one and apply consistently at migration-authoring time) |
| `va_type` | VARCHAR(2) UNIQUE NOT NULL | e.g. `01`..`06` |
| `dynamic` | BOOLEAN NOT NULL | `true` for 04/05/06, `false` for 01/02/03 |
| `billing` | VARCHAR(10) NOT NULL, `CHECK (billing IN ('none','variable','fixed'))` | mirrors `domain.VATypeBilling` |
| `description` | VARCHAR(255) | human-readable, e.g. "Static no bill" |
| `partner_service_id` | VARCHAR(8) NOT NULL | the associated `partnerServiceId` for this `va_type` (FK-equivalent to `master_partner_service_ids.partner_service_id`, enforced at the application layer since the six rules are a fixed cross-reference, not a strict DB FK requirement) |
| `created_at` / `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | for auditing and to detect "has this row changed" if ever needed |

Seed data (migration): the six rows exactly matching the current `vaTypeRules` map (see spec.md's original rule table above), so first-boot behavior is unchanged.

### Entity: `master_partner_service_ids` (new table)

Replaces the hardcoded `reservedVAPartnerServiceIDs` map (User Story 4, FR-015).

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | see note above on UUID representation |
| `partner_service_id` | VARCHAR(8) UNIQUE NOT NULL | numeric per spec ("number, max length 8"), stored as `VARCHAR(8)` to match the existing `va_transactions.partner_service_id VARCHAR(8)` column convention and avoid leading-zero loss |
| `bank_code` | VARCHAR(20) UNIQUE NOT NULL | descriptive/reference only in this feature (not validated against request fields, per spec.md Assumptions) |
| `created_at` / `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |

Seed data (migration): the three rows for `15973`/`15974`/`15975` with placeholder `bank_code` values (to be corrected by the operator once real bank mappings are known — out of scope to determine here).

### Entity: VA Type Rule Cache (Redis, not a table)

| Redis key | Value | TTL | Refreshed |
|---|---|---|---|
| `master:va_types` | JSON array of all `master_va_type` rows | 5 minutes | On schedule (5-min ticker) and immediately after any mutation through the app's data-access layer |
| `master:partner_service_ids` | JSON array of all `master_partner_service_ids` rows | 5 minutes | Same as above |

An in-process (per-replica) last-known-good snapshot of both lists is held in memory, updated on every successful read (cache hit or Postgres refill), and served instead of erroring when Redis is temporarily unreachable (FR-018).

### Domain interface: `VATypeRuleProvider`

Replaces the current package-level `domain.LookupVATypeRule` / `domain.IsReservedVAPartnerServiceID` functions in `internal/domain/va.go` with a consumer-defined interface (Constitution I):

```go
type VATypeRuleProvider interface {
    LookupVATypeRule(ctx context.Context, partnerServiceID, vaType string) (rule VATypeRule, ok bool, err error)
    IsReservedPartnerServiceID(ctx context.Context, partnerServiceID string) (bool, error)
}
```

`MerchantVAUsecase` depends on this interface instead of calling the package-level functions directly; the concrete implementation (Postgres read + Redis cache + in-process fallback + 5-minute ticker + `RefreshNow()`) lives in a new `internal/infrastructure/cache` package per plan.md's amendment.
