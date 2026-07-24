# Implementation Plan: Swagger API Documentation & Testing

**Branch**: `005-swagger-api-documentation` | **Date**: 2026-07-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/005-swagger-api-documentation/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add generated, interactive OpenAPI (Swagger) documentation for all 13 currently registered HTTP endpoints (health, SNAP B2B token, admin client/key management, signature utilities, transfer-VA, merchant VA dashboard), sourced from `swaggo/swag` doc-comment annotations on each Echo handler and served via `echo-swagger` at `/swagger/*`. Each endpoint gets a plain-language description, full request/response schemas with examples, per-condition error documentation matched to actual handler error branches, and "try it out" execution against the running service, with auth-tier guidance (none / SNAP signature headers / admin token) per endpoint.

## Technical Context

**Language/Version**: Go 1.26.5 (per `go.mod`)

**Primary Dependencies**: Echo v4 (existing), `github.com/swaggo/swag` (new, doc generation CLI/lib), `github.com/swaggo/echo-swagger` (new, Swagger UI middleware for Echo)

**Storage**: N/A — this feature adds no persisted data; it documents existing DTOs

**Testing**: Standard Go `testing` package + `testify` (existing project convention); doc generation validated via the quickstart.md manual/scripted checks, not unit tests (no new business logic is introduced)

**Target Platform**: Linux server (existing deployment target, Docker container per Constitution Principle V)

**Project Type**: Web service (single Go module, Clean Architecture layers per constitution)

**Performance Goals**: N/A — documentation UI is not on any latency-critical request path; no impact to existing endpoint performance

**Constraints**: Annotations MUST live only in the Adapter/Delivery layer (handler files) per Constitution Principle I — no Domain layer changes; generated `docs` package MUST NOT be hand-edited; Swagger route registration follows the same non-env-gated pattern as other routes in `main.go`

**Scale/Scope**: 13 endpoints across 6 tag groups; no new runtime dependencies beyond two doc-generation libraries; zero change to existing request/response behavior

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Principle I (Clean Architecture)**: PASS. Swagger annotations are Go comments attached to existing Adapter/Delivery-layer handler functions; no Domain or Usecase layer code is touched. The generated `docs` package is itself an adapter-layer concern (imported only from `cmd/api/main.go`).
- **Principle II (Configuration-Driven Integrations)**: N/A — this feature does not touch provider integration.
- **Principle III (TDD)**: PASS with adaptation — this feature is documentation, not business logic; there is no new behavior to unit-test. Validation instead follows quickstart.md's runnable scenarios (doc accuracy checked against live responses), which is the appropriate testability standard for a docs feature. No existing test coverage is reduced.
- **Principle IV (Context propagation)**: N/A — no new I/O-performing functions are added; Swagger UI serves static/generated content through Echo's existing request handling.
- **Principles V–VII (Docker, non-root, secrets)**: PASS — no new runtime secrets; the two added Go dependencies compile into the existing static binary; no Dockerfile changes required beyond ensuring `docs/` is included in the build context (build-stage only, not needed at runtime as a separate mount).
- **Principle VIII (Observability)**: N/A — Swagger UI/spec endpoints are simple GETs; existing Echo OTel middleware (if globally applied) will cover them automatically with no extra work.
- **Principle IX (Async/Asynq/Redis)**: N/A — no queues or async workflows involved.
- **Principle X (Idempotency)**: N/A — Swagger UI/doc.json are read-only GET endpoints, not mutating requests; no `Idempotency-Key` requirement applies.
- **Principle XI (90% coverage)**: PASS — no new business-logic packages are introduced (`docs` is generated, typically excluded from coverage like other generated code, consistent with the constitution's stated exclusion for generated code); existing package coverage is unaffected since handler behavior doesn't change, only comments are added above functions.

**Result**: No violations. Complexity Tracking section left empty.

## Project Structure

### Documentation (this feature)

```text
specs/005-swagger-api-documentation/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── swagger-ui.md
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── api/
    └── main.go                          # add general API info annotations + mount echo-swagger route + blank-import docs

docs/                                     # generated by `swag init` — not hand-edited
├── docs.go
├── swagger.json
└── swagger.yaml

internal/
└── adapter/
    └── delivery/
        └── http/
            └── handler/
                ├── token_handler.go      # add @Summary/@Description/@Param/@Success/@Failure/@Router annotations
                ├── client_handler.go     # (same)
                ├── signature_handler.go  # (same)
                ├── va_handler.go         # (same)
                └── merchant_va_handler.go# (same)
```

**Structure Decision**: Single Go web-service project (existing layout, no new top-level modules). Annotations are added in-place above existing handler methods in `internal/adapter/delivery/http/handler/`; a new generated `docs/` directory is produced by `swag init` and wired into `cmd/api/main.go` via a blank import plus the `echo-swagger` route registration alongside existing routes. This matches Constitution Principle I's Adapter/Delivery layer boundary — no Domain or Usecase code changes.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
