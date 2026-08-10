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
POST /openapi/v1.0/access-token/b2b
```

**Required Headers:**

These are the four headers BCA documents for the access-token endpoint
(BCA OpenAPI OAuth & Signature v1.1). No other header is sent or read —
notably there is no `Idempotency-Key`: idempotency on the transaction
endpoints is keyed on the ASPI-standard `X-EXTERNAL-ID`.

| Header | Mandatory | Description |
|--------|-----------|-------------|
| `Content-Type` | Y | `application/json` |
| `X-CLIENT-KEY` | Y | Client identifier (`client_id`) |
| `X-TIMESTAMP` | Y | Request timestamp, ISO 8601 with timezone |
| `X-SIGNATURE` | Y | Asymmetric signature, `SHA256withRSA(privateKey, clientId + "\|" + timestamp)`, base64 |

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
  "responseMessage": "Successful",
  "accessToken": "eyJhbGciOiJSUzI1NiIs...",
  "tokenType": "Bearer",
  "expiresIn": "900"
}
```

**Error Responses**, per the Errors table in *BCA API — OAuth & Signature
OpenAPI v1.1*. That table gives the failing **field** its own case code rather
than one code for the whole endpoint, so the code alone tells you what to fix:

| Code | Status | Condition |
|------|--------|-----------|
| `4007301` | 400 | `Invalid field format [clientId/clientSecret/grantType]` — or `[X-TIMESTAMP]`, which shares this code. A timestamp that is stale (outside ±5 min) reports the same code as one that will not parse; v1.1 publishes no separate code for staleness |
| `4007302` | 400 | `Invalid mandatory field [X-CLIENT-KEY]` |
| `4017300` | 401 | `Unauthorized. [Signature]` / `[Unknown client]` / `[Connection not allowed]`. The bracketed reason is contract, not prose — callers match on it |
| `4097300` | 409 | Duplicate idempotency key still in flight |
| `5007300` | 500 | Internal server error |

> `4007300` is **not** emitted. v1.1 dropped it from the list, moving
> `[clientId/clientSecret/grantType]` onto `4007301` alongside `[X-TIMESTAMP]`
> — consistent with case code `01` meaning "invalid field format" throughout
> SNAP. Integrations that matched on `4007300` must be updated.

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

## Testing against UAT

`scripts/` holds one script per endpoint plus several end-to-end flows. Driving
them by hand means knowing which script to call, which of the two credential
files it wants (merchant and vendor secrets are different, and swapping them
fails as an invalid signature), and copying VA numbers between steps.

`scripts/qa.sh` is the single entry point over all of them:

```bash
cp scripts/.env.qa.example scripts/.env.qa   # base URL + credential paths
./scripts/qa.sh doctor                       # tools, credentials, connectivity
./scripts/qa.sh                              # interactive menu

./scripts/qa.sh create -a 250000.00          # create a VA and remember it
./scripts/qa.sh inquiry                      # no arguments — acts on that VA
./scripts/qa.sh pay
./scripts/qa.sh status
```

It picks the right credential file per command, carries the VA between steps,
and shells out to the same scripts a developer would run by hand — it does not
re-implement any request. `./scripts/qa.sh request inquiry` prints a signed,
copy-paste-ready request for Postman or the ASPI simulator instead of sending
it. Set `QA_LOG_DIR` to keep a plain-text log of every run.

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

Migrations are located in `db/migrations/` (paired `NNNNNN_description.up.sql`/`.down.sql` files) and are applied using [`migrate/migrate`](https://github.com/golang-migrate/migrate) via the `migrate` service in `docker-compose.yml`:

```bash
docker compose up -d migrate
```

- `000001_create_client_apps`: Client application registry
- `000002_create_client_keys`: Client RSA public keys
- `000003_create_va_transactions`: Virtual Account transactions + bill details
- `000004_add_va_fields`: SNAP-compliant columns on `va_transactions`/`va_bill_details` (customer name/email/phone, `trx_id`, `notification_url`, amount/label columns, etc.)
- `000005_add_va_bill_details_missing_fields`: `bill_code`/`bill_name`/`bill_short_name` on `va_bill_details` (required for bill-detail persistence to actually work)
- `000006_add_va_type_and_sequences`: VA type classification and sequence support
- `000007_create_va_payments`: Per-payment ledger for cumulative/variable-bill VAs
- `000008_create_master_va_type_and_partner_service_ids`: Master VA type + partner service ID lookup tables
- `000009_create_va_notification_deliveries`: Merchant callback delivery-attempt audit trail (auto and manual resends)

## Vendor Integration (Virtual Account)

The system supports configurable vendor integrations via `.env.<vendor>.<channel>` files. All ASPI SNAP Virtual Account endpoints are registered under a single unified path — `/openapi/v1.0/transfer-va/*` — regardless of which vendor config is active; there is no per-vendor path prefix.

Full field-level ASPI compliance details (request/response schemas, header requirements) are in [`aspi-open-api-va.yaml`](./aspi-open-api-va.yaml) and [`specs/004-snap-va-field-compliance/`](./specs/004-snap-va-field-compliance/).

### BCA specification baseline

Field limits, response codes and endpoint versions are transcribed from these
BCA technical documents. When they disagree with the older *Developer API BCA*
portal export, **these win** — they are the versioned, dated artefacts:

| Document | Version | Covers |
|---|---|---|
| BCA API — OAuth & Signature OpenAPI | v1.1 (Jan 2025) | `stringToSign`, RSA/HMAC signing, access-token error codes |
| Tech. Doc. OpenAPI VA-BillPresentment API | v2.4 (Sep 2025) | `/transfer-va/inquiry` (service 24) |
| Tech. Doc. OpenAPI VA-Payment-Flag API | v2.3 (Sep 2025) | `/transfer-va/payment` (service 25) |
| Tech. Doc. OpenAPI VA-Payment-Status API V2 | v1.0 | `/transfer-va/status` (service 26) |

Things worth knowing because they changed, or because they are easy to get wrong:

- **The status service is `/openapi/v2.0`**, while inquiry and payment are
  `/openapi/v1.0`. It also runs in the opposite direction — partner → BCA — so
  it is the one path we *call* rather than serve. See
  [Payment reconciliation](docs/guides/vendor-onboarding.md#payment-reconciliation-we-call-you).
- **`virtualAccountName` is capped at 30 characters** and `create-va` now
  refuses a longer one (`4002702`). The name is echoed on every inquiry for
  that VA, so a longer value stored at registration makes BCA fail the inquiry
  at the channel rather than at the call that caused it.
- **`freeTexts` entries are capped at 32 characters** each (up from 18), and
  the limit is enforced per entry, not just on the count.
- **`inquiryRequestId` is `String(30)`**, down from 128. It is not independent
  of `paymentRequestId`, which must equal it and is itself capped at 30.
- **`amount` is a real, optional field** on the inquiry payload. It is accepted
  and never required — the customer-entered amount belongs to the payment, not
  to the bill being presented.
- **`responseMessage` for `2002400`/`2002500` is `"Successful"`**; for
  `2002600` (status) it is `"Success"`. BCA's own wording differs per service
  and has changed twice, so each is spelled as its own current table has it.
- **401 bodies on transfer-va carry `"data": {}`** alongside `responseCode` and
  `responseMessage`, per the OAuth v1.1 `401xx00` sample. The access-token
  endpoint's own 401 stays two fields.
- **`customerNo`/`virtualAccountNo` are `18`/`26`** in every current table.
  Inbound we still accept `20`/`28` so VA numbers this system issued under the
  old sequence stay payable; issuance is `18`/`26`.

**Onboarding a real vendor or merchant?** See
[docs/guides/vendor-onboarding.md](docs/guides/vendor-onboarding.md) or
[docs/guides/merchant-onboarding.md](docs/guides/merchant-onboarding.md) for
the full authentication model each side must implement (request signing,
timestamp freshness, credential provisioning) — vendors and merchants use
completely independent auth mechanisms, so read the one that matches your role.

### Adding a New Vendor

1. Copy `.env.vendor.channel.example` to `.env.<vendor>.<channel>`:

```bash
cp .env.vendor.channel.example .env.bca.va
```

2. Update the configuration:

```bash
VENDOR_CLIENT_ID=your_client_id
VENDOR_CLIENT_SECRET=your_client_secret
# BCA hosts: https://devapi.klikbca.com (UAT), https://api.klikbca.com (prod)
VENDOR_BASE_URL=https://devapi.klikbca.com
VENDOR_CHANNEL_ID=95231
VENDOR_PARTNER_ID=your_partner_id
# Endpoint paths are also the RelativeUrl component of stringToSign, so a
# wrong version here fails as 401, not 404. STATUS is v2.0, the others v1.0.
VENDOR_ENDPOINT_INQUIRY=/openapi/v1.0/transfer-va/inquiry
VENDOR_ENDPOINT_PAYMENT=/openapi/v1.0/transfer-va/payment
VENDOR_ENDPOINT_STATUS=/openapi/v2.0/transfer-va/status
```

3. Restart the server — the config is picked up from `.env.<vendor>.<channel>` at startup.

### Vendor-Facing API Endpoints (Service Code 24-27, 31)

| Method | Path | Service Code | Description |
|--------|------|---------------|-------------|
| POST | `/openapi/v1.0/transfer-va/inquiry` | 24 | VA bill inquiry (vendor → PSP) |
| POST | `/openapi/v1.0/transfer-va/payment` | 25 | VA payment notification (vendor → PSP); triggers an async merchant callback |
| POST | `/openapi/v2.0/transfer-va/status` | 26 | VA payment status inquiry (vendor → PSP). **v2.0**, not v1.0 — BCA versions this service separately; v1.0 stays registered for vendors already pointed there |
| POST | `/openapi/v1.0/transfer-va/create-va` | 27 | Create a Virtual Account (merchant-facing) |
| POST | `/openapi/v1.0/transfer-va/list` | — | List/filter registered VA **numbers**, one entry per VA (merchant dashboard convenience API, not an ASPI endpoint) |
| POST | `/openapi/v1.0/transfer-va/list-transactions` | — | List/filter individual **payments**, one entry per transaction (merchant dashboard convenience API, not an ASPI endpoint) |
| DELETE | `/openapi/v1.0/transfer-va/delete-va` | 31 | Delete a VA — deactivates the registration for no-bill VAs, cancels the pending transaction otherwise (merchant-facing) |

Service Code 28-35 (`update-va`, `update-status`, `inquiry-va`, `inquiry-intrabank`, `payment-intrabank`, `notify-payment-intrabank`, `report`) are defined in the OpenAPI spec but **not yet implemented**.

**Required headers** for `/openapi/v1.0/transfer-va/*` per ASPI spec: `X-TIMESTAMP`, `X-SIGNATURE` (both required), plus `X-PARTNER-ID`/`X-EXTERNAL-ID` (required by the API contract) and optionally `CHANNEL-ID` (spec marks it `required: false`). **`X-CLIENT-KEY` is NOT used here** — it only applies to `POST /openapi/v1.0/access-token/b2b`.

**VA lifecycle — two shapes.** Which one applies is decided by the VA type's `billing` classification in `master_va_type`, not by a hardcoded list, so operator-added types inherit the matching behavior.

*No-bill VAs (`vaType` 01 static, 04 dynamic) — register once, pay many times.* These behave like an e-wallet top-up address. `create-va` writes only the **VA registration** (`va_accounts`: partnerServiceId, customerNo, virtualAccountNo, holder name/email/phone, callback URL) and creates **no transaction**. A transaction row is created per *payment*, so the same VA number accepts an unlimited number of payments for any amount. Calling `create-va` again for the same number is an **update** of the holder details, not a conflict — you only need to call it once. `delete-va` deactivates the registration (there is no pending transaction to cancel) and leaves settled payments readable. A registration with no `expiredDate` never expires.

*Bill-bearing VAs (`vaType` 02/03 static, 05/06 dynamic) — one bill, one transaction cycle.* `create-va` creates a pending transaction bound to `totalAmount`, and a `virtualAccountNo` is reusable across cycles: creating a new VA under a number is rejected (`409 4092700`) only while that number still has a *pending* (unpaid) transaction. Once a transaction is *paid* (`status "00"`) it is immutable: `delete-va` against it is rejected (`405 4053101`), and a second `payment` call against it — even with a brand-new `paymentRequestId` — is rejected (`409 4092500`) rather than silently overwriting the recorded amount/reference. Variable-bill types (02/05) accept multiple payments until the cumulative total reaches `totalAmount`. See `scripts/e2e-va-cancel-flow.sh` below for a runnable demonstration of these rules.

Because one no-bill VA can hold many payments, the merchant listing is split: `POST .../list` returns one entry per VA number (with `transactionCount` and `totalPaid`), and `POST .../list-transactions` returns one entry per payment.

### Configuration Reference

See `.env.vendor.channel.example` for all available configuration options:

- Authentication (client ID, secret, keys)
- API endpoints (inquiry, status, payment)
- Channel configuration (channel ID, partner ID)
- Request settings (timeout, signature algorithm)
- SNAP headers (required headers list — defaults to `X-TIMESTAMP,X-SIGNATURE`; add `X-PARTNER-ID`/`X-EXTERNAL-ID`/`CHANNEL-ID` only if your vendor contractually requires them beyond the ASPI defaults)
- Logging options

## Scripts

`scripts/` contains dev/test tooling for exercising the SNAP VA flows without a real vendor/merchant connection. Each script's header comment documents its own flags; the highlights:

Onboarding (see [docs/guides/](docs/guides/) for the full merchant/vendor auth model):

| Script | Purpose |
|--------|---------|
| `onboard-vendor.sh` | Generates a `.env.<vendor>.<channel>` config file with a fresh `VENDOR_CLIENT_SECRET` — no API call, no keypair (vendor auth is HMAC-only). |
| `onboard-merchant.sh` | Generates an RSA keypair, registers it + a fresh shared secret via the admin API, and writes a `.env.merchant.<name>` credentials file (test-tooling convenience — the server itself never reads it; its DB is the source of truth). |

VA operations:

| Script | Purpose |
|--------|---------|
| `curl-b2b-token.sh` | Get a B2B access token (RSA-signed) |
| `merchant-create-va.sh` | Create a VA (`-b`/`-d` optionally attach one bill detail) |
| `merchant-list-va.sh` / `merchant-delete-va.sh` | List registered VA numbers (one row per VA) / cancel (delete) a VA |
| `merchant-list-transactions.sh` | List individual payments (one row per transaction); `-v` filters to one VA |
| `vendor-inquiry-va.sh` | Simulate a vendor VA inquiry |
| `vendor-payment-va.sh` | Simulate a vendor payment notification (triggers the merchant callback) |
| `e2e-va-flow.sh` | Runs the happy-path flow in one command: create-va → inquiry → payment → **verifies the merchant callback actually arrives** (spins up a throwaway local HTTP listener and prints the received callback payload) |
| `e2e-va-cancel-flow.sh` | Runs the cancel/immutability flow in one command: create a VA, cancel it while pending (and confirm its number is reusable), then pay a second VA and **prove a paid transaction can no longer be cancelled or re-paid** (asserts the expected `405`/`409` rejections at each step) |

**Vendor and merchant are two independent identities/credentials — never
reuse one for the other.** The vendor-facing scripts (`vendor-inquiry-va.sh`,
`vendor-payment-va.sh`) sign with a `.env.<vendor>.<channel>` file's
`VENDOR_CLIENT_SECRET` (HMAC only, no accessToken). The merchant-facing
scripts (`merchant-create-va.sh`, `merchant-list-va.sh`,
`merchant-delete-va.sh`) take a `.env.merchant.<name>` file (from
`onboard-merchant.sh`) via `-f`, which supplies both an accessToken (fetched
automatically) and an HMAC signature. The `e2e-*.sh` flows accordingly take
**two** separate flags: `-m <.env.merchant.NAME>` and `-f <.env.vendor.channel>`.

```bash
# 1. Onboard a test vendor and merchant against a local instance:
./scripts/onboard-vendor.sh -n bca -c va
./scripts/onboard-merchant.sh -n mytest -K changeme -u http://localhost:8080

# 2. Happy path: create-va -> inquiry -> payment -> callback verification
./scripts/e2e-va-flow.sh -s 12345678 -c 0812345678 -n "Merchant Name" \
  -m .env.merchant.mytest -f .env.bca.va \
  -a 10000 -b INV-001 -d "Invoice Januari"

# 3. Cancel / paid-immutability checks: cancel-while-pending + reuse, then
#    prove a paid VA rejects both cancellation and a second payment attempt
./scripts/e2e-va-cancel-flow.sh -s 12345678 -c 0812345678 -n "Merchant Name" \
  -m .env.merchant.mytest -f .env.bca.va -a 10000
```

See [VA lifecycle](#vendor-facing-api-endpoints-service-code-24-27-31) above for the reuse/immutability rules `e2e-va-cancel-flow.sh` asserts at each step.

## License

Private project.
