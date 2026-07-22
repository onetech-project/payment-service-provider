# Quickstart & End-to-End Validation Guide: SNAP Token Management

**Feature Branch**: `001-snap-token-management`
**Date**: 2026-07-22

---

## 1. Prerequisites & Environment Setup

- Go 1.23+ installed locally
- Docker and Docker Compose
- PostgreSQL database (port 5432)
- Redis server (port 6379)

---

## 2. Environment Variables

Create `.env` file from sample:
```bash
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=payment_gateway
REDIS_ADDR=localhost:6379
JWT_PRIVATE_KEY_PATH=./certs/jwt_rsa_private.pem
JWT_PUBLIC_KEY_PATH=./certs/jwt_rsa_public.pem
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

---

## 3. Running Unit and Integration Tests

Run TDD test suite with race condition detection:
```bash
go test -race -v ./...
```

---

## 4. End-to-End Validation Scenarios

### Scenario 1: Generate Client RSA Keys & Client Registration
1. Generate test RSA Keypair:
   ```bash
   openssl genpkey -algorithm RSA -out client_private.pem -pkeyopt rsa_keygen_bits:2048
   openssl rsa -in client_private.pem -pubout -out client_public.pem
   ```
2. Insert test client in PostgreSQL:
   ```sql
   INSERT INTO client_apps (id, client_id, client_name, status)
   VALUES ('client-uuid-1', 'client-partner-001', 'Partner Alpha', 'ACTIVE');

   INSERT INTO client_keys (id, client_id, key_id, public_key_pem, is_active)
   VALUES ('key-uuid-1', 'client-partner-001', 'key-01', '<PEM_CONTENT_HERE>', true);
   ```

---

### Scenario 2: Request Valid SNAP B2B Access Token (`POST /v1.0/access-token/b2b`)

1. Calculate Signature using Go/Bash helper or OpenSSL:
   ```bash
   TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%S+00:00")
   CLIENT_KEY="client-partner-001"
   STRING_TO_SIGN="${CLIENT_KEY}|${TIMESTAMP}"
   SIGNATURE=$(echo -n "$STRING_TO_SIGN" | openssl dgst -sha256 -sign client_private.pem | base64 -w 0)
   ```

2. Execute HTTP Request:
   ```bash
   curl -X POST http://localhost:8080/v1.0/access-token/b2b \
     -H "Content-Type: application/json" \
     -H "X-CLIENT-KEY: client-partner-001" \
     -H "X-TIMESTAMP: ${TIMESTAMP}" \
     -H "X-SIGNATURE: ${SIGNATURE}" \
     -H "Idempotency-Key: test-idempotency-key-001" \
     -d '{"grantType": "client_credentials"}'
   ```

3. Expected Response (200 OK):
   ```json
   {
     "responseCode": "2007300",
     "responseMessage": "Successful",
     "accessToken": "eyJhbGciOiJSUzI1Ni...",
     "tokenType": "Bearer",
     "expiresIn": "900",
     "additionalInfo": {}
   }
   ```

---

### Scenario 3: Verify Idempotency Replay
Re-send the exact same command above with `-H "Idempotency-Key: test-idempotency-key-001"`. Verify that the server returns HTTP 200 OK with the exact cached response payload instantly (< 2ms) without executing duplicate business logic or DB calls.
