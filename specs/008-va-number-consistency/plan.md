# Implementation Plan: Virtual Account Number Consistency with SNAP Standard

**Branch**: `008-va-number-consistency` | **Date**: 2026-07-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/008-va-number-consistency/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Enforce the SNAP-standard invariant `virtualAccountNo = partnerServiceId + customerNo` at VA creation time in `MerchantVAUsecase.CreateVA` (`internal/usecase/merchant_va_usecase.go`): for static VA (customerNo merchant-supplied), validate the merchant-supplied `virtualAccountNo` against the concatenation and reject on mismatch; for dynamic VA (customerNo server-generated), make `virtualAccountNo` optional — honor it if the merchant supplies it (subject to the existing pending-conflict check), otherwise derive it from `partnerServiceId` + the newly generated `customerNo`. No new dependencies, no schema changes — this is a validation/derivation change confined to the existing use case layer.

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — existing module `backbone-new`

**Primary Dependencies**: None new. Reuses existing `domain.VARepository` (`GetVAByVirtualAccountNo`, `NextCustomerNoSequence`, `RegisterStaticCustomerNo`) and `domain.VATypeRuleProvider` already wired into `MerchantVAUsecase`.

**Storage**: PostgreSQL (existing `VAInquiryRecord`/VA transaction tables) — no migration required; this feature only changes what values are computed/validated before the existing `SaveInquiry` call, not the schema.

**Testing**: Go `testing` + existing usecase unit tests (`internal/usecase/merchant_va_usecase_test.go` pattern) covering static mismatch rejection, dynamic auto-derivation, dynamic merchant-supplied value, and conflict cases; per constitution Principle III (TDD) and XI (>90% coverage).

**Target Platform**: Linux server (existing Docker Compose stack: app, postgres, redis)

**Project Type**: Web service (single Go backend, Clean Architecture layers per constitution)

**Performance Goals**: No new performance requirements — the added logic is O(1) string concatenation/comparison plus at most one additional repository read already performed today (`GetVAByVirtualAccountNo`); negligible added latency to the existing create-VA request path.

**Constraints**: `virtualAccountNo` MUST remain ≤28 characters (existing ASPI VAIdentity constraint, `merchant_va_usecase.go:101-104`); `partnerServiceId` + `customerNo` concatenation MUST NOT be truncated — if it would exceed 28 chars, VA creation is rejected (FR-007).

**Scale/Scope**: Confined to the single `CreateVA` method and its direct helpers in `internal/usecase/merchant_va_usecase.go`; no changes to inquiry/payment/status use cases, HTTP handlers, or DTOs beyond error-code additions.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Clean Architecture**: PASS — all changes stay within `internal/usecase/merchant_va_usecase.go` (application layer), operating only on existing `domain` types and repository interfaces. No new framework/driver dependency introduced.
- **II. Configuration-Driven Integrations**: N/A — this feature does not touch provider/vendor adapter configuration; it governs the merchant-facing create-VA contract only.
- **III. TDD**: PASS (planned) — new unit tests will be written first for each acceptance scenario (static mismatch, dynamic auto-derive, dynamic merchant-supplied, conflict) before implementation changes land.
- **IV. Context-Aware Operations**: PASS — no new I/O call sites introduced beyond the existing `ctx`-threaded repository calls already in `CreateVA`.
- **VII. Zero Secrets Policy**: N/A — no secrets involved.
- **X. Idempotency**: PASS — unaffected; this feature does not change idempotency-key handling, only VA-number validation/derivation prior to persistence.
- **XI. Coverage > 90%**: PASS (planned) — new branches (mismatch rejection, derivation, conflict) will be covered by new unit tests per Phase 2 tasks.

No violations requiring Complexity Tracking justification.

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
│   └── va.go                              # VAInquiryRecord, VARepository, VATypeRule — no changes to schema, may add a helper for max-length concat check
├── usecase/
│   ├── merchant_va_usecase.go             # CreateVA: static validation + dynamic derivation logic changes here
│   └── merchant_va_usecase_test.go        # New/updated unit tests (TDD, written first)
└── adapter/
    └── delivery/http/handler/             # No changes expected — request/response DTOs and error-code mapping already generic (domain.DomainError -> HTTP response)

specs/008-va-number-consistency/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (create-va request/response contract deltas)
└── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

**Structure Decision**: Single Go backend project (existing Clean Architecture layout per constitution). This feature is a targeted correction confined to the `usecase` layer's `CreateVA` method — no new packages, directories, or services are introduced. Existing `domain.VARepository` methods (`GetVAByVirtualAccountNo`, `NextCustomerNoSequence`, `RegisterStaticCustomerNo`) are reused as-is.

## Complexity Tracking

*No Constitution Check violations — this section is not applicable.*
