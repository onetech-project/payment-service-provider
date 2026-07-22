# Implementation Plan: SNAP Token Management Service & Endpoint

**Branch**: `001-snap-token-management` | **Date**: 2026-07-22 | **Spec**: [spec.md](file:///home/faris/code/manjo/backbone-new/specs/001-snap-token-management/spec.md)

**Input**: Feature specification from `/specs/001-snap-token-management/spec.md`

## Summary

Build and pilot the SNAP-compliant B2B OAuth Access Token Management service (`POST /v1.0/access-token/b2b`). This service acts as the foundation of the Payment Gateway, validating asymmetric RSA-SHA256 signatures over `X-CLIENT-KEY` and `X-TIMESTAMP`, issuing signed JWT access tokens, enforcing strict `Idempotency-Key` locking/replay via Redis, persisting client & token session records in PostgreSQL, and emitting complete OpenTelemetry traces/logs to the Grafana stack.

## Technical Context

**Language/Version**: Go 1.23+

**Primary Dependencies**: `github.com/labstack/echo/v4`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/redis/go-redis/v9`, `github.com/hibiken/asynq`, `github.com/golang-jwt/jwt/v5`, `go.opentelemetry.io/otel`

**Storage**: PostgreSQL 16 (Relational persistence), Redis 7 (Caching, Idempotency store, Asynq broker)

**Testing**: Go standard `testing`, `github.com/stretchr/testify`, `go test -race ./...`

**Target Platform**: Docker container running on Linux (Alpine 3.20 base, non-root user `appuser`)

**Project Type**: Payment Gateway Microservice / Web Service (Clean Architecture)

**Performance Goals**: < 50ms p95 response time for `POST /v1.0/access-token/b2b`; sub-millisecond public key and token cache lookup in Redis.

**Constraints**: Bank Indonesia SNAP standard compliance; strict `Idempotency-Key` header enforcement; zero hardcoded secrets; full OTel trace propagation; non-root Docker runtime.

**Scale/Scope**: Gateway handling high-concurrency payment B2B authorization tokens.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Compliance Status | Rationale / Implementation |
|-----------|-------------------|----------------------------|
| **I. Clean Architecture** | ✅ PASS | Core `domain` is isolated without web/db/framework imports. Explicit DI constructors throughout. |
| **II. Zero-Code External Config** | ✅ PASS | Dynamic adapter interface structure for future payment gateway providers. |
| **III. TDD Discipline** | ✅ PASS | Unit/Integration tests created before implementation (Red-Green-Refactor). |
| **IV. Context-Aware Operations** | ✅ PASS | All DB, Redis, HTTP handlers accept and propagate `ctx context.Context` with OTel spans. |
| **V. Multi-Stage Docker Build** | ✅ PASS | Multi-stage Dockerfile with GOCACHE/GOMODCACHE mounts and minimal runtime image. |
| **VI. Non-Root Security** | ✅ PASS | Execution explicitly pinned to `USER appuser` (UID 10001). |
| **VII. Zero Secrets in Code** | ✅ PASS | All RSA private keys, DB passwords, and secrets fetched from Vault / environment stores. |
| **VIII. OpenTelemetry Observability** | ✅ PASS | OTel instrumentation sending traces/logs/metrics to Alloy -> Tempo/Loki/Prometheus. |
| **IX. Async & State Management** | ✅ PASS | PostgreSQL for persistent storage; Redis & Asynq for caching & async queue operations. |
| **X. Strict Idempotency Key** | ✅ PASS | Echo middleware checking `Idempotency-Key`, performing Redis `SETNX` locking & payload replay. |

## Project Structure

### Documentation (this feature)

```text
specs/001-snap-token-management/
├── spec.md              # Feature specification
├── plan.md              # Implementation plan (this file)
├── research.md          # Phase 0 technical research & decisions
├── data-model.md        # Phase 1 data models & database schema
├── quickstart.md        # Phase 1 validation & quickstart guide
├── contracts/           # Phase 1 OpenAPI API contracts
│   └── snap-token-b2b.json
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
cmd/
└── api/
    └── main.go                  # Application entry point & DI wire-up

internal/
├── domain/                      # Domain entities & interfaces (Zero external imports)
│   ├── client.go
│   ├── token.go
│   ├── idempotency.go
│   └── errors.go
├── usecase/                     # Application business logic orchestration
│   ├── token_usecase.go
│   └── client_usecase.go
├── adapter/
│   └── delivery/
│       └── http/
│           ├── handler/         # Echo Handlers & DTOs
│           │   └── token_handler.go
│           └── middleware/      # Echo Middleware
│               ├── idempotency.go
│               └── telemetry.go
└── infrastructure/
    ├── database/                # PostgreSQL pgx pool & repository impl
    │   ├── postgres.go
    │   └── client_repository.go
    ├── redis/                   # Redis client & cache repository impl
    │   ├── redis.go
    │   └── idempotency_store.go
    ├── crypto/                  # RSA signature verifier & JWT generator
    │   ├── rsa_verifier.go
    │   └── jwt_issuer.go
    └── telemetry/               # OpenTelemetry tracer & meter initialization
        └── otel.go

db/
└── migrations/                  # Versioned SQL migration files
    ├── 000001_create_client_apps.up.sql
    ├── 000001_create_client_apps.down.sql
    ├── 000002_create_client_keys.up.sql
    └── 000002_create_client_keys.down.sql

Dockerfile                       # Multi-stage non-root container definition
docker-compose.yml               # Local development stack (Postgres, Redis, Alloy)
```

## Complexity Tracking

*No constitution violations present.*
