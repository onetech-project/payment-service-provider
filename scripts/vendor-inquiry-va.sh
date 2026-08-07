#!/usr/bin/env bash
#
# Vendor-side: POST /openapi/v1.0/transfer-va/inquiry
# Simulates the switching vendor calling this PSP to inquire a VA/bill before
# the customer pays. Protected by SNAPAuthMiddleware (per-vendor config, see
# .env.<vendor>.<channel>), which requires X-TIMESTAMP/X-SIGNATURE (plus
# X-PARTNER-ID/X-EXTERNAL-ID per ASPI spec) — verified via HMAC-SHA512
# (feature 009-transfer-va-auth).
#
# Auth (feature 011-vendor-access-token-signature): for vendors migrated to
# ClientID-based onboarding (VENDOR_CLIENT_ID set in the vendor's
# .env.<vendor>.<channel>), this endpoint ALSO requires a valid accessToken
# (Authorization: Bearer), bound into the AccessToken component of
# stringToSign — mirroring merchant-create-va.sh's convention. For
# non-migrated (legacy) vendors, omit -f's VENDOR_CLIENT_ID/
# VENDOR_PRIVATE_KEY_PATH (or pass no -o) and the empty-AccessToken legacy
# convention is used unchanged.
#
# Usage:
#   ./scripts/vendor-inquiry-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> \
#       (-e <client-secret> | -f <env-file>) [-o <access-token>] [-a <amount>] [-i <channel-id>] [-p <partner-id>] [-u <base-url>]
#
# -f loads VENDOR_CLIENT_SECRET straight out of a .env.<vendor>.<channel> file
# (same raw-secret convention the server itself uses, see vendor_config.go),
# so the secret never has to be typed as a plain CLI argument (visible in
# shell history / `ps aux`). -e still wins if both are given.
#
# -f also auto-fetches an accessToken via curl-b2b-token.sh when the env file
# has VENDOR_CLIENT_ID + VENDOR_PRIVATE_KEY_PATH set (migrated vendor) — no
# need to pass -o yourself in that case. Pass -o directly to override.
#
# Requires: curl, openssl, uuidgen, jq (for accessToken auto-fetch)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
ENDPOINT="/openapi/v1.0/transfer-va/inquiry"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
AMOUNT="100000.00"
CLIENT_SECRET=""
ENV_FILE=""
CHANNEL_ID="95231"
PARTNER_ID="111111"
ACCESS_TOKEN=""
CLIENT_KEY=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> (-e <client-secret> | -f <env-file>) [-o <access-token>] [-k <client-key>] [-a <amount>] [-i <channel-id>] [-p <partner-id>] [-u <base-url>]" >&2
	exit 1
}

# read_env_var extracts KEY=value from a .env.<vendor>.<channel> file,
# stripping surrounding quotes the same way vendor_config.go's parseEnvFile does.
read_env_var() {
	local file="$1" key="$2" line value
	line="$(grep -E "^${key}=" "$file" | tail -n1)"
	[[ -n "$line" ]] || return 1
	value="${line#*=}"
	if [[ "$value" == \"*\" && "$value" == *\" ]]; then
		value="${value:1:${#value}-2}"
	elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
		value="${value:1:${#value}-2}"
	fi
	printf '%s' "$value"
}

while getopts "s:c:v:a:e:f:o:k:i:p:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	k) CLIENT_KEY="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$CLIENT_SECRET" ]] && CLIENT_SECRET="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_SECRET || true)"
	# read_env_var succeeds (and prints "") when the key exists but its value is
	# blank, e.g. "VENDOR_CLIENT_SECRET=" — flag that explicitly so it's not
	# confused with "the -f flag itself is missing".
	[[ -z "$CLIENT_SECRET" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_SECRET is empty — fill it in, or pass -e <client-secret> directly." >&2

	VENDOR_CLIENT_ID="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_ID || true)"
	# X-CLIENT-KEY is not an ASPI transaction-request header, but a deployment
	# is free to list it in VENDOR_REQUIRED_HEADERS, and the UAT instance does
	# — SNAPAuthMiddleware then rejects inquiry/payment/status without it
	# ("Missing required header: X-CLIENT-KEY"). Default it to the vendor's
	# clientId so those deployments work; -k overrides.
	[[ -z "$CLIENT_KEY" ]] && CLIENT_KEY="$VENDOR_CLIENT_ID"

	if [[ -z "$ACCESS_TOKEN" ]]; then
		VENDOR_PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" VENDOR_PRIVATE_KEY_PATH || true)"
		if [[ -n "$VENDOR_CLIENT_ID" && -n "$VENDOR_PRIVATE_KEY_PATH" ]]; then
			echo "==> Fetching accessToken for vendor client ${VENDOR_CLIENT_ID}..." >&2
			TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$VENDOR_CLIENT_ID" -p "$VENDOR_PRIVATE_KEY_PATH" -u "$BASE_URL")"
			ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty' 2>/dev/null || true)"
			[[ -z "$ACCESS_TOKEN" ]] && { echo "!! Failed to obtain accessToken for ${VENDOR_CLIENT_ID} — aborting." >&2; exit 1; }
		fi
		# VENDOR_CLIENT_ID/VENDOR_PRIVATE_KEY_PATH absent means this vendor has
		# not migrated to feature 011 yet — ACCESS_TOKEN stays empty and the
		# legacy signing convention below is used unchanged.
	fi
fi

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NO" || -z "$CLIENT_SECRET" ]] && usage

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
# $(date +%s) alone is only second-resolution — repeated inquiry calls within
# the same second would otherwise collide onto the identical
# inquiryRequestId and get treated as an idempotent replay instead of a
# distinct call. $RANDOM avoids that regardless of timing.
INQUIRY_REQUEST_ID="INQ-$(date +%s)$RANDOM"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"
TXN_DATE_INIT="$(date +%Y-%m-%dT%H:%M:%S%:z)"

# amount is mandatory per ASPI spec (InquiryRequest.required); txnDateInit is
# the spec-correct field name (previously mis-sent as trxDateInit).
BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "txnDateInit": "${TXN_DATE_INIT}",
  "amount": {"value": "${AMOUNT}", "currency": "IDR"},
  "inquiryRequestId": "${INQUIRY_REQUEST_ID}"
}
JSON
)

# SNAP symmetric signature: HMAC_SHA512(clientSecret, stringToSign)
# stringToSign = HTTPMethod:EndpointUrl:AccessToken:Lowercase(HexEncode(SHA-256(minify(body)))):Timestamp
# AccessToken is the real accessToken for migrated vendors (feature
# 011-vendor-access-token-signature), or "" for legacy (non-migrated) vendors.
# The body hash is lowercase hex, per BCA's Signature Symmetric spec. Set
# BODY_HASH_ENCODER="openssl base64 -A" for a vendor configured with
# VENDOR_BODY_HASH_ENCODING=base64 (feature 012-base64-hash-encoding).
# X-SIGNATURE itself is always base64.
# `jq -cj .` is the MinifyJson step and is load-bearing: the server hashes the
# minified body, so hashing $BODY raw (it is pretty-printed here) yields a
# different digest and every request comes back 401. -j (not just -c)
# suppresses jq's trailing newline, which would otherwise be hashed too.
BODY_HASH="$(printf '%s' "$BODY" | jq -cj . | openssl dgst -sha256 -binary | ${BODY_HASH_ENCODER:-xxd -p -c 256})"
STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-flow.sh).
echo "==> POST ${BASE_URL}${ENDPOINT}" >&2
[[ -n "$ACCESS_TOKEN" ]] && echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> stringToSign: $STRING_TO_SIGN" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo "==> Request body:" >&2
echo "$BODY" | (command -v jq >/dev/null && jq . || cat) >&2
echo >&2

# Authorization header is only sent when an accessToken was obtained/passed
# (migrated vendor) — kept out of the array entirely for legacy vendors,
# matching stringToSign's "" AccessToken component above.
AUTH_HEADER=()
[[ -n "$ACCESS_TOKEN" ]] && AUTH_HEADER=(-H "Authorization: Bearer ${ACCESS_TOKEN}")

# X-CLIENT-KEY is sent only when known — it is never part of stringToSign, so
# adding it is inert on deployments that don't list it as required.
CLIENT_KEY_HEADER=()
[[ -n "$CLIENT_KEY" ]] && CLIENT_KEY_HEADER=(-H "X-CLIENT-KEY: ${CLIENT_KEY}")

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	"${AUTH_HEADER[@]}" \
	"${CLIENT_KEY_HEADER[@]}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
