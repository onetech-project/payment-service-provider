<!--
Sync Impact Report:
- Version change: 1.2.0 → 1.3.0
- List of modified principles:
  - Added XI: Mandatory Test Coverage > 90%
- Added sections:
  - Added Coverage Gate to Quality Gates
  - Added coverage configuration guidance
- Removed sections: None
- Templates requiring updates:
  - ✅ .specify/templates/plan-template.md (Checked & aligned)
  - ✅ .specify/templates/spec-template.md (Checked & aligned)
  - ✅ .specify/templates/tasks-template.md (Checked & aligned)
- Follow-up TODOs: None
-->

# Payment Integration Gateway Constitution

## Core Principles

### I. Clean Architecture & Composition over Inheritance
- System architecture MUST strictly enforce Clean Architecture separation of concerns:
  - **Domain Layer**: Pure business entities, value objects, domain errors, and repository/gateway interface definitions. MUST NOT depend on any third-party framework, database driver, ORM, or web library (including Echo, GORM, Redis, or Asynq).
  - **Use Case / Application Layer**: Business logic orchestration and background task handlers. Depends solely on Domain interfaces.
  - **Adapter / Delivery Layer**: Echo HTTP handlers, Asynq task processors, request validation, response serialization, and external payment gateway/bank adapters.
  - **Infrastructure Layer**: PostgreSQL repositories, Redis clients, configuration parsing, credential store integration, database migrations, and telemetry exporters.
- Dependencies MUST be explicit and injected via constructors using Dependency Injection (DI). Global state, package-level variables, and singletons are strictly forbidden.
- Struct composition MUST be preferred over pseudo-inheritance patterns. Interfaces MUST be defined by the consumer, keeping signatures minimal and single-purpose (SOLID / Interface Segregation).
- Adhere to **KISS** (Keep It Simple, Stupid), **DRY** (Don't Repeat Yourself), and **YAGNI** (You Aren't Gonna Need It). Maintainability, readability, and code simplicity MUST take precedence over speculative abstractions.

### II. Configuration-Driven External Integrations (Zero-Code Modification)
- The primary goal of this system is to serve as an internal gateway connecting to diverse payment gateways, banks, and external financial systems.
- Integrating a new external payment provider or modifying existing provider parameters MUST be achievable purely through dynamic configuration without modifying application source code.
- Provider integration MUST follow a pluggable, dynamic Adapter/Strategy pattern where request mapping, response parsing, routing, and header/credential lookups are resolved from configuration metadata.

### III. Test-Driven Development & Mandatory Testability (TDD)
- Test-Driven Development (TDD) is MANDATORY for all feature implementations and bug fixes:
  1. Write failing unit/integration test first.
  2. Implement minimal code to pass the test.
  3. Refactor while maintaining green test status.
- Code MUST be architected for high testability. All external dependencies (PostgreSQL repositories, Redis caches, Asynq queues, payment providers, credential stores) MUST be mockable via Go interfaces.
- Test suites MUST cover happy paths, edge cases, database transaction rollbacks, queue retry failures, timeout handling, and external gateway error responses.

### IV. Context-Aware Operations & Timeout Propagation
- Every function performing I/O, database queries (PostgreSQL), cache operations (Redis), queue enqueuing (Asynq), HTTP handling (Echo), or external payment API calls MUST accept `ctx context.Context` as its first parameter.
- Context cancellation signals, deadlines, OpenTelemetry trace spans, and tracing baggage MUST be propagated downstream without truncation across both synchronous HTTP calls and asynchronous Asynq tasks.
- External payment calls and database transactions MUST enforce explicit timeouts derived from context deadlines.

### V. Multi-Stage Docker Builds & Immutable Container Artifacts
- Container images MUST be built using multi-stage Dockerfiles:
  - **Build Stage**: Compiles Go source code with build cache mounts (e.g. `go mod download` layer caching) to optimize pipeline execution speed.
  - **Runtime Stage**: Contains only the compiled static binary and essential CA certificates.
- Production container artifacts MUST be completely immutable, minimal (e.g., `distroless` or `alpine`), and free of compiler toolchains, shell utilities, or unneeded dependencies.

### VI. Non-Root Container Execution & Container Security
- Containers MUST NEVER run under the `root` superuser.
- Dockerfiles MUST explicitly create and switch to a dedicated non-root user (e.g. `USER appuser`, `uid 10001`).
- The application MUST run with read-only root filesystems where possible and MUST NOT require elevated Linux capabilities.

### VII. External Credential Store & Zero Secrets Policy
- Hardcoding secrets, API keys, merchant tokens, private keys, database passwords, or Redis credentials in source code, configuration files, environment files, or Docker images is STRICTLY FORBIDDEN.
- Secrets MUST be retrieved dynamically at startup or runtime from an external enterprise Credential Store / Key Vault (e.g. HashiCorp Vault, AWS Secrets Manager, or Kubernetes Secret volumes).
- Secret values MUST be masked in application logs, traces, and Asynqmon dashboard views.

### VIII. Unified OpenTelemetry Observability (Alloy, Loki, Prometheus, Tempo, Grafana)
- Observability MUST be natively integrated into all components using the **OpenTelemetry (OTel)** Go SDK:
  - **Traces (Tempo)**: Every incoming HTTP request (Echo), database query (PostgreSQL), background job (Asynq), and external gateway call MUST create distributed trace spans. Context MUST be propagated across network boundaries.
  - **Metrics (Prometheus)**: System metrics (HTTP request counts/latency, Asynq queue latency/throughput, database connection pool, Redis cache hit/miss rates) MUST be exported in Prometheus format via OpenTelemetry collectors.
  - **Logs (Loki)**: All application logging MUST be structured JSON formatted and correlated with active OTel `trace_id` and `span_id`.
  - **Collector (Grafana Alloy)**: Telemetry signals (logs, metrics, traces) MUST be shipped via Grafana Alloy agent to Loki, Prometheus, and Tempo, visualized in unified Grafana dashboards.

### IX. Asynchronous Processing & State Management (PostgreSQL, Redis, Asynq, Asynqmon)
- **Persistence (PostgreSQL)**: PostgreSQL MUST be the single source of truth for transactional data (payment transactions, ledger entries, provider configurations). All schema alterations MUST be managed via versioned migration scripts.
- **Caching & Queue Engine (Redis & Asynq)**: Redis MUST be utilized for high-speed caching, distributed locks, and as the backing broker for **Asynq** background task queues.
- **Asynchronous Workflows (Asynq)**: Non-blocking long-running operations (payment webhooks, retry attempts, reconciliation processing, notifications) MUST be executed asynchronously using Asynq queues with explicit retry and dead-letter policies.
- **Queue Monitoring (Asynqmon)**: **Asynqmon** web dashboard MUST be deployed and exposed securely for real-time inspection, monitoring, and payload management of Asynq queues and tasks.

### X. Strict Idempotency Key Enforcement
- **Ingress Request Idempotency**: All state-mutating HTTP API requests (e.g. payment creation, refunds, transfers, payouts, cancellations) MUST require a valid `Idempotency-Key` header. Requests lacking a key on mutating endpoints MUST be rejected with `400 Bad Request`.
- **Atomicity & Locking**: The system MUST use Redis atomic locks (`SETNX` or distributed lock pattern) during in-flight processing to prevent concurrent execution of duplicate requests.
- **Payload Verification & Response Replay**:
  - If a request with a previously completed `Idempotency-Key` and identical request payload hash is received, the system MUST return the cached/persisted response payload with identical status code without re-executing business logic or external payment calls.
  - If the same `Idempotency-Key` is submitted with a modified payload/parameters, the system MUST reject the request with `422 Unprocessable Entity` or `400 Bad Request` payload mismatch error.
- **Egress Gateway Idempotency**: When invoking external payment gateways, banks, or financial providers, the system MUST generate or forward deterministic idempotency keys to guarantee zero double-charging or duplicate external transactions.

### XI. Mandatory Test Coverage > 90%
- **Coverage Threshold**: The project MUST maintain a minimum test coverage of **90%** across all packages. This threshold applies to combined line and branch coverage measured by Go's built-in coverage tooling.
- **Coverage Enforcement**: Every CI/CD pipeline run MUST verify coverage meets or exceeds 90%. Builds failing to meet this threshold MUST be rejected.
- **Coverage Scope**: Coverage calculation MUST include:
  - All packages under `internal/usecase/` (business logic)
  - All packages under `internal/adapter/` (HTTP handlers, middleware)
  - All packages under `internal/infrastructure/` (database, Redis, crypto, telemetry)
  - All packages under `internal/domain/` (domain logic where applicable)
- **Exclusions**: Generated code, `cmd/` entrypoints, and migration files MAY be excluded from coverage calculations with explicit justification documented in CI configuration.
- **Coverage Reporting**: Coverage reports MUST be generated on every test run and archived for trend analysis. Coverage deltas MUST be visible in pull request reviews.

## Technology & Architecture Constraints

### Tech Stack & Tooling
- **Language**: Go (latest stable version)
- **Web Framework**: Echo (HTTP delivery layer only)
- **Primary Database**: PostgreSQL (Relational storage with versioned SQL migrations)
- **Cache & Message Broker**: Redis (Idempotency storage, distributed locks, queue engine)
- **Background Task Processing**: Asynq & Asynqmon (Task queue engine and monitoring UI)
- **Telemetry & Observability**: OpenTelemetry Go SDK, Grafana Alloy (Collector), Grafana Loki (Logs), Prometheus (Metrics), Grafana Tempo (Traces), Grafana (Dashboards)
- **Containerization**: Docker (Multi-stage build, layer cache, non-root runner)
- **Secret Management**: External Credential Store / Vault API driver
- **Testing**: Standard Go `testing` package, testify/assert, mock generation
- **Coverage Tooling**: `go test -coverprofile`, `go tool cover` for threshold enforcement

### Directory Layout Convention
```text
internal/
├── domain/            # Entities, Value Objects, Domain Interfaces (No framework/driver dependencies)
├── usecase/           # Business Logic & Workflow Orchestration
├── adapter/
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── middleware/ # Idempotency, Correlation ID, OTel Middleware
│   │   │   └── handler/    # Echo Handlers, DTOs, Routing
│   │   └── worker/         # Asynq Task Processors & Queue Consumers
│   └── gateway/            # Dynamic Payment Gateway / Bank Adapters (With Egress Idempotency)
└── infrastructure/
    ├── database/      # PostgreSQL connection pool & migrations
    ├── redis/         # Redis client, distributed locks & idempotency store
    ├── queue/         # Asynq Client & Enqueuer wrappers
    ├── telemetry/     # OpenTelemetry setup (Tracer, Meter, Logger)
    ├── secret/        # Credential store client
    └── config/        # Dynamic configuration parser
```

## Development & Quality Assurance Workflow

1. **Pre-Implementation**: Define spec.md and plan.md following this constitution.
2. **TDD Execution**: Write unit/integration tests (verifying PostgreSQL queries, Redis caching, Asynq worker handlers, Idempotency lock & replay behavior, OTel trace creation, and gateway config mapping) -> Verify test fails -> Implement code -> Verify test passes -> Refactor.
3. **Quality Gates**:
   - `go test -race -v ./...` MUST pass with zero errors.
   - `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` MUST report >= 90% coverage.
   - `golangci-lint run` MUST produce zero warnings.
   - Docker build MUST succeed using multi-stage non-root container target.
4. **Integration Verification**:
   - Verify zero-code modification when updating payment provider configuration.
   - Verify concurrent duplicate requests with the same `Idempotency-Key` are safely locked and replayed cleanly without duplicate external payment execution.
   - Verify full trace continuity across Echo -> Idempotency Middleware -> PostgreSQL -> Asynq -> External Gateway.

## Governance

- This Constitution supersedes all informal team development practices and architectural guidelines.
- Any proposed architectural change, new dependency, or principle modification requires an explicit amendment proposal.
- **Version bump rules**:
  - **MAJOR**: Changing foundational architecture or breaking governance principles.
  - **MINOR**: Adding new core principles, infrastructure dependencies, or telemetry/security standards.
  - **PATCH**: Clarifying existing guidance, fixing typos, or updating documentation references.

**Version**: 1.3.0 | **Ratified**: 2026-07-22 | **Last Amended**: 2026-07-22
