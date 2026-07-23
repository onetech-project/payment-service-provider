# Implementation Plan: SNAP VA Field & Validation Compliance Fix

**Branch**: `004-snap-va-field-compliance` | **Date**: 2026-07-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-snap-va-field-compliance/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Correct three SNAP ASPI Virtual Account field/validation defects that break vendor interoperability: (1) rename the Inquiry request's transaction-init-date JSON field from the incorrect `trxDateInit` to the spec-correct `txnDateInit`; (2) add the spec-mandatory `amount` field to `VAInquiryRequest`, bind and validate it; (3) fix `VAPaymentRequest`/`Payment` usecase validation to require only `paymentRequestId` + `paidAmount` (per spec), dropping the non-spec `transactionDate` mandatory check and the `totalAmount` mandatory check. All downstream Go code (repository records, DTOs, existing tests) that reference the old field/validation behavior must be updated in lockstep — no legacy aliasing.

## Technical Context

**Language/Version**: Go (latest stable, per project constitution)

**Primary Dependencies**: Echo (HTTP), existing internal `domain`/`usecase`/`infrastructure` packages; no new dependencies required

**Storage**: PostgreSQL (via `internal/infrastructure/database/va_repository.go`) — `VAInquiryRecord`/`VAPaymentRecord` structs carry `TotalAmount`/`TransactionDate` columns that must stay consistent with the corrected domain rules

**Testing**: Go's built-in `testing` package (table-driven tests), existing suite in `internal/usecase/va_usecase_test.go`, `internal/domain/va_test.go`, handler tests

**Target Platform**: Linux server (containerized, per constitution Principle V)

**Project Type**: Single Go backend service (Clean Architecture: domain / usecase / adapter / infrastructure)

**Performance Goals**: N/A — pure correctness fix, no performance-sensitive path touched

**Constraints**: Must maintain >90% test coverage per constitution Principle XI; must not introduce backward-compatibility shims for old field names per project convention

**Scale/Scope**: Narrow, surgical change scoped to `internal/domain/va.go`, `internal/usecase/va_usecase.go`, and their direct test/consumer files (see Phase 0 research for the full touch-point list)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Principle I (Clean Architecture)**: PASS — changes stay within existing domain/usecase layers; no new cross-layer dependency introduced.
- **Principle III (TDD)**: PASS, actionable — existing tests in `va_usecase_test.go` currently assert the *wrong* behavior (require `TransactionDate`/`TotalAmount`); this plan requires updating those tests to assert the corrected spec behavior *before* changing the production code, per TDD.
- **Principle II (Configuration-Driven Integrations)**: N/A — this is a field-mapping/validation bug fix, not a new provider integration.
- **Principle XI (>90% Coverage)**: PASS — no net reduction in coverage; new `amount` field and revised validation branches must be covered by updated/new test cases.
- No other principles are implicated (no Docker, secrets, observability, idempotency, or async-queue changes).
- **Result**: No violations. No Complexity Tracking entries needed.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── va.go                 # VAInquiryRequest, VAPaymentRequest field fixes
│   └── va_test.go            # existing struct-level tests to update
├── usecase/
│   ├── va_usecase.go         # Inquiry/Payment validation logic fixes
│   └── va_usecase_test.go    # existing tests asserting old (wrong) behavior
├── infrastructure/
│   └── database/
│       └── va_repository.go  # VAInquiryRecord/VAPaymentRecord consistency
└── adapter/
    └── delivery/http/handler/
        ├── va_handler_test.go
        └── merchant_va_handler_test.go

aspi-open-api-va.yaml          # source-of-truth spec (read-only reference)
```

**Structure Decision**: Single Go backend service following existing Clean Architecture layout. No new packages/files — this is a targeted fix confined to the domain and usecase layers already used by the SNAP VA feature, with test updates in their co-located `_test.go` files.

## Complexity Tracking

> No Constitution Check violations — this section is not applicable.
