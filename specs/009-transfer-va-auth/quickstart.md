# Quickstart: Validating Transfer-VA Signature & Token Enforcement

Validates the behavior described in [spec.md](./spec.md) and [data-model.md](./data-model.md) end-to-end against a running instance.

## Prerequisites

- Local stack running: `docker compose up -d` (app, postgres, redis).
- A registered B2B test client (RSA keypair in `client_apps`/`client_keys`) and a vendor HMAC secret in a `.env.<vendor>.<channel>` file — see `specs/001-snap-token-management/quickstart.md` Scenario 1 for onboarding commands. This repo already has `.env.bca.va` set up from prior feature work.
- `curl`, `openssl`, `jq` installed.

## Scenario 1 — Vendor-side: bad signature rejected, correct signature still works (US1)

```bash
./scripts/vendor-inquiry-va.sh -s 15973 -c "04000000000000000001" -v "1597304000000000000000001" \
  -e "wrong-secret-on-purpose" -t "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: `401 Unauthorized`, `responseCode` `4010000` (or equivalent), request rejected before reaching `vaHandler.Inquiry`.

Then repeat with the correct `VENDOR_CLIENT_SECRET` value from `.env.bca.va`:

```bash
./scripts/vendor-inquiry-va.sh -s 15973 -c "04000000000000000001" -v "1597304000000000000000001" \
  -f .env.bca.va -t "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: succeeds exactly as before (US1 Scenario 1) — proves enforcement doesn't break correctly-signed requests.

## Scenario 2 — Vendor-side: stale timestamp rejected (US2)

Manually craft a request with `X-TIMESTAMP` set to 1 hour in the past (requires bypassing the script's auto-generated timestamp — e.g. a raw `curl` with a hand-computed signature for that old timestamp).

**Expected**: `401 Unauthorized` — rejected for staleness, independent of whether the signature itself is otherwise correctly computed for that (old) timestamp.

## Scenario 3 — Vendor-side: fail-closed on missing secret (Edge Case)

Temporarily blank out `VENDOR_CLIENT_SECRET` in a test `.env.<vendor>.<channel>` file, restart, then send any request to that vendor/channel's inquiry endpoint.

**Expected**: `401 Unauthorized` for every request to that vendor/channel, regardless of signature — proves the fail-closed behavior (FR-004).

## Scenario 4 — Merchant-side: create-VA rejected without a token (US3)

```bash
curl -s -X POST http://localhost:8080/openapi/v1.0/transfer-va/create-va \
  -H "Content-Type: application/json" \
  -d '{"partnerServiceId":"15973","virtualAccountName":"No Token Test","trxId":"trx-no-token"}'
```

**Expected**: `401 Unauthorized` — no `Authorization` header at all.

## Scenario 5 — Merchant-side: create-VA succeeds with a valid token

```bash
ACCESS_TOKEN="$(./scripts/curl-b2b-token.sh -i "$CLIENT_ID" -p "$PRIVATE_KEY_PATH" -u http://localhost:8080 | jq -r '.accessToken')"
./scripts/merchant-create-va.sh -s 15973 -n "With Token Test" -y 04 \
  -t "trx-with-token-$(date +%s)" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: `responseCode` 2xx — unchanged happy-path behavior (US3 Scenario 1), since current tooling already sends this token via `-o`.

## Scenario 6 — Merchant-side: expired/invalid token rejected

```bash
curl -s -X POST http://localhost:8080/openapi/v1.0/transfer-va/create-va \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer not-a-real-jwt" \
  -d '{"partnerServiceId":"15973","virtualAccountName":"Bad Token Test","trxId":"trx-bad-token"}'
```

**Expected**: `401 Unauthorized`.

## Scenario 7 — Isolation: mechanisms don't cross endpoint groups (US4)

Repeat Scenario 1's correctly-signed vendor inquiry call with no `Authorization` header at all → still succeeds (bearer token not required on vendor routes). Repeat Scenario 5's merchant create-VA call with no `X-SIGNATURE` header at all → still succeeds (signature not required on merchant routes).

## Scenario 8 — Regression: full e2e flows still pass

```bash
./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080
```

**Expected**: all checks still pass (19/19 as of feature 008), now running against the always-enforced vendor signature path plus the now-token-gated merchant path — since the script already sends a real `accessToken` and a correctly-computed signature, none of the new checks should change its outcome.
