# Backbone Payment Gateway

A SNAP (Standardized National API for Payment) compliant B2B payment gateway built with Go. Implements secure client authentication via RSA signatures, JWT-based access token generation, and idempotent request handling.

## Tech Stack

- **Language:** Go 1.26.5
- **HTTP Framework:** Echo v4
- **Database:** PostgreSQL 16
- **Cache:** Redis 7
- **Observability:** OpenTelemetry (tracing)
- **Containerization:** Docker (multi-stage build, non-root)

## Project Structure

```
backbone-new/
├── cmd/api/              # Application entry point
├── internal/
│   ├── domain/           # Domain models and interfaces
│   ├── usecase/          # Business logic
│   ├── adapter/
│   │   ├── delivery/     # HTTP handlers and middleware
│   │   └── gateway/      # External service adapters
│   └── infrastructure/   # Database, Redis, crypto, telemetry
├── db/migrations/        # PostgreSQL migrations
├── scripts/              # Validation and utility scripts
├── Dockerfile            # Multi-stage production build
└── docker-compose.yml    # Full stack orchestration
```

## Prerequisites

- Go 1.26.5+
- Docker & Docker Compose
- PostgreSQL 16 (for local dev without Docker)
- Redis 7 (for local dev without Docker)

## Quick Start

### Docker (Recommended)

Start the full stack with Docker Compose:

```bash
docker compose up --build
```

This starts:
- Application on `http://localhost:8080`
- PostgreSQL on `localhost:5432`
- Redis on `localhost:6379`

### Local Development

1. Start infrastructure services:

```bash
docker compose up postgres redis -d
```

2. Run the application:

```bash
go run ./cmd/api
```

The server starts on port `8080` by default.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL user |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `payment_gateway` | Database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | *(empty)* | Redis password |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(empty)* | OpenTelemetry collector endpoint |

## API Endpoints

### Health Check

```
GET /health
```

Response:
```json
{
  "status": "UP",
  "service": "payment-integration-gateway"
}
```

### SNAP B2B Access Token

```
POST /v1.0/access-token/b2b
```

**Required Headers:**

| Header | Description |
|--------|-------------|
| `X-CLIENT-KEY` | Client identifier |
| `X-TIMESTAMP` | Request timestamp |
| `X-SIGNATURE` | RSA signature (SHA256withRSA) of the payload |
| `Idempotency-Key` | Unique request identifier for idempotency |

**Request Body:**

```json
{
  "grantType": "client_credentials"
}
```

**Success Response (200):**

```json
{
  "responseCode": "2007300",
  "responseMessage": "Success",
  "accessToken": "eyJhbGciOiJSUzI1NiIs...",
  "tokenType": "bearer",
  "expiresIn": "86400"
}
```

**Error Responses:**

| Code | Status | Description |
|------|--------|-------------|
| `4007300` | 400 | Bad request (missing headers, invalid payload) |
| `4017300` | 401 | Unauthorized (invalid signature, client not found/revoked) |
| `4097300` | 409 | Conflict (duplicate idempotency key in progress) |
| `4227300` | 422 | Payload mismatch for idempotency key |
| `5007300` | 500 | Internal server error |

## Running Tests

```bash
# Run all tests with race detector
go test -race -v ./...

# Run tests for specific packages
go test -race -v ./internal/usecase/...
go test -race -v ./internal/infrastructure/crypto/...
go test -race -v ./internal/adapter/delivery/http/handler/...
```

## Linting

```bash
golangci-lint run
```

## Docker Build

Build the production image:

```bash
docker build -t backbone-payment-gateway:latest .
```

Run validation script (tests + Docker build):

```bash
./scripts/validate-snap-token.sh
```

## Architecture

The project follows clean architecture principles:

- **Domain** (`internal/domain/`): Core business models and repository interfaces
- **UseCase** (`internal/usecase/`): Application-specific business logic
- **Adapter** (`internal/adapter/`): External interfaces (HTTP handlers, middleware)
- **Infrastructure** (`internal/infrastructure/`): External service implementations (PostgreSQL, Redis, crypto)

Key components:

- **RSA Signature Verification**: Validates client authentication using SHA256withRSA
- **JWT Issuance**: Generates time-limited access tokens signed with server RSA keys
- **Idempotency Middleware**: Prevents duplicate request processing using Redis-backed distributed locks
- **Telemetry Middleware**: Correlates requests with OpenTelemetry traces and structured logging

## Database Migrations

Migrations are located in `db/migrations/` and are applied automatically on startup:

- `000001_create_client_apps`: Client application registry
- `000002_create_client_keys`: Client RSA public keys
- `000003_create_va_transactions`: Virtual Account transactions

## Vendor Integration (Virtual Account)

The system supports configurable vendor integrations via `.env.<vendor>.<channel>` files.

### Adding a New Vendor

1. Copy `.env.vendor.channel.example` to `.env.<vendor>.<channel>`:

```bash
cp .env.vendor.channel.example .env.bca.va
```

2. Update the configuration:

```bash
VENDOR_CLIENT_ID=your_client_id
VENDOR_CLIENT_SECRET=your_client_secret
VENDOR_BASE_URL=https://sandbox.bca.co.id
VENDOR_CHANNEL_ID=95231
VENDOR_PARTNER_ID=your_partner_id
```

3. Restart the server - routes are auto-registered at `/v1.0/<vendor>/<channel>/`

### Vendor API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1.0/<vendor>/<channel>/transfer-va/inquiry` | VA Bill Inquiry |
| POST | `/v1.0/<vendor>/<channel>/transfer-va/payment` | VA Payment Notification |
| POST | `/v1.0/<vendor>/<channel>/transfer-va/status` | VA Payment Status |

### Configuration Reference

See `.env.vendor.channel.example` for all available configuration options:

- Authentication (client ID, secret, keys)
- API endpoints (inquiry, status, payment)
- Channel configuration (channel ID, partner ID)
- Request settings (timeout, signature algorithm)
- SNAP headers (required headers list)
- Logging options

## License

Private project.
