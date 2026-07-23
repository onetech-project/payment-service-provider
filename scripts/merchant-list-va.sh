#!/usr/bin/env bash
#
# Merchant-side: POST /v1.0/transfer-va/list
# There's no separate single-VA "inquiry" endpoint on the merchant side in
# this codebase (inquiry/payment/status under /v1.0/transfer-va are the
# vendor-facing SNAP callbacks — see vendor-inquiry-va.sh). This is the
# merchant-facing equivalent: list/check VA status, optionally filtered down
# to one virtualAccountNo.
#
# Usage:
#   ./scripts/merchant-list-va.sh -s <partnerServiceId> [-v <virtualAccountNo>] [-u <base-url>]
#
# Requires: curl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
VA_NO=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> [-v <virtualAccountNo>] [-u <base-url>]" >&2
	exit 1
}

while getopts "s:v:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$PARTNER_SERVICE_ID" ]] && usage

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "virtualAccountNo": "${VA_NO}",
  "page": 1,
  "pageSize": 20
}
JSON
)

echo "==> POST ${BASE_URL}/v1.0/transfer-va/list"
echo

curl -sS -X POST "${BASE_URL}/v1.0/transfer-va/list" \
	-H "Content-Type: application/json" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
