# Quickstart: Validate Base64 Hash/Signature Encoding

## Prerequisites

- Local stack running (`docker compose up -d --build`, app rebuilt with this feature's changes).
- A vendor config (e.g. `.env.bca.va`) and/or an onboarded merchant (`onboard-merchant.sh`) available, as used in prior features' e2e runs.

## Scenario 1: Vendor request signed with base64 body hash is accepted

1. Compute `BODY_HASH` as `openssl dgst -sha256 -binary <<< "$BODY" | openssl base64 -A` (updated `vendor-inquiry-va.sh`).
2. Build `stringToSign` and `X-SIGNATURE` (HMAC-SHA512, base64) using the updated script.
3. `POST {snapBasePath}/transfer-va/inquiry`.
4. **Expected**: normal business response (`200`/business-level success), same as before this change — only the encoding of the internal comparison changed.

## Scenario 2: Vendor request signed with the old hex body hash is rejected

1. Manually compute `BODY_HASH` the old way (`openssl dgst -sha256 -hex`) and sign with it.
2. Send the request.
3. **Expected**: `401 "Unauthorized. [Invalid signature]"` — same generic message as any other signature mismatch, since hex vs. base64 isn't distinguished as a special case (per contracts/encoding-contract.md).

## Scenario 3: Merchant request (create-va) signed with base64 succeeds

1. Use updated `merchant-create-va.sh` (base64 `BODY_HASH` + `X-SIGNATURE`) with a valid `Authorization: Bearer` token (feature 010).
2. **Expected**: `200`/business-level success.

## Scenario 4: This system's outbound call to a vendor uses base64

1. Trigger a code path where this system calls out to a vendor's own API as a SNAP client (`snap/client.go`) — e.g. via whatever existing integration test or usecase exercises `internal/adapter/gateway/snap/client.go`.
2. **Expected**: outbound `X-SIGNATURE` and body-hash component are base64-encoded (verify via request capture/log in a test double, or by reading `client_test.go` assertions after the change).

## Scenario 5: Merchant webhook callback signature is base64

1. Trigger a payment notification (e.g. via `vendor-payment-va.sh` against a VA created with a `notificationUrl`, per `merchant-create-va.sh`'s existing e2e pattern).
2. Inspect the delivered callback's `X-Signature` header (e.g. via a local HTTP capture endpoint, or existing e2e helper scripts that echo received headers).
3. **Expected**: base64-encoded HMAC-SHA512 signature, verifiable with `openssl dgst -sha512 -hmac <secret> -binary <<< "$BODY" | openssl base64 -A`.

## Scenario 6: Idempotency payload-mismatch detection still works

1. Send two requests with the same `X-EXTERNAL-ID` but different bodies to any idempotency-protected endpoint.
2. **Expected**: second request rejected with the existing `422 "Unprocessable Entity. X-EXTERNAL-ID payload mismatch."` response, confirming the internal hash change didn't break mismatch detection.

## Automated verification

```bash
go test ./internal/infrastructure/crypto/... ./internal/adapter/delivery/http/middleware/... ./internal/adapter/delivery/worker/... ./internal/usecase/... ./internal/adapter/gateway/snap/... -v
```

Full suite + coverage gate (per constitution Principle XI):

```bash
go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```
