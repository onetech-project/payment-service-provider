# Quickstart: Validating Merchant HMAC Signature Verification

Validates the behavior in [spec.md](./spec.md) and [data-model.md](./data-model.md) end-to-end.

## Prerequisites

- Local stack running: `docker compose up -d`.
- A registered B2B test client (RSA keypair in `client_apps`/`client_keys`) — see `specs/001-snap-token-management/quickstart.md` Scenario 1.
- `ADMIN_API_KEY` known (default `changeme` in local `.env`).
- `curl`, `openssl`, `jq` installed.

## Scenario 1 — Provision a merchant shared secret (US3)

```bash
curl -s -X POST http://localhost:8080/admin/clients/<clientId>/secret \
  -H "X-Admin-API-Key: changeme" -H "Content-Type: application/json" \
  -d '{"secretId":"secret-01","secretValue":"my-merchant-shared-secret"}'
```

**Expected**: `201`, secret provisioned.

## Scenario 2 — Request with valid token but no signature is rejected (US1)

```bash
ACCESS_TOKEN="$(./scripts/curl-b2b-token.sh -i <clientId> -p <private_key.pem> -u http://localhost:8080 | jq -r '.accessToken')"
curl -s -X POST http://localhost:8080/openapi/v1.0/transfer-va/create-va \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "X-EXTERNAL-ID: qs-$(date +%s)" \
  -d '{"partnerServiceId":"088899","virtualAccountName":"Test","trxId":"trx-qs-1"}'
```

**Expected**: `401 Unauthorized` — valid token alone is no longer sufficient (FR-004, US4 Scenario 2).

## Scenario 3 — Request with valid token AND valid signature succeeds (US1, US4)

Compute `X-SIGNATURE = HMAC-SHA512(secretValue, "POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:<sha256hex(body)>:<timestamp>")` and retry with `X-TIMESTAMP`/`X-SIGNATURE` headers added.

**Expected**: `2002700` success — unchanged happy-path behavior (SC-004).

## Scenario 4 — Valid signature but invalid/missing token is rejected (US4 Scenario 3)

Repeat Scenario 3 with a garbage `Authorization` header but a signature computed as if the real token were used.

**Expected**: `401` — token check still runs first and independently.

## Scenario 5 — Unprovisioned client fails closed (US3 Scenario 2)

Use a different, valid `accessToken` for a client that has never had `POST /admin/clients/:clientId/secret` called for it, with any signature.

**Expected**: `401` regardless of signature correctness.

## Scenario 6 — Stale timestamp rejected (US2)

Repeat Scenario 3 with `X-TIMESTAMP` set an hour in the past (and a signature computed against that same old timestamp).

**Expected**: `401`.

## Scenario 7 — Vendor-side endpoints unaffected (US5)

Re-run `./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080` — all 19 checks from feature 009 must still pass unchanged, since this feature does not touch `SNAPAuthMiddleware`.

## Scenario 8 — Full regression

Re-run the full merchant-side flow (`merchant-create-va.sh` etc., updated to sign requests once this feature ships) end-to-end and confirm no regressions versus feature 009's behavior beyond the new signature requirement.
