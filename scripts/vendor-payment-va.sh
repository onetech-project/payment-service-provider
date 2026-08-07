#!/usr/bin/env bash
#
# Vendor-side: POST /openapi/v1.0/transfer-va/payment
# Simulates the switching vendor notifying this PSP that a customer paid a
# VA. On success, the PSP looks up the notificationUrl registered via
# merchant-create-va.sh and asynchronously calls the merchant back
# (internal/usecase/va_usecase.go notifyMerchant -> Asynq queue ->
# internal/adapter/delivery/worker/payment_notification_worker.go).
#
# Protected by SNAPAuthMiddleware — verified via HMAC-SHA512 (feature
# 009-transfer-va-auth).
#
# Auth (feature 011-vendor-access-token-signature): for vendors migrated to
# ClientID-based onboarding (VENDOR_CLIENT_ID set in the vendor's
# .env.<vendor>.<channel>), this endpoint ALSO requires a valid accessToken
# (Authorization: Bearer), bound into the AccessToken component of
# stringToSign. Non-migrated vendors keep the legacy empty-AccessToken
# convention unchanged (see vendor-inquiry-va.sh for the equivalent flow).
#
# Usage:
#   ./scripts/vendor-payment-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> \
#       -a <amount> (-e <client-secret> | -f <env-file>) [-o <access-token>] [-i <channel-id>] [-p <partner-id>] [-n <virtualAccountName>] [-C <channelCode>] [-A <flagAdvise>] [-u <base-url>]
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
ENDPOINT="/openapi/v1.0/transfer-va/payment"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
AMOUNT="100000.00"
CLIENT_SECRET=""
ENV_FILE=""
CHANNEL_ID="95231"
PARTNER_ID="111111"
ACCESS_TOKEN=""
TRX_ID=""
PAYMENT_REQUEST_ID=""
# Fields BCA's PaymentRequest table marks Mandatory (Y) that the wider SNAP
# standard leaves optional. A vendor configured with
# VENDOR_STRICT_MANDATORY_FIELDS=true — which is what .env.bca.va sets, and
# therefore what BCA conformance actually looks like — rejects a payment
# without them (4002502). They were absent here, so this script could only
# ever drive a NON-BCA-conformant vendor.
VA_NAME="Payer Name"
CHANNEL_CODE="6011"
FLAG_ADVISE="N"

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> -a <amount> (-e <client-secret> | -f <env-file>) [-o <access-token>] [-t <trxId>] [-q <paymentRequestId>] [-i <channel-id>] [-p <partner-id>] [-n <virtualAccountName>] [-C <channelCode>] [-A <flagAdvise>] [-u <base-url>]" >&2
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

while getopts "s:c:v:a:e:f:o:t:q:i:p:u:n:C:A:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	o) ACCESS_TOKEN="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	q) PAYMENT_REQUEST_ID="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	C) CHANNEL_CODE="$OPTARG" ;;
	A) FLAG_ADVISE="$OPTARG" ;;
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
# $(date +%s) alone is only second-resolution — two payment calls against the
# same VA within the same second (e.g. a fast retry/e2e test) would otherwise
# collide onto the identical paymentRequestId and get silently treated as an
# idempotent replay of the SAME payment instead of a genuinely new attempt.
# $RANDOM makes each run's id unique regardless of timing (payment_request_id
# column is VARCHAR(30); "PAY-" + 10-digit epoch + up to 5-digit $RANDOM fits).
[[ -z "$PAYMENT_REQUEST_ID" ]] && PAYMENT_REQUEST_ID="PAY-$(date +%s)$RANDOM"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"
TRX_DATE="$(date +%Y-%m-%dT%H:%M:%S%:z)"
# reference_no column is varchar(11) — keep it short
REFERENCE_NO="R$(date +%s | tail -c 10)"

# Fields follow the ASPI PaymentRequest table (Transfer Kredit > Virtual
# Account, service code 25):
#   - paymentRequestId (M): "If Payment comes from the Inquiry process, this
#     value must be the same with inquiryRequestId" — pass the inquiry's id
#     via -q for that flow.
#   - trxId (C): "Mandatory if Payment comes from the Create VA Request" —
#     pass the create-va trxId via -t. Omitted entirely when it doesn't
#     apply, rather than filling in a locally invented id.
#   - totalAmount (O): only checked for mismatch when present.
#   - inquiryRequestId is NOT a PaymentRequest field in the spec (it only
#     appears in paymentRequestId's description) and transactionDate does not
#     exist on this endpoint — only trxDateTime does. Both are omitted.
TRX_ID_FIELD=""
[[ -n "$TRX_ID" ]] && TRX_ID_FIELD="\"trxId\": \"${TRX_ID}\","

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "virtualAccountName": "${VA_NAME}",
  ${TRX_ID_FIELD}
  "paymentRequestId": "${PAYMENT_REQUEST_ID}",
  "channelCode": ${CHANNEL_CODE},
  "flagAdvise": "${FLAG_ADVISE}",
  "paidAmount": {"value": "${AMOUNT}", "currency": "IDR"},
  "totalAmount": {"value": "${AMOUNT}", "currency": "IDR"},
  "trxDateTime": "${TRX_DATE}",
  "referenceNo": "${REFERENCE_NO}"
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
#
# `jq -cj .` is the MinifyJson step, and it is load-bearing: BCA specifies
# SHA-256(MinifyJson(RequestBody)), and the server hashes the minified body
# (crypto.HashRequestBody). $BODY here is pretty-printed, so hashing it raw
# produced a different digest than the server computed and every request came
# back 401 "Unauthorized. [Signature]". -j (not just -c) suppresses jq's
# trailing newline, which would otherwise be hashed too.
BODY_HASH="$(printf '%s' "$BODY" | jq -cj . | openssl dgst -sha256 -binary | ${BODY_HASH_ENCODER:-xxd -p -c 256})"
STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-flow.sh).
echo "==> POST ${BASE_URL}${ENDPOINT}" >&2
[[ -n "$ACCESS_TOKEN" ]] && echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo "==> paymentRequestId: ${PAYMENT_REQUEST_ID}" >&2
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

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	"${AUTH_HEADER[@]}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)

echo >&2
echo "==> If a VA with this virtualAccountNo was created via merchant-create-va.sh" >&2
echo "    (with a notificationUrl), a callback should have been enqueued to Asynq" >&2
echo "    and delivered by the payment_notification_worker shortly after this call." >&2
