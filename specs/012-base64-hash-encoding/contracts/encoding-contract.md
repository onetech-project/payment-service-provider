# Contract: Base64 Hash/Signature Encoding

Applies to every signature-protected surface in this system, replacing hex encoding with standard base64 (`base64.StdEncoding` / `openssl base64`, `=`-padded, no URL-safe variant) for the request-body hash and HMAC signature components below.

## Inbound: Vendor-facing endpoints (`SNAPAuthMiddleware`)

`POST {snapBasePath}/transfer-va/{inquiry,payment,status}`

- `X-SIGNATURE` header: HMAC-SHA512 over `stringToSign`, base64-encoded (was hex).
- `stringToSign`'s `RequestBodyHash` component: SHA-256 of the raw request body, base64-encoded (was hex).
- All other header checks (`X-TIMESTAMP`, `CHANNEL-ID`, `X-PARTNER-ID`, `X-EXTERNAL-ID`, and — for migrated vendors per feature 011 — `Authorization`) are unchanged.
- A request signed using the old hex convention is rejected with the existing `401 "Unauthorized. [Invalid signature]"` response — no distinct error message, since from the server's perspective this is indistinguishable from any other signature mismatch.

## Inbound: Merchant-facing endpoints (`MerchantAuthMiddleware`)

`POST {snapBasePath}/transfer-va/{create-va,list,delete-va}`

- Same `X-SIGNATURE`/`stringToSign` body-hash encoding change as above; `Authorization: Bearer` token validation (feature 010) unchanged.

## Outbound: This system as a SNAP client (`snap/client.go`)

- When calling a vendor's own API, the request-body hash this system computes for its own `stringToSign` is base64-encoded (was hex), and the HMAC signature it sends is base64-encoded (was hex).

## Utility: Signature-generation endpoints (`signature_usecase.go`)

`POST /openapi/v1.0/util/signature-service` (`GenerateServiceSignature`)

- The `bodyHash` computed internally, and the returned `signature` value, are both base64-encoded (was hex). `POST /openapi/v1.0/util/signature-auth` (`GenerateAccessTokenSignature`, RSA-based) is unaffected — already base64.

## Outbound: Merchant webhook callback (`payment_notification_worker.go`)

- `X-Signature` header on payment-notification callbacks: HMAC-SHA512 over the JSON body, base64-encoded (was hex). `X-Timestamp` and body format unchanged.

## Internal: Idempotency payload-mismatch detection (`idempotency.go`)

- `PayloadHash` stored in the Redis-cached response envelope: SHA-256 of the request body, base64-encoded (was hex). No externally-visible contract — this value is never sent in any response or header.

## Client tooling (informational — not a server contract, but must match it)

`scripts/vendor-inquiry-va.sh`, `vendor-payment-va.sh`, `merchant-create-va.sh`, `merchant-delete-va.sh`, `merchant-list-va.sh` must compute `BODY_HASH` and `SIGNATURE` via base64 (`openssl dgst ... -binary | openssl base64 -A`) to remain compatible with the server after this change.
