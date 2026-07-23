#!/usr/bin/env bash
#
# Merchant-side: DELETE /v1.0/transfer-va/delete-va
#
# Usage:
#   ./scripts/merchant-delete-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] [-u <base-url>]
#
# Requires: curl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
TRX_ID=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> [-t <trxId>] [-u <base-url>]" >&2
	exit 1
}

while getopts "s:c:v:t:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
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
echo "==> DELETE ${BASE_URL}/v1.0/transfer-va/delete-va" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo >&2

curl -sS -X DELETE "${BASE_URL}/v1.0/transfer-va/delete-va" \
	-H "Content-Type: application/json" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
