# Phase 0 Research: Static and Dynamic Virtual Account Creation

All Technical Context fields were resolvable directly from the existing codebase (no `NEEDS CLARIFICATION` markers remained after `/speckit-clarify`). This document records the decisions made for the mechanisms the spec's clarifications require that don't yet exist in the codebase: sequential `customerNo` generation, concurrency-safe locking, and per-payment recording for variable-bill VAs.

## 1. Sequential, race-free `customerNo` generation for dynamic VAs

- **Decision**: Use a dedicated PostgreSQL table `va_customer_no_sequences` keyed by `va_type` (2-digit code), with a single `next_seq BIGINT` counter column, incremented inside a transaction using `SELECT ... FOR UPDATE` before use. The resulting `customerNo` is built in the usecase layer as `vaType (2 digits) + next_seq zero-padded to 18 digits`.
- **Rationale**: The codebase has no existing DB sequence, Redis `INCR`, or counters table (confirmed via codebase search). A row-locked counter table keeps the source of truth in PostgreSQL — the constitution's single source of truth for transactional data (Principle IX) — and reuses the existing pgx v5 transaction/connection pool rather than introducing a new stateful dependency. `SELECT ... FOR UPDATE` on a single narrow row is a well-understood, low-contention pattern for per-key counters.
- **Alternatives considered**:
  - *Postgres native `SEQUENCE` object per `vaType`*: Rejected — native sequences don't participate in transaction rollback semantics the same way as a row update (gaps on rollback are usually fine, but a plain counter row keeps behavior explicit and auditable, and avoids needing 3 new `CREATE SEQUENCE` objects plus DDL-level `ALTER SEQUENCE` for reset scenarios).
  - *Redis `INCR`*: Rejected as sole source of truth — Redis is used for caching/locks/idempotency in this codebase (Principle IX), not as the durable system of record; using it for the authoritative sequence would risk divergence from PostgreSQL on Redis data loss/eviction.

## 2. Concurrency-safe locking for both dynamic sequence increment and static `customerNo` registration

- **Decision**: Reuse the existing Redis distributed lock helpers (`internal/infrastructure/redis/redis.go`: `AcquireLock` / `ReleaseLock`, already used by the idempotency middleware) as a request-level lock keyed by `va_type` (for dynamic generation) or `partnerServiceId:customerNo` (for static registration), wrapping the PostgreSQL transaction that performs the counter increment or uniqueness check + insert.
- **Rationale**: The same lock primitive already exists and is tested in production for the idempotency middleware; adding a second, narrowly-scoped lock key avoids introducing a new locking mechanism/dependency (KISS/DRY, Principle I) while still guaranteeing only one request at a time performs the read-modify-write. The underlying `SELECT ... FOR UPDATE` also provides a second layer of DB-level protection if two processes ever raced past the Redis lock (defense in depth), consistent with Constitution IX's Redis-for-locks + Postgres-for-truth split.
- **Alternatives considered**:
  - *Postgres row lock only (no Redis)*: Considered simpler, but a bare `FOR UPDATE` transaction under high concurrency serializes on the DB connection pool with the risk of connection-pool starvation; the existing Redis lock pattern already handles retry/backoff behavior the team is familiar with operationally.
  - *Application-level in-memory mutex*: Rejected — the service runs as multiple replicas (implied by the existing Redis-based idempotency lock, which exists specifically because in-memory locks don't work across replicas).

## 3. Recording individual payments for "variable bill" VAs

- **Decision**: Add a new `va_payments` table (one row per received payment, FK to `va_transactions.id`), summed at read/inquiry time (or maintained via an incrementally-updated `paid_amount` on `va_transactions` updated in the same transaction as each insert) to determine "lunas" (fully paid) status once the sum reaches `total_amount`.
- **Rationale**: `va_transactions` already has a single `paid_amount` column (from `000004_add_va_fields.up.sql`), but the spec's clarification requires each payment to be **individually recorded**, which a single scalar column cannot satisfy. A child table mirrors the existing `va_bill_details` pattern (one parent transaction row, many child detail rows) already used in this codebase, keeping the new addition idiomatic rather than introducing a new persistence paradigm.
- **Alternatives considered**:
  - *JSONB array of payments on `va_transactions`*: Rejected — the existing schema convention for one-to-many VA data is a dedicated child table (`va_bill_details`), and a relational table gives indexable, queryable payment history (needed for reconciliation) without JSONB query complexity.
  - *Only updating the existing scalar `paid_amount`*: Rejected — does not satisfy FR-013 / SC-006's requirement that each payment be individually recorded and auditable.

## Summary

| Area | Decision |
|---|---|
| Dynamic `customerNo` generation | Row-locked counter table `va_customer_no_sequences`, keyed by `va_type`, producing an 18-digit zero-padded sequence |
| `customerNo` format | 20 digits total = 2-digit `vaType` + 18-digit sequence (dynamic); merchant-defined ≤20 digits (static) |
| Concurrency control | Existing Redis `AcquireLock`/`ReleaseLock` helper, keyed per `va_type` (dynamic) or `partnerServiceId:customerNo` (static), wrapping a Postgres transaction with `SELECT ... FOR UPDATE` |
| Variable-bill payment tracking | New `va_payments` child table, one row per payment, summed against `va_transactions.total_amount` for "lunas" determination |
| Sequence-unavailable error | Surfaced as a new `domain.DomainError` code (5xx) with a reason field describing the failure cause (e.g., lock timeout, DB unavailable) |

No unresolved `NEEDS CLARIFICATION` items remain.

## Amendment (2026-07-28): Master Data + Cache Refresh Strategy

### 4. Moving VA type rules and reserved partner service IDs to database-backed master data

- **Decision**: Introduce `master_va_type` and `master_partner_service_ids` tables (see data-model.md), seeded via migration with the six existing rules and three existing reserved IDs so first-boot behavior is byte-for-byte identical to the current hardcoded maps. Replace the current package-level `domain.LookupVATypeRule`/`domain.IsReservedVAPartnerServiceID` functions with a `domain.VATypeRuleProvider` interface, implemented by a new infrastructure component that reads from this data.
- **Rationale**: Constitution Principle II requires configuration-driven behavior for exactly this kind of routing/mapping data; hardcoded Go maps fail that principle once an operator needs to add a VA type or partner service ID without a deploy. A DB table is the natural fit since Constitution IX already designates PostgreSQL as the single source of truth for this class of data, and the existing migration-based schema evolution process applies unchanged.
- **Alternatives considered**:
  - *Config file (YAML/JSON) hot-reloaded from disk*: Rejected — the constitution's Directory Layout Convention already has a `config/` package for *dynamic configuration parsing* aimed at payment-gateway/provider config, but this data is more naturally relational (two small normalized tables) and benefits from the same durability/backup/migration tooling as the rest of the domain's data, rather than a separate file-distribution mechanism across replicas.
  - *Keep hardcoded, add a feature flag per new rule*: Rejected — doesn't scale past a handful of flags and still requires a deploy per change, defeating the stated goal.

### 5. Serving lookups from Redis instead of PostgreSQL on every request

- **Decision**: Cache both tables' full contents in Redis under two keys (`master:va_types`, `master:partner_service_ids`) as serialized JSON, with a 5-minute TTL as the safety-net refresh interval, PLUS an explicit `RefreshNow()` call made synchronously by whatever code path mutates either table through the application. A small in-process (per-instance) snapshot is kept as a fallback: on every successful cache read (whether cache-hit or cache-refill-from-Postgres), the in-process copy is updated; if Redis is unreachable, the provider serves the last such snapshot instead of failing the request outright (FR-018).
- **Rationale**: This mirrors the existing `ClientKeyCache` pattern in `internal/infrastructure/redis/client_key_cache.go` (a thin Redis-backed cache wrapper with TTL), so it's idiomatic for this codebase rather than introducing a new caching paradigm. Since Redis is shared across all service replicas, a write-through refresh from any one instance is immediately visible to all others on their next Redis read — no pub/sub broadcast or per-instance invalidation signal is needed, keeping the design simple (KISS/YAGNI, Constitution I) while still meeting the "immediate refresh on change" requirement (FR-017) for changes made via the app itself. Out-of-band DB edits (e.g., a raw SQL `UPDATE` bypassing the app) are explicitly out of scope for "immediate" and fall back to the 5-minute scheduled refresh, which is called out as an Assumption in spec.md to set the right expectation.
- **Alternatives considered**:
  - *Redis pub/sub to broadcast invalidation to all replicas' local in-memory caches*: Rejected for this iteration — adds a new inter-process signaling mechanism and a subscriber lifecycle to manage, for a benefit (avoiding one Redis round-trip per lookup) that doesn't matter at this data's tiny scale (6-ish rows). Documented as a natural future optimization if `/create-va` throughput ever makes even a Redis round-trip per request material.
  - *Postgres `LISTEN`/`NOTIFY`*: Rejected — introduces a persistent DB connection/listener per instance purely for this small, low-frequency invalidation signal; the write-through-to-shared-Redis approach achieves the same effect with infrastructure the codebase already has.
  - *No caching, direct Postgres read per request with a connection-pool-level statement cache*: Rejected — doesn't satisfy SC-009's explicit intent (query volume must not scale linearly with request volume), and re-queries data that changes on the order of "operator edits per month," not per request.

## Summary (amendment)

| Area | Decision |
|---|---|
| Master data storage | `master_va_type` (id, va_type, dynamic, billing, description) and `master_partner_service_ids` (id, partner_service_id, bankCode) tables, seeded with today's hardcoded values |
| Domain interface | `domain.VATypeRuleProvider` (consumer-defined per Constitution I) replaces the current package-level lookup functions |
| Cache | Redis, JSON-serialized full-table snapshots, 5-minute TTL, keyed `master:va_types` / `master:partner_service_ids` |
| Refresh-on-change | Synchronous write-through to Redis from the same call that mutates a row via the app's own data-access layer; out-of-band DB edits rely on the 5-minute TTL instead |
| Redis-unavailable fallback | Per-instance in-process snapshot updated on every successful read, served when Redis is unreachable |

No unresolved `NEEDS CLARIFICATION` items remain for this amendment.
