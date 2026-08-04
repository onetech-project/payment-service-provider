#!/usr/bin/env bash
#
# Prints a fully-signed, copy-paste-ready SNAP request (method, URL, every
# header, and the exact request body) for pasting into the ASPI client
# simulator — or any other manual HTTP client.
#
# Why this exists: the other scripts in this directory send the request
# themselves, so their diagnostics are for debugging, not for copying. Here the
# body is emitted MINIFIED and byte-identical to what was signed, because
# X-SIGNATURE covers a SHA-256 of the exact body bytes. Re-indenting,
# re-ordering keys, or letting a client "prettify" the JSON before sending
# changes those bytes and the request fails with [Invalid signature].
#
# Usage:
#   ./scripts/aspi-simulator-request.sh -e <endpoint> -f <env-file> [options]
#
#   -e  one of: token | create-va | inquiry | payment | status | delete-va
#   -f  credentials file (.env.merchant.NAME for create-va/delete-va,
#       .env.<vendor>.<channel> for inquiry/payment/status; either works fo
#       token when the file carries a clientId + private key)
#   -u  base URL (default https://uatbca.manjo.co.id)
#   -s  partnerServiceId (default "   12345" — 8 chars, left-padded with spaces)
#   -c  customerNo (default: generated from the clock)
#   -v  virtualAccountNo (default: partnerServiceId + customerNo)
#   -n  virtualAccountName (create-va)
#   -a  amount (default 150000.00)
#   -t  trxId — for payment, pass the trxId returned by create-va
#   -q  paymentRequestId — for payment, pass the inquiry's inquiryRequestId
#   -r  inquiryRequestId (inquiry/status)
#
# The generated X-TIMESTAMP is only accepted within ±5 minutes of server time,
# and the accessToken expires after 15 minutes. Generate one request at a time,
# immediately before pasting it — don't prepare a batch in advance.
#
# Requires: curl, openssl, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="https://uatbca.manjo.co.id"
ENDPOINT_NAME=""
ENV_FILE=""
PARTNER_SERVICE_ID="   12345"
CUSTOMER_NO=""
VA_NO=""
VA_NAME="Simulator Test"
AMOUNT="150000.00"
TRX_ID=""
PAYMENT_REQUEST_ID=""
INQUIRY_REQUEST_ID=""
CHANNEL_ID="95231"
PARTNER_ID="1-MANJO-SNAP"

usage() {
	echo "Usage: $0 -e <token|create-va|inquiry|payment|status|delete-va> -f <env-file> [-u <base-url>] [-s <partnerServiceId>] [-c <customerNo>] [-v <virtualAccountNo>] [-n <name>] [-a <amount>] [-t <trxId>] [-q <paymentRequestId>] [-r <inquiryRequestId>]" >&2
	exit 1
}

# Mirrors vendor_config.go's parseEnvFile quote handling so a value written
# with or without quotes reads back the same either way.
read_env_var() {
	local file="$1" key="$2" line value
	line="$(grep -E "^${key}=" "$file" | tail -n1)" || return 1
	[[ -n "$line" ]] || return 1
	value="${line#*=}"
	if [[ "$value" == \"*\" && "$value" == *\" ]]; then
		value="${value:1:${#value}-2}"
	elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
		value="${value:1:${#value}-2}"
	fi
	printf '%s' "$value"
}

while getopts "e:f:u:s:c:v:n:a:t:q:r:i:p:h" opt; do
	case "$opt" in
	e) ENDPOINT_NAME="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	q) PAYMENT_REQUEST_ID="$OPTARG" ;;
	r) INQUIRY_REQUEST_ID="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$ENDPOINT_NAME" || -z "$ENV_FILE" ]] && usage
[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

# Credentials: accept either naming convention so one flag covers both roles.
CLIENT_ID="$(read_env_var "$ENV_FILE" MERCHANT_CLIENT_ID || read_env_var "$ENV_FILE" VENDOR_CLIENT_ID || true)"
CLIENT_SECRET="$(read_env_var "$ENV_FILE" MERCHANT_SECRET_VALUE || read_env_var "$ENV_FILE" VENDOR_CLIENT_SECRET || true)"
PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" MERCHANT_PRIVATE_KEY_PATH || read_env_var "$ENV_FILE" VENDOR_PRIVATE_KEY_PATH || true)"

[[ -z "$CLIENT_ID" ]] && { echo "!! ${ENV_FILE}: no MERCHANT_CLIENT_ID / VENDOR_CLIENT_ID" >&2; exit 1; }
[[ -z "$PRIVATE_KEY_PATH" || ! -f "$PRIVATE_KEY_PATH" ]] && { echo "!! private key not found (MERCHANT_PRIVATE_KEY_PATH / VENDOR_PRIVATE_KEY_PATH): ${PRIVATE_KEY_PATH:-<unset>}" >&2; exit 1; }

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
[[ -z "$CUSTOMER_NO" ]] && CUSTOMER_NO="$(date +%H%M%S)$((RANDOM % 90 + 10))"
[[ -z "$VA_NO" ]] && VA_NO="${PARTNER_SERVICE_ID}${CUSTOMER_NO}"
EXTERNAL_ID="$(date +%s)$((RANDOM % 9000 + 1000))"

emit() {
	local method="$1" url="$2" body="$3"
	shift 3
	echo "================================================================"
	echo "  ${method} ${url}"
	echo "================================================================"
	echo
	echo "--- HEADERS ---"
	printf '%s\n' "$@"
	echo
	echo "--- BODY (paste EXACTLY as-is — do not reformat) ---"
	echo "$body"
	echo
	echo "--- valid until $(date -d "+5 minutes" +%H:%M:%S 2>/dev/null || echo '~5 minutes from now') (X-TIMESTAMP skew window) ---"
}

# ---------------------------------------------------------------- token
if [[ "$ENDPOINT_NAME" == "token" ]]; then
	EP="/openapi/v1.0/access-token/b2b"
	BODY='{"grantType":"client_credentials","additionalInfo":{}}'
	# Asymmetric: SHA256withRSA over clientId|timestamp — no body involved.
	SIG="$(printf '%s' "${CLIENT_ID}|${TIMESTAMP}" | openssl dgst -sha256 -sign "$PRIVATE_KEY_PATH" | openssl base64 -A)"
	emit POST "${BASE_URL}${EP}" "$BODY" \
		"Content-Type: application/json" \
		"X-CLIENT-KEY: ${CLIENT_ID}" \
		"X-TIMESTAMP: ${TIMESTAMP}" \
		"X-SIGNATURE: ${SIG}"
	exit 0
fi

[[ -z "$CLIENT_SECRET" ]] && { echo "!! ${ENV_FILE}: no MERCHANT_SECRET_VALUE / VENDOR_CLIENT_SECRET — cannot sign a service request" >&2; exit 1; }

# Every service endpoint binds a real accessToken into stringToSign, so fetch
# one first. It must be the SAME token the request later presents in
# Authorization, which is why it is fetched here rather than reused from a
# previous run.
ACCESS_TOKEN="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$CLIENT_ID" -p "$PRIVATE_KEY_PATH" -u "$BASE_URL" 2>/dev/null | jq -r '.accessToken // empty')"
[[ -z "$ACCESS_TOKEN" ]] && { echo "!! failed to obtain an accessToken for ${CLIENT_ID} at ${BASE_URL}" >&2; exit 1; }

METHOD="POST"
case "$ENDPOINT_NAME" in
create-va)
	EP="/openapi/v1.0/transfer-va/create-va"
	[[ -z "$TRX_ID" ]] && TRX_ID="TRX-$(date +%s)$((RANDOM % 9000 + 1000))"
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg n "$VA_NAME" --arg t "$TRX_ID" --arg a "$AMOUNT" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,virtualAccountName:$n,trxId:$t,totalAmount:{value:$a,currency:"IDR"},virtualAccountTrxType:"C"}')"
	;;
inquiry)
	EP="/openapi/v1.0/transfer-va/inquiry"
	[[ -z "$INQUIRY_REQUEST_ID" ]] && INQUIRY_REQUEST_ID="INQ-$(date +%s)$((RANDOM % 9000 + 1000))"
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg d "$TIMESTAMP" --arg a "$AMOUNT" --arg r "$INQUIRY_REQUEST_ID" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,txnDateInit:$d,amount:{value:$a,currency:"IDR"},inquiryRequestId:$r}')"
	;;
payment)
	EP="/openapi/v1.0/transfer-va/payment"
	# paymentRequestId must equal the inquiry's inquiryRequestId when the
	# payment follows an inquiry (ASPI PaymentRequest); trxId is mandatory when
	# it follows a create-VA. Both come from the earlier responses via -q/-t.
	[[ -z "$PAYMENT_REQUEST_ID" ]] && PAYMENT_REQUEST_ID="PAY-$(date +%s)$((RANDOM % 9000 + 1000))"
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg t "$TRX_ID" --arg q "$PAYMENT_REQUEST_ID" --arg a "$AMOUNT" \
		--arg d "$TIMESTAMP" --arg n "R$(date +%s | tail -c 10)" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v}
		 + (if $t == "" then {} else {trxId:$t} end)
		 + {paymentRequestId:$q,paidAmount:{value:$a,currency:"IDR"},totalAmount:{value:$a,currency:"IDR"},trxDateTime:$d,referenceNo:$n}')"
	;;
status)
	EP="/openapi/v1.0/transfer-va/status"
	[[ -z "$INQUIRY_REQUEST_ID" ]] && { echo "!! -r <inquiryRequestId> is required for status" >&2; exit 1; }
	[[ -z "$PAYMENT_REQUEST_ID" ]] && PAYMENT_REQUEST_ID="$INQUIRY_REQUEST_ID"
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg r "$INQUIRY_REQUEST_ID" --arg q "$PAYMENT_REQUEST_ID" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,inquiryRequestId:$r,paymentRequestId:$q}')"
	;;
delete-va)
	METHOD="DELETE"
	EP="/openapi/v1.0/transfer-va/delete-va"
	[[ -z "$TRX_ID" ]] && TRX_ID="TRX-$(date +%s)$((RANDOM % 9000 + 1000))"
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" --arg t "$TRX_ID" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,trxId:$t}')"
	;;
*)
	echo "!! unknown endpoint: ${ENDPOINT_NAME}" >&2
	usage
	;;
esac

# SNAP symmetric signature over the exact minified body emitted below.
BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -binary | openssl base64 -A)"
STRING_TO_SIGN="${METHOD}:${EP}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

emit "$METHOD" "${BASE_URL}${EP}" "$BODY" \
	"Content-Type: application/json" \
	"Authorization: Bearer ${ACCESS_TOKEN}" \
	"X-TIMESTAMP: ${TIMESTAMP}" \
	"X-SIGNATURE: ${SIGNATURE}" \
	"X-PARTNER-ID: ${PARTNER_ID}" \
	"X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	"CHANNEL-ID: ${CHANNEL_ID}" \
	"X-CLIENT-KEY: ${CLIENT_ID}"

echo
echo "stringToSign (for debugging a signature mismatch):"
echo "${STRING_TO_SIGN}"
