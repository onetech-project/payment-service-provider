#!/usr/bin/env bash
#
# Merchant-side: POST /v1.0/transfer-va/create-va (Service Code 27)
# Creates a new Virtual Account. virtualAccountNo is client-supplied per ASPI
# VAIdentity (not server-generated). The payment callback URL is a proprietary
# extension — there is no top-level notificationUrl field in VAUpsertRequest —
# so it is sent via additionalInfo.dbUrlProcess, the extension slot the spec
# itself defines for this endpoint (aspi-open-api-va.yaml:317-320). See
# vendor-payment-va.sh for the callback flow this registers.
#
# Usage:
#   ./scripts/merchant-create-va.sh -s <partnerServiceId> -c <customerNo> \
#       -n <virtualAccountName> -a <amount> (-e <client-secret> | -f <env-file>) \
#       [-w <notificationUrl>] [-v <virtualAccountNo>] [-t <trxId>] [-i <channel-id>] \
#       [-p <partner-id>] [-o <access-token>] [-b <billNo>] [-d <billName>] [-u <base-url>]
#
# -f loads VENDOR_CLIENT_SECRET straight out of a .env.<vendor>.<channel> file
# (same raw-secret convention the server itself uses, see vendor_config.go),
# so the secret never has to be typed as a plain CLI argument (visible in
# shell history / `ps aux`). -e still wins if both are given.
#
# -b <billNo> attaches one billDetails entry (billNo + billAmount = -a; -d sets
# billName) so create-va actually exercises bill-detail persistence
# (internal/infrastructure/database/va_repository.go SaveBillDetails) instead
# of always sending an empty billDetails array. Omit -b for a VA with no bills.
#
# Requires: curl, openssl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
ENDPOINT="/v1.0/transfer-va/create-va"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NAME=""
AMOUNT="100000.00"
NOTIFICATION_URL=""
TRX_ID=""
VA_NO=""
BILL_NO=""
BILL_NAME=""
CLIENT_SECRET=""
ENV_FILE=""
CHANNEL_ID="95231"
PARTNER_ID="111111"
ACCESS_TOKEN=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -a <amount> (-e <client-secret> | -f <env-file>) [-w <notificationUrl>] [-v <virtualAccountNo>] [-t <trxId>] [-i <channel-id>] [-p <partner-id>] [-o <access-token>] [-b <billNo>] [-d <billName>] [-u <base-url>]" >&2
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

while getopts "s:c:n:a:w:v:t:e:f:i:p:o:b:d:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	w) NOTIFICATION_URL="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	b) BILL_NO="$OPTARG" ;;
	d) BILL_NAME="$OPTARG" ;;
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
fi

# X-CLIENT-KEY is intentionally not requested/sent here — per ASPI spec it's
# only used on the access-token endpoint, never on transfer-va endpoints
# (aspi-open-api-va.yaml has no XClientKey parameter on any transfer-va path).
[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NAME" || -z "$CLIENT_SECRET" ]] && usage
# trxId becomes the row's inquiry_request_id, which is UNIQUE across the
# WHOLE va_transactions table (not just per-VA) — $(date +%s) alone would
# collide if two different VAs are created within the same second (e.g. two
# calls in one e2e script run), silently clobbering the first VA's row
# instead of creating a second one. $RANDOM avoids that regardless of timing.
[[ -z "$TRX_ID" ]] && TRX_ID="TRX-$(date +%s)$RANDOM"
# virtualAccountNo is mandatory per ASPI spec VAIdentity (client-supplied, not
# server-generated); default to partnerServiceId+customerNo only for convenience.
[[ -z "$VA_NO" ]] && VA_NO="${PARTNER_SERVICE_ID}${CUSTOMER_NO}"

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"

# The callback URL rides in additionalInfo.dbUrlProcess (ASPI's own extension
# slot for VAUpsertRequest), never as a top-level notificationUrl field.
ADDITIONAL_INFO_FIELD=""
if [[ -n "$NOTIFICATION_URL" ]]; then
	ADDITIONAL_INFO_FIELD="\"additionalInfo\": {\"dbUrlProcess\": \"${NOTIFICATION_URL}\"},"
fi

# billDetails per ASPI BillDetail schema — only attached when -b is given, so
# a plain create-va call (no bills) still sends a spec-valid payload.
BILL_DETAILS_FIELD=""
if [[ -n "$BILL_NO" ]]; then
	BILL_NAME_FIELD=""
	if [[ -n "$BILL_NAME" ]]; then
		BILL_NAME_FIELD="\"billName\": \"${BILL_NAME}\","
	fi
	BILL_DETAILS_FIELD="\"billDetails\": [{\"billNo\": \"${BILL_NO}\", ${BILL_NAME_FIELD} \"billAmount\": {\"value\": \"${AMOUNT}\", \"currency\": \"IDR\"}}],"
fi

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "virtualAccountName": "${VA_NAME}",
  "trxId": "${TRX_ID}",
  ${ADDITIONAL_INFO_FIELD}
  ${BILL_DETAILS_FIELD}
  "totalAmount": {"value": "${AMOUNT}", "currency": "IDR"},
  "virtualAccountTrxType": "C"
}
JSON
)

# SNAP symmetric signature: HMAC_SHA512(clientSecret, stringToSign)
# stringToSign = HTTPMethod:EndpointUrl:AccessToken:Lowercase(HexEncode(SHA-256(minify(body)))):Timestamp
BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -hex | awk '{print $NF}')"
STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -hex | awk '{print $NF}')"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-flow.sh).
echo "==> POST ${BASE_URL}${ENDPOINT}" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo >&2

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
