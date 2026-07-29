#!/usr/bin/env bash
#
# Merchant-side: DELETE /openapi/v1.0/transfer-va/delete-va
#
# Usage:
#   ./scripts/merchant-delete-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] -o <access-token> -g <merchant-secret> [-u <base-url>]
#
# -o <access-token> is required (feature 009-transfer-va-auth): this
# endpoint requires a valid Authorization: Bearer <accessToken> issued by
# curl-b2b-token.sh / POST /access-token/b2b.
#
# -g <merchant-secret> is required (feature 010-merchant-hmac-signature):
# this endpoint also requires a valid X-TIMESTAMP/X-SIGNATURE, computed with
# a shared secret provisioned via POST /admin/clients/:clientId/secret. This
# is a DIFFERENT secret from a vendor's VENDOR_CLIENT_SECRET — do not reuse
# a .env.<vendor>.<channel> file's secret here.
#
# Requires: curl, openssl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
ENDPOINT="/openapi/v1.0/transfer-va/delete-va"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
TRX_ID=""
ACCESS_TOKEN=""
MERCHANT_SECRET=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] -o <access-token> -g <merchant-secret> [-u <base-url>]" >&2
	exit 1
}

while getopts "s:c:v:t:o:g:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	g) MERCHANT_SECRET="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

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
BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -hex | awk '{print $NF}')"
STRING_TO_SIGN="DELETE:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$MERCHANT_SECRET" -hex | awk '{print $NF}')"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-cancel-flow.sh).
echo "==> DELETE ${BASE_URL}${ENDPOINT}" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
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
