#!/usr/bin/env bash
#
# Merchant-side: DELETE /openapi/v1.0/transfer-va/delete-va
#
# Usage:
#   ./scripts/merchant-delete-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] (-g <secret> -o <access-token> | -f <.env.merchant.NAME>) [-u <base-url>]
#
# Auth (feature 009-transfer-va-auth + 010-merchant-hmac-signature): this
# endpoint requires BOTH a valid accessToken (Authorization: Bearer) AND a
# valid X-SIGNATURE computed with the merchant's shared secret.
#   -f <.env.merchant.NAME>  Preferred. A credentials file produced by
#                            onboard-merchant.sh. Fetches a fresh accessToken
#                            automatically via curl-b2b-token.sh.
#   -g <secret> -o <token>   Manual: the merchant's shared secret and an
#                            already-obtained accessToken directly.
# This merchant secret is a DIFFERENT credential from a vendor's
# VENDOR_CLIENT_SECRET — never reuse a .env.<vendor>.<channel> file here.
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
ENDPOINT="/openapi/v1.0/transfer-va/delete-va"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
TRX_ID=""
ACCESS_TOKEN=""
MERCHANT_SECRET=""
ENV_FILE=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] (-g <secret> -o <access-token> | -f <.env.merchant.NAME>) [-u <base-url>]" >&2
	exit 1
}

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

while getopts "s:c:v:t:o:g:f:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	g) MERCHANT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$MERCHANT_SECRET" ]] && MERCHANT_SECRET="$(read_env_var "$ENV_FILE" MERCHANT_SECRET_VALUE || true)"
	if [[ -z "$ACCESS_TOKEN" ]]; then
		MERCHANT_CLIENT_ID="$(read_env_var "$ENV_FILE" MERCHANT_CLIENT_ID || true)"
		MERCHANT_PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" MERCHANT_PRIVATE_KEY_PATH || true)"
		if [[ -n "$MERCHANT_CLIENT_ID" && -n "$MERCHANT_PRIVATE_KEY_PATH" ]]; then
			TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$MERCHANT_CLIENT_ID" -p "$MERCHANT_PRIVATE_KEY_PATH" -u "$BASE_URL")"
			ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty' 2>/dev/null || true)"
			[[ -z "$ACCESS_TOKEN" ]] && { echo "!! Failed to obtain accessToken for ${MERCHANT_CLIENT_ID} — aborting." >&2; exit 1; }
		fi
	fi
fi

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NO" || -z "$ACCESS_TOKEN" || -z "$MERCHANT_SECRET" ]] && usage

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "trxId": "${TRX_ID}"
}
JSON
)

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"

# SNAP symmetric signature, merchant-side convention (feature
# 010-merchant-hmac-signature): AccessToken component is the REAL bearer
# token (unlike the vendor-side convention in vendor-payment-va.sh, which
# always uses an empty string there since no header ever carries it).
# bodyHash/signature are base64-encoded (feature 012-base64-hash-encoding), not hex.
BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -binary | openssl base64 -A)"
STRING_TO_SIGN="DELETE:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$MERCHANT_SECRET" -binary | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-cancel-flow.sh).
echo "==> DELETE ${BASE_URL}${ENDPOINT}" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> stringToSign: $STRING_TO_SIGN" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo >&2

curl -sS -X DELETE "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-H "Authorization: Bearer ${ACCESS_TOKEN}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
