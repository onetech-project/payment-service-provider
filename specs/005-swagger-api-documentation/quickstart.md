# Quickstart: Validating Swagger API Documentation

## Prerequisites

- Go toolchain matching `go.mod` (`go 1.26.5`).
- `swaggo/swag` CLI installed: `go install github.com/swaggo/swag/cmd/swag@latest`.
- Local `.env` / config sufficient to run `cmd/api` (per existing project run instructions).

## Setup

```bash
# from repo root
swag init -g cmd/api/main.go --output docs
go build ./...
```

## Run

```bash
go run ./cmd/api
```

## Validate — User Story 1 (Browse Complete API Reference)

1. Open `http://localhost:<port>/swagger/index.html` in a browser.
2. Confirm all tags are present: Health, Token, Admin Client Management, Signature Utilities, Virtual Account, Merchant VA Dashboard.
3. Confirm the endpoint count under each tag matches the inventory in [data-model.md](data-model.md#endpoint-inventory-from-current-router-registration) (13 endpoints total).
4. Expand any endpoint and confirm a plain-language description is present (not just the method/path).

**Expected outcome**: every registered route is visible and documented; SC-001 satisfied.

## Validate — User Story 2 (Try Out Requests)

1. In Swagger UI, expand `GET /health`, click "Try it out" → "Execute". Confirm a real `200` response body is shown.
2. Expand `POST /openapi/v1.0/transfer-va/inquiry`. Click "Authorize" (or fill header params inline) and supply `X-CLIENT-KEY` / `X-TIMESTAMP` / `X-SIGNATURE` values (generate a signature first via `POST /api/v1/utilities/signature-service`, documented inline). Execute and confirm either a real success or a real SNAP auth-failure response is returned (not a mocked example).
3. Expand `POST /admin/clients`. Supply the admin auth header. Execute and confirm a real response.

**Expected outcome**: at least one endpoint per auth tier (public / SNAP-signed / admin) can be executed live from the docs; SC-004 satisfied.

## Validate — User Story 3 (Request/Response/Error Contract)

1. Pick any endpoint, e.g. `POST /openapi/v1.0/transfer-va/payment`.
2. Confirm the request body schema lists every field with type, required flag, and description, matching the actual `domain.VAPaymentRequest` (or equivalent) struct.
3. Confirm a realistic example request body is shown.
4. Confirm the success response schema + example match the actual handler's success JSON.
5. Trigger at least one documented error case against the running instance (e.g. send an inquiry with a missing mandatory field) and confirm the returned `responseCode`/`responseMessage` matches a documented `@Failure` entry exactly.

**Expected outcome**: docs are verifiably accurate against running code, not aspirational; SC-002 and SC-003 satisfied.

## Regeneration check (drift prevention)

1. Modify a handler's doc-comment annotation (e.g. change a `@Summary`).
2. Re-run `swag init -g cmd/api/main.go --output docs`.
3. Restart the service and confirm the Swagger UI reflects the change.

**Expected outcome**: documentation is regenerated from source, confirming FR-010.
