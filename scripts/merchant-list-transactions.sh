#!/usr/bin/env bash
#
# Merchant-side: POST /openapi/v1.0/transfer-va/list-transactions
# Lists individual PAYMENT/transaction events, one entry per payment —
# the per-payment counterpart of merchant-list-va.sh, which lists one entry
# per registered VA number (feature 013-no-bill-payment-transaction).
#
# This split exists because a no-bill VA (vaType 01/04) is registered once and
# then paid many times: listing transactions under the VA listing would report
# one such VA as N separate VAs.
#
# -v filters to a single virtualAccountNo, which is the usual way to answer
# "how many times has this VA been paid, and for how much each time?".
#
# Usage:
#   ./scripts/merchant-list-transactions.sh [-s <partnerServiceId>] [-v <virtualAccountNo>] (-g <secret> -o <access-token> | -f <.env.merchant.NAME>) [-u <base-url>]
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
ENDPOINT="/openapi/v1.0/transfer-va/list-transactions"
PARTNER_SERVICE_ID=""
VA_NO=""
ACCESS_TOKEN=""
MERCHANT_SECRET=""
ENV_FILE=""

usage() {
	echo "Usage: $0 [-s <partnerServiceId>] [-v <virtualAccountNo>] (-g <secret> -o <access-token> | -f <.env.merchant.NAME>) [-u <base-url>]" >&2
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

while getopts "s:v:o:g:f:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
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

# Unlike the VA listing, partnerServiceId is optional: filtering by
# virtualAccountNo alone is the common case here.
[[ -z "$ACCESS_TOKEN" || -z "$MERCHANT_SECRET" ]] && usage
[[ -z "$PARTNER_SERVICE_ID" && -z "$VA_NO" ]] && usage

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "virtualAccountNo": "${VA_NO}",
  "page": 1,
  "pageSize": 20
}
JSON
)

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"

# SNAP symmetric signature, merchant-side convention (feature
# 010-merchant-hmac-signature): AccessToken component is the REAL bearer
# token (unlike the vendor-side convention in vendor-inquiry-va.sh, which
# always uses an empty string there since no header ever carries it).
# bodyHash/signature are base64-encoded (feature 012-base64-hash-encoding), not hex.
# `jq -cj .` is the MinifyJson step and is load-bearing: the server hashes the
# minified body, so hashing $BODY raw (it is pretty-printed here) yields a
# different digest and every request comes back 401. -j (not just -c)
# suppresses jq's trailing newline, which would otherwise be hashed too.
BODY_HASH="$(printf '%s' "$BODY" | jq -cj . | openssl dgst -sha256 -binary | openssl base64 -A)"
STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$MERCHANT_SECRET" -binary | openssl base64 -A)"

echo "==> POST ${BASE_URL}${ENDPOINT}" >&2
echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> stringToSign: $STRING_TO_SIGN" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo >&2

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "Idempotency-Key: $(uuidgen)" \
	-H "X-EXTERNAL-ID: $(date +%Y%m%d%H%M%S)$RANDOM" \
	-H "Authorization: Bearer ${ACCESS_TOKEN}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
