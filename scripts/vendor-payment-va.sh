#!/usr/bin/env bash
#
# Vendor-side: POST /openapi/v1.0/transfer-va/payment
# Simulates the switching vendor notifying this PSP that a customer paid a
# VA. On success, the PSP looks up the notificationUrl registered via
# merchant-create-va.sh and asynchronously calls the merchant back
# (internal/usecase/va_usecase.go notifyMerchant -> Asynq queue ->
# internal/adapter/delivery/worker/payment_notification_worker.go).
#
# Usage:
#   ./scripts/vendor-payment-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> \
#       -a <amount> (-e <client-secret> | -f <env-file>) [-k <client-key>] [-i <channel-id>] [-p <partner-id>] [-u <base-url>]
#
# X-CLIENT-KEY is NOT part of the ASPI spec for this endpoint (only used on the
# access-token endpoint) — sent here only if -k is explicitly provided.
#
# -f loads VENDOR_CLIENT_SECRET straight out of a .env.<vendor>.<channel> file
# (same raw-secret convention the server itself uses, see vendor_config.go),
# so the secret never has to be typed as a plain CLI argument (visible in
# shell history / `ps aux`). -e still wins if both are given.
#
# Requires: curl, openssl, uuidgen
set -euo pipefail

BASE_URL="http://localhost:8080"
ENDPOINT="/openapi/v1.0/transfer-va/payment"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
AMOUNT="100000.00"
CLIENT_KEY=""
CLIENT_SECRET=""
ENV_FILE=""
CHANNEL_ID="95231"
PARTNER_ID="111111"
ACCESS_TOKEN=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> -a <amount> (-e <client-secret> | -f <env-file>) [-k <client-key>] [-i <channel-id>] [-p <partner-id>] [-t <access-token>] [-u <base-url>]" >&2
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

while getopts "s:c:v:a:k:e:f:i:p:t:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	k) CLIENT_KEY="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	t) ACCESS_TOKEN="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$CLIENT_SECRET" ]] && CLIENT_SECRET="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_SECRET || true)"
	[[ -z "$CLIENT_KEY" ]] && CLIENT_KEY="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_ID || true)"
	# read_env_var succeeds (and prints "") when the key exists but its value is
	# blank, e.g. "VENDOR_CLIENT_SECRET=" — flag that explicitly so it's not
	# confused with "the -f flag itself is missing".
	[[ -z "$CLIENT_SECRET" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_SECRET is empty — fill it in, or pass -e <client-secret> directly." >&2
fi

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NO" || -z "$CLIENT_SECRET" ]] && usage

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
# $(date +%s) alone is only second-resolution — two payment calls against the
# same VA within the same second (e.g. a fast retry/e2e test) would otherwise
# collide onto the identical paymentRequestId and get silently treated as an
# idempotent replay of the SAME payment instead of a genuinely new attempt.
# $RANDOM makes each run's id unique regardless of timing (payment_request_id
# column is VARCHAR(30); "PAY-" + 10-digit epoch + up to 5-digit $RANDOM fits).
INQUIRY_REQUEST_ID="INQ-$(date +%s)$RANDOM"
PAYMENT_REQUEST_ID="PAY-$(date +%s)$RANDOM"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"
TRX_DATE="$(date +%Y-%m-%dT%H:%M:%S%:z)"
# reference_no column is varchar(11) — keep it short
REFERENCE_NO="R$(date +%s | tail -c 10)"

# paymentRequestId + paidAmount are the only mandatory fields per ASPI spec
# (PaymentRequest.required); totalAmount is optional (checked for mismatch
# only when present); transactionDate does not exist in the spec — only
# trxDateTime does — so it is intentionally omitted here.
BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "inquiryRequestId": "${INQUIRY_REQUEST_ID}",
  "paymentRequestId": "${PAYMENT_REQUEST_ID}",
  "paidAmount": {"value": "${AMOUNT}", "currency": "IDR"},
  "totalAmount": {"value": "${AMOUNT}", "currency": "IDR"},
  "trxDateTime": "${TRX_DATE}",
  "referenceNo": "${REFERENCE_NO}"
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
echo "==> paymentRequestId: ${PAYMENT_REQUEST_ID}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo >&2

CLIENT_KEY_HEADER=()
if [[ -n "$CLIENT_KEY" ]]; then
	CLIENT_KEY_HEADER=(-H "X-CLIENT-KEY: ${CLIENT_KEY}")
fi

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	"${CLIENT_KEY_HEADER[@]}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)

echo >&2
echo "==> If a VA with this virtualAccountNo was created via merchant-create-va.sh" >&2
echo "    (with a notificationUrl), a callback should have been enqueued to Asynq" >&2
echo "    and delivered by the payment_notification_worker shortly after this call." >&2
