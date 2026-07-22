# Implementation Plan: BCA Virtual Account Integration

**Branch**: `002-bca-virtual-account` | **Date**: 2026-07-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-bca-virtual-account/spec.md`

## Summary

Integrate with BCA's SNAP Virtual Account API for biller transactions. The system will receive inbound VA inquiry, payment notification, and status inquiry requests from BCA, process them through a clean architecture pipeline, and respond with appropriate SNAP-standard responses. Configuration is stored in `.env.bca.va` files for zero-code modification per constitution Principle II.

## Technical Context

**Language/Version**: Go 1.26.5

**Primary Dependencies**: Echo v4 (HTTP), pgx/v5 (PostgreSQL), go-redis/v9 (Redis), golang-jwt/v5 (JWT), OpenTelemetry SDK

**Storage**: PostgreSQL (transaction records, VA master data), Redis (idempotency, caching)

**Testing**: Go `testing` + testify/assert + testify/mock

**Target Platform**: Linux server (Docker)

**Project Type**: Web service (SNAP Payment Gateway)

**Performance Goals**: VA inquiry < 5s, payment notification < 10s, 100 concurrent requests

**Constraints**: 30s BCA timeout window, 90% test coverage, SNAP response code compliance

**Scale/Scope**: Single biller integration, multi-bills support, IDR/USD/SGD currencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clean Architecture | PASS | New domain entities, usecases, handlers, and gateway adapter follow existing patterns |
| II. Configuration-Driven | PASS | `.env.bca.va` file with all BCA config; zero code changes for provider updates |
| III. TDD & Testability | PASS | All dependencies (BCA client, repository, cache) are interface-based and mockable |
| IV. Context Propagation | PASS | All new functions accept `ctx context.Context` as first parameter |
| V. Multi-Stage Docker | PASS | No Dockerfile changes needed; existing build handles new packages |
| VI. Non-Root Container | PASS | No container changes needed |
| VII. Credential Store | PASS | BCA credentials loaded from `.env.bca.va`; secrets not hardcoded |
| VIII. OpenTelemetry | PASS | Existing telemetry middleware covers new endpoints automatically |
| IX. Async Processing | PASS | Payment notifications can be async via Asynq if needed |
| X. Idempotency | PASS | X-EXTERNAL-ID used as idempotency key for BCA requests |
| XI. 90% Coverage | PASS | All new packages will have comprehensive tests |

**Gate Result**: ALL PASS - proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/002-bca-virtual-account/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── bca-va-api.yaml  # OpenAPI contract
└── tasks.md             # Phase 2 output (speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── client.go           # existing
│   ├── errors.go           # existing
│   ├── token.go            # existing
│   └── va.go               # NEW: VA domain entities and interfaces
├── usecase/
│   ├── client_usecase.go   # existing
│   ├── token_usecase.go    # existing
│   └── va_usecase.go       # NEW: VA business logic
├── adapter/
│   ├── delivery/
│   │   └── http/
│   │       ├── handler/
│   │       │   ├── token_handler.go    # existing
│   │       │   └── va_handler.go       # NEW: VA HTTP handlers
│   │       └── middleware/
│   │           ├── idempotency.go      # existing
│   │           └── bca_auth.go         # NEW: BCA signature validation
│   └── gateway/
│       └── bca/
│           └── va_client.go            # NEW: BCA VA API client
└── infrastructure/
    ├── config/
    │   └── bca_va_config.go            # NEW: .env.bca.va loader
    ├── crypto/
    │   ├── jwt_issuer.go    # existing
    │   └── hmac.go          # NEW: HMAC-SHA256 for BCA signatures
    ├── database/
    │   ├── postgres.go      # existing
    │   └── va_repository.go # NEW: VA transaction persistence
    └── redis/
        ├── redis.go         # existing
        └── bca_cache.go     # NEW: BCA token/response caching
```

**Structure Decision**: Follows existing Clean Architecture layout. New VA domain entities in `domain/va.go`, business logic in `usecase/va_usecase.go`, HTTP handlers in `adapter/delivery/http/handler/va_handler.go`, BCA API client in `adapter/gateway/bca/va_client.go`, and infrastructure support in `infrastructure/`.

## Complexity Tracking

No constitution violations. All design decisions align with existing patterns.
