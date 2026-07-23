# Implementation Plan: Merchant VA Dashboard

**Branch**: `003-merchant-va-dashboard` | **Date**: 2026-07-23 | **Spec**: `/specs/003-merchant-va-dashboard/spec.md`

**Input**: Feature specification from `/specs/003-merchant-va-dashboard/spec.md`

## Summary

Add internal merchant dashboard endpoints for creating VA transactions (SNAP service code 27, `VAUpsertRequest`), querying VA transaction lists, deleting pending VAs (SNAP service code 31, HTTP DELETE, `DeleteVARequest`), and processing payment notifications from banks (SNAP service code 25, `PaymentRequest`). All endpoints follow SNAP VA API format per ASPI OpenAPI specification (`aspi-open-api.yaml`). The feature extends the existing `va_transactions` and `va_bill_details` tables and reuses the established domain types, repository patterns, and SNAP middleware infrastructure.

## Technical Context

**Language/Version**: Go 1.26.5

**Primary Dependencies**: Echo v4 (HTTP framework), pgx v5 (PostgreSQL driver), go-redis v9 (caching/idempotency), OpenTelemetry SDK (observability), testify (testing)

**Storage**: PostgreSQL (existing `va_transactions` and `va_bill_details` tables via migration 000003), Redis (idempotency locks, response caching)

**Testing**: Go standard `testing` package, `testify/assert` + `testify/mock` for unit tests. Table-driven test pattern. Mock interfaces defined in test files.

**Target Platform**: Linux container (Alpine 3.20, distroless-compatible multi-stage Docker build, non-root user 10001:10001)

**Project Type**: Web-service (SNAP Payment Integration Gateway)

**Performance Goals**: Create VA ≤3s avg, list query ≤2s for ≤10k records, payment notifications ≤5s delivery, delete VA ≤1s avg, 500 concurrent create VA requests

**Constraints**: <200ms p95 for non-external-call endpoints, <100MB container memory, offline-capable (no external bank API dependency for merchant create/list)

**Scale/Scope**: Single-feature addition across ~7 new/modified files. Existing tables reused; migration 000004 adds columns to `va_transactions` and `va_bill_details` for SNAP compliance.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clean Architecture | ✅ PASS | New merchant usecase depends only on domain interfaces. Handlers call usecase, not repo directly. |
| II. Config-Driven Integrations | ✅ PASS | Merchant endpoints are internal; no new external provider config needed. Notification URL per-transaction in payload. |
| III. TDD | ✅ PASS | Will write failing tests first for merchant usecase and handler. Mock interfaces follow existing patterns. |
| IV. Context Propagation | ✅ PASS | All new functions accept `ctx context.Context` as first parameter. |
| V. Multi-Stage Docker | ✅ PASS | Existing Dockerfile already implements multi-stage build. No changes needed. |
| VI. Non-Root Container | ✅ PASS | Existing Dockerfile uses USER 10001:10001. No changes needed. |
| VII. Credential Store | ✅ PASS | No new secrets introduced. Merchant auth uses existing SNAP B2B token system. |
| VIII. OpenTelemetry | ✅ PASS | New HTTP endpoints auto-instrumented via existing `TelemetryMiddleware()`. |
| IX. Async Processing (Asynq) | ⚠️ GAP | Payment notification delivery to merchants needs Asynq workers. Requires new `adapter/delivery/worker/` and `infrastructure/queue/` packages. |
| X. Idempotency | ✅ PASS | Create VA uses existing IdempotencyMiddleware. Payment notifications use X-EXTERNAL-ID for dedup. Delete VA is idempotent. |
| XI. Test Coverage > 90% | ✅ PASS | TDD approach ensures coverage. Must verify `go test -coverprofile` threshold. |

**Gate Verdict**: PASS with NOTE — Asynq worker infrastructure (Principle IX) is a new dependency requiring `github.com/hibiken/asynq` to be added.

## ASPI OpenAPI Schema Reference

All schemas from `aspi-open-api.yaml`:

| Schema | Service Code | Used By |
|--------|-------------|---------|
| `VAUpsertRequest` | 27 (Create), 28 (Update) | Create VA endpoint |
| `VAUpsertResponse` | 27, 28 | Create VA response |
| `PaymentRequest` | 25 | Payment notification (bank → system) |
| `PaymentResponse` | 25 | Payment response |
| `DeleteVARequest` | 31 | Delete VA endpoint |
| `DeleteVAResponse` | 31 | Delete VA response |
| `InquiryVARequest` | 30 | Single VA lookup (not used in this feature) |
| `BillDetail` | All | Bill details in all VA operations |
| `Amount` | All | Monetary amount with currency |
| `ResponseStatus` | All | Standard response wrapper |

### Key Schema Corrections (from ASPI YAML)

- `trxId` is **REQUIRED** in `VAUpsertRequest` (Create VA)
- `totalAmount`, `billDetails`, `expiredDate`, `virtualAccountTrxType`, `feeAmount` are **OPTIONAL** in `VAUpsertRequest`
- `BillDetail` has **14 fields** (not 9): added `billAmountLabel`, `billAmountValue`, `billerReferenceId`, `reason` (MultiLangText), `additionalInfo`
- `PaymentRequest` has many optional fields: `cumulativePaymentAmount`, `paidBills`, `journalNum`, `paymentType`, `flagAdvise`
- `responseCode` is **STRING** (max 7 chars), not integer
- `CHANNEL-ID` header is **OPTIONAL**
- `paymentRequestId` max length is **128** (not 30)

## Project Structure

### Documentation (this feature)

```text
specs/003-merchant-va-dashboard/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── va.go                    # EXISTING — extend with MerchantVAUsecase interface, VAUpsertRequest, DeleteVARequest, PaymentNotificationPayload, BillDetail (14 fields), VAListItem types
│   └── errors.go                # EXISTING — add merchant-specific error constants
├── usecase/
│   ├── va_usecase.go            # EXISTING — modify Payment to enqueue notification, add expiry check
│   └── merchant_va_usecase.go   # NEW — MerchantVAUsecase implementation (create, list, delete)
├── adapter/
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── handler/
│   │   │   │   ├── va_handler.go           # EXISTING — unchanged (vendor-facing)
│   │   │   │   └── merchant_va_handler.go  # NEW — MerchantVA handler for create-va/list/delete-va
│   │   │   └── middleware/
│   │   │       ├── idempotency.go          # EXISTING — reuse for create VA
│   │   │       └── snap_auth.go            # EXISTING — reuse for merchant auth
│   │   └── worker/
│   │       └── payment_notification_worker.go  # NEW — Asynq worker for merchant webhook notification
│   └── gateway/
│       └── snap/client.go        # EXISTING — unchanged
├── infrastructure/
│   ├── database/
│   │   └── va_repository.go      # EXISTING — extend with ListVA, GetVABillDetails, UpdateVAStatus, GetVAByVirtualAccountNo methods
│   ├── redis/
│   │   └── redis.go             # EXISTING — unchanged
│   ├── queue/
│   │   └── asynq.go             # NEW — Asynq client/enqueuer wrapper
│   └── telemetry/
│       └── otel.go              # EXISTING — unchanged
cmd/
└── api/
    └── main.go                  # EXISTING — refactor routes to unified /v1.0/transfer-va/*, add create-va/list/delete-va endpoints
db/
└── migrations/
    └── 000004_add_va_fields.up.sql   # NEW — add SNAP-compliant columns to va_transactions and va_bill_details
    └── 000004_add_va_fields.down.sql  # NEW
```

### Migration 000004 — Columns to Add

**va_transactions:**
- `customer_name` VARCHAR(255) NOT NULL — SNAP: virtualAccountName
- `customer_email` VARCHAR(255) — SNAP: virtualAccountEmail
- `customer_phone` VARCHAR(30) — SNAP: virtualAccountPhone
- `trx_id` VARCHAR(64) NOT NULL — SNAP: trxId (required in VAUpsertRequest)
- `expired_date` TIMESTAMPTZ — SNAP: expiredDate
- `virtual_account_trx_type` VARCHAR(1) DEFAULT 'C' — SNAP: C,O,I,M,L,N,X
- `notification_url` VARCHAR(512) NOT NULL — Merchant webhook URL
- `journal_num` VARCHAR(6) — SNAP: journalNum
- `payment_type` VARCHAR(1) — SNAP: paymentType
- `flag_advise` VARCHAR(1) — SNAP: flagAdvise
- `paid_bills` VARCHAR(6) — SNAP: paidBills (hex)

**va_bill_details:**
- `bill_amount_currency` VARCHAR(3) — SNAP: billAmount.currency
- `bill_amount_label` VARCHAR(25) — SNAP: billAmountLabel
- `bill_amount_value` VARCHAR(25) — SNAP: billAmountValue
- `biller_reference_id` VARCHAR(64) — SNAP: billerReferenceId
- `reason_en` VARCHAR(64) — SNAP: reason.english
- `reason_id` VARCHAR(64) — SNAP: reason.indonesia

### Route Structure (SNAP ASPI-Compliant)

All routes live under `/v1.0/transfer-va/*`.

```
/v1.0/
├── access-token/b2b              # POST — B2B token (existing)
└── transfer-va/
    ├── inquiry                   # POST — VA inquiry (existing, service code 24)
    ├── payment                   # POST — VA payment notification (existing, service code 25)
    ├── status                    # POST — VA status inquiry (existing, service code 26)
    ├── create-va                 # POST — Create VA (NEW, service code 27)
    ├── list                      # POST — List VA transactions (NEW, convenience API)
    └── delete-va                 # DELETE — Delete VA transaction (NEW, service code 31)
```

**Why**: SNAP standard mandates unified paths after `/v1.0/`. Vendor/channel differentiation via SNAP auth middleware. Delete VA uses HTTP DELETE per ASPI spec.

## Complexity Tracking

> No constitution violations requiring justification. The Asynq dependency addition (Principle IX gap) is addressed via adapter pattern consistent with Redis and PostgreSQL infrastructure.
