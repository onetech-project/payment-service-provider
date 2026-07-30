# Quickstart: Validate Vendor Access Token in Symmetric Signature

## Prerequisites

- Local stack running (`docker compose up -d`, app + postgres + redis).
- A vendor entry in `config.VendorConfig` (env/config file) with `ClientID` set to a `client_id` that has an active `client_apps` row and registered `client_keys` RSA key (provision via the existing admin client-onboarding path used for merchants, feature 010).
- `VendorConfig.ClientSecret` set as today (used for HMAC verification, unchanged).

## Scenario 1: Migrated vendor, correctly bound token → accepted

1. Obtain a token: `POST {snapBasePath}/access-token/b2b` with the vendor's `client_id` credentials → returns `accessToken`.
2. Build `stringToSign = "POST:{path}:{accessToken}::" ` — i.e. `HTTPMethod:RelativeURL:AccessToken:SHA256Hex(Body):Timestamp` (see `scripts/vendor-inquiry-va.sh` for the exact concatenation once updated).
3. Compute `X-SIGNATURE` via HMAC using `VendorConfig.ClientSecret`.
4. `POST {snapBasePath}/transfer-va/inquiry` with `Authorization: Bearer {accessToken}`, `X-SIGNATURE`, `X-TIMESTAMP`, and other required headers.
5. **Expected**: `200`/normal business response — request accepted.

## Scenario 2: Migrated vendor, missing Authorization → rejected with distinguishable error

1. Same as Scenario 1 but omit `Authorization` and sign with the legacy empty-AccessToken `stringToSign`.
2. **Expected**: `401`, error message identifies the missing/invalid `Authorization` header (not a generic signature-mismatch message).

## Scenario 3: Migrated vendor, token swapped after signing → rejected

1. Compute a valid signature using token A's `stringToSign`.
2. Send the request with `Authorization: Bearer {tokenB}` (a different, otherwise-valid token).
3. **Expected**: `401 invalid signature`.

## Scenario 4: Migrated vendor, token from a different vendor's client_id → rejected

1. Obtain a valid token for vendor X's `client_id`.
2. Sign and send a request against vendor Y's `VendorConfig` (different `ClientID`, different `ClientSecret`) using vendor X's token in both `Authorization` and the `stringToSign`.
3. **Expected**: `401 invalid signature` (ClientID-claim mismatch, step 4 of the [middleware contract](./contracts/snap-auth-middleware.md)).

## Scenario 5: Legacy (non-migrated) vendor — unchanged behavior

1. Using a `VendorConfig` with no `ClientID` set, sign a request with the legacy empty-AccessToken `stringToSign` and no `Authorization` header.
2. **Expected**: `200`/normal business response — unchanged from pre-feature behavior.

## Automated verification

Run the extended middleware unit tests:

```bash
go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v
```

Full suite + coverage gate (per constitution Principle XI):

```bash
go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```
