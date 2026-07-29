#!/usr/bin/env bash
#
# Merchant-side: DELETE /openapi/v1.0/transfer-va/delete-va
#
# Usage:
#   ./scripts/merchant-delete-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] [-o <access-token>] [-u <base-url>]
#
# -o <access-token> is required (feature 009-transfer-va-auth): this
# endpoint now requires a valid Authorization: Bearer <accessToken> issued
# by curl-b2b-token.sh / POST /access-token/b2b.
#
# Requires: curl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
TRX_ID=""
ACCESS_TOKEN=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] [-o <access-token>] [-u <base-url>]" >&2
	exit 1
}

while getopts "s:c:v:t:o:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NO" ]] && usage

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "trxId": "${TRX_ID}"
}
JSON
)

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-cancel-flow.sh).
echo "==> DELETE ${BASE_URL}/openapi/v1.0/transfer-va/delete-va" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo >&2

AUTH_HEADER=()
if [[ -n "$ACCESS_TOKEN" ]]; then
	AUTH_HEADER=(-H "Authorization: Bearer ${ACCESS_TOKEN}")
fi

curl -sS -X DELETE "${BASE_URL}/openapi/v1.0/transfer-va/delete-va" \
	-H "Content-Type: application/json" \
	-H "X-EXTERNAL-ID: $(date +%Y%m%d%H%M%S)$RANDOM" \
	"${AUTH_HEADER[@]}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
