# Technical Research & Architectural Decisions: SNAP Token Management

**Feature Branch**: `001-snap-token-management`
**Date**: 2026-07-22

---

## 1. SNAP Asymmetric Signature Verification (SHA256withRSA)

### Decision
Use standard Go library packages (`crypto/rsa`, `crypto/x509`, `crypto/sha256`, `encoding/pem`) to verify incoming SNAP `X-SIGNATURE` headers.

### Rationale
- Zero third-party crypto dependency reduces security surface area.
- Go standard library RSA implementation is heavily audited and highly performant.
- SNAP asymmetric signature verification string format:
  ```text
  stringToSign = X-CLIENT-KEY + "|" + X-TIMESTAMP
  ```
- Verification process:
  1. Base64-decode the `X-SIGNATURE` header.
  2. Compute SHA-256 hash of `stringToSign`.
  3. Verify signature against stored client RSA public key (PEM decoded) using `rsa.VerifyPKCS1v15`.

### Alternatives Considered
- *Third-party JWT/JWS verification packages*: Rejected to avoid unneeded abstractions and potential incompatibilities with SNAP custom string formatting.

---

## 2. Ingress Idempotency Engine & Distributed Locking

### Decision
Implement custom Echo middleware backed by Redis using atomic `SETNX` and SHA-256 request payload hashing.

### Rationale
- Guaranteed single-execution semantics for concurrent token requests.
- Redis Key design:
  - In-flight lock: `idempotency:lock:{idempotency_key}` (TTL 30s)
  - Cached response: `idempotency:val:{idempotency_key}` (TTL 24 hours)
- Algorithm:
  1. If `Idempotency-Key` header missing on mutating routes -> Return `400 Bad Request`.
  2. Compute SHA-256 of request body payload.
  3. Check Redis for completed response (`idempotency:val:{key}`).
     - If exists & payload hash matches -> Return cached HTTP response immediately (replay).
     - If exists & payload hash differs -> Return `422 Unprocessable Entity` (payload mismatch).
  4. Acquire lock (`SETNX idempotency:lock:{key}`).
     - If lock fails -> Return `409 Conflict` (in-flight request processing).
  5. Process request -> Save response payload & status code in Redis (`idempotency:val:{key}`) -> Release lock.

---

## 3. Database & Caching Architecture

### Decision
- **PostgreSQL**: Primary transactional persistence layer using `github.com/jackc/pgx/v5/pgxpool`.
- **Redis**: High-speed cache for client public keys, active JWT sessions, distributed locks, and Asynq broker.

### Rationale
- Clean Architecture separation: `domain` defines `ClientRepository` and `TokenSessionRepository` interfaces; `infrastructure/database/postgres` implements pgx SQL queries; `infrastructure/redis` implements Redis caching.
- PostgreSQL handles long-term storage of client credentials and audit logs with strict relational integrity.
- Redis reduces DB read load to < 1ms for public key lookups during high-volume SNAP token requests.

---

## 4. Observability & OpenTelemetry Instrumentation

### Decision
Use `go.opentelemetry.io/otel` with OTLP gRPC exporter forwarding to **Grafana Alloy** collector.

### Rationale
- Native OpenTelemetry instrumentation allows trace spans to be created at:
  - Echo HTTP middleware level (`otelhttp` / custom Echo middleware)
  - Database queries (`pgx` OTel tracer wrapper)
  - Redis calls (`extrarediscache` / OTel hooks)
- All log entries emitted via `slog` include `trace_id` and `span_id` automatically, linking Loki logs to Tempo traces in Grafana dashboards.

---

## 5. Docker Containerization & Security

### Decision
Multi-stage Dockerfile targeting `alpine:3.20` running as non-root user `appuser` (UID 10001).

### Rationale
- **Build Stage**: `golang:1.23-alpine` with build cache mounts:
  ```dockerfile
  RUN --mount=type=cache,target=/go/pkg/mod \
      --mount=type=cache,target=/root/.cache/go-build \
      go build -ldflags="-s -w" -o /app/server ./cmd/api
  ```
- **Runtime Stage**: Minimizes image footprint (< 25MB) and executes under unprivileged `appuser`.
