#!/usr/bin/env bash
#
# Merchant-side: POST /openapi/v1.0/transfer-va/create-va (Service Code 27)
# Creates a new Virtual Account. virtualAccountNo is client-supplied per ASPI
# VAIdentity (not server-generated). The payment callback URL is a proprietary
# extension — there is no top-level notificationUrl field in VAUpsertRequest —
# so it is sent via additionalInfo.dbUrlProcess, the extension slot the spec
# itself defines for this endpoint (aspi-open-api-va.yaml:317-320). See
# vendor-payment-va.sh for the callback flow this registers.
#
# Usage:
#   ./scripts/merchant-create-va.sh -s <partnerServiceId> -c <customerNo> \
#       -n <virtualAccountName> -a <amount> (-e <secret> -o <access-token> | -f <.env.merchant.NAME>) \
#       [-w <notificationUrl>] [-v <virtualAccountNo>] [-t <trxId>] [-i <channel-id>] \
#       [-p <partner-id>] [-b <billNo>] [-d <billName>] \
#       [-y <vaType>] [-x <expiredDate>] [-u <base-url>]
#
# -x <expiredDate> sets the top-level expiredDate field (ISO-8601, e.g.
# 2026-07-28T10:15:00+07:00) used by feature 007-merchant-expiry-callback's
# on-access expiry detection (VAUsecase.Inquiry/Payment). Omit for a VA that
# never expires via that mechanism.
#
# Auth (feature 009-transfer-va-auth + 010-merchant-hmac-signature): this
# endpoint requires BOTH a valid accessToken (Authorization: Bearer) AND a
# valid X-SIGNATURE, computed with the merchant's shared secret. Two ways to
# supply these:
#   -f <.env.merchant.NAME>  Preferred. A credentials file produced by
#                            onboard-merchant.sh (MERCHANT_CLIENT_ID,
#                            MERCHANT_PRIVATE_KEY_PATH, MERCHANT_SECRET_VALUE).
#                            This script fetches a fresh accessToken
#                            automatically via curl-b2b-token.sh — no need to
#                            pass -o yourself.
#   -e <secret> -o <token>   Manual: pass the merchant's shared secret and an
#                            already-obtained accessToken directly.
# This merchant secret is a DIFFERENT credential from a vendor's
# VENDOR_CLIENT_SECRET — never reuse a .env.<vendor>.<channel> file here.
#
# -b <billNo> attaches one billDetails entry (billNo + billAmount = -a; -d sets
# billName) so create-va actually exercises bill-detail persistence
# (internal/infrastructure/database/va_repository.go SaveBillDetails) instead
# of always sending an empty billDetails array. Omit -b for a VA with no bills.
#
# -y <vaType> sets additionalInfo.vaType per feature 006-static-dynamic-va's
# six static/dynamic VA type combinations (01=static no bill, 02=static
# variable bill, 03=static fixed bill, 04=dynamic no bill, 05=dynamic variable
# bill, 06=dynamic fixed bill). When -y is 04/05/06 (dynamic), -c/<customerNo>
# is NOT required — the server generates it, and this script echoes the
# server-assigned value back on stdout diagnostics (see
# e2e-dynamic-va-flow.sh). "No bill" types (01/04) must NOT send totalAmount
# per FR-012, so -a is silently omitted from the request body in that case;
# pass -a explicitly for 02/03/05/06 (variable/fixed bill).
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
ENDPOINT="/openapi/v1.0/transfer-va/create-va"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NAME=""
AMOUNT="100000.00"
NOTIFICATION_URL=""
TRX_ID=""
VA_NO=""
BILL_NO=""
BILL_NAME=""
VA_TYPE=""
EXPIRED_DATE=""
CLIENT_SECRET=""
ENV_FILE=""
CHANNEL_ID="95231"
PARTNER_ID="111111"
ACCESS_TOKEN=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -a <amount> (-e <secret> -o <access-token> | -f <.env.merchant.NAME>) [-w <notificationUrl>] [-v <virtualAccountNo>] [-t <trxId>] [-i <channel-id>] [-p <partner-id>] [-b <billNo>] [-d <billName>] [-y <vaType>] [-x <expiredDate>] [-u <base-url>]" >&2
	exit 1
}

# read_env_var extracts KEY=value from a .env.<vendor>.<channel> or
# .env.merchant.<name> file, stripping surrounding quotes the same way
# vendor_config.go's parseEnvFile does.
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

while getopts "s:c:n:a:w:v:t:e:f:i:p:o:b:d:y:x:u:h" opt; do
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
	y) VA_TYPE="$OPTARG" ;;
	x) EXPIRED_DATE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$CLIENT_SECRET" ]] && CLIENT_SECRET="$(read_env_var "$ENV_FILE" MERCHANT_SECRET_VALUE || true)"
	[[ -z "$CLIENT_SECRET" ]] && echo "!! ${ENV_FILE}: MERCHANT_SECRET_VALUE is empty — fill it in, or pass -e <secret> directly." >&2

	if [[ -z "$ACCESS_TOKEN" ]]; then
		MERCHANT_CLIENT_ID="$(read_env_var "$ENV_FILE" MERCHANT_CLIENT_ID || true)"
		MERCHANT_PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" MERCHANT_PRIVATE_KEY_PATH || true)"
		if [[ -n "$MERCHANT_CLIENT_ID" && -n "$MERCHANT_PRIVATE_KEY_PATH" ]]; then
			echo "==> Fetching accessToken for merchant client ${MERCHANT_CLIENT_ID}..." >&2
			TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$MERCHANT_CLIENT_ID" -p "$MERCHANT_PRIVATE_KEY_PATH" -u "$BASE_URL")"
			ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty' 2>/dev/null || true)"
			[[ -z "$ACCESS_TOKEN" ]] && { echo "!! Failed to obtain accessToken for ${MERCHANT_CLIENT_ID} — aborting." >&2; exit 1; }
		else
			echo "!! ${ENV_FILE}: MERCHANT_CLIENT_ID/MERCHANT_PRIVATE_KEY_PATH missing — cannot auto-fetch accessToken; pass -o <access-token> directly." >&2
		fi
	fi
fi

# X-CLIENT-KEY is intentionally not requested/sent here — per ASPI spec it's
# only used on the access-token endpoint, never on transfer-va endpoints
# (aspi-open-api-va.yaml has no XClientKey parameter on any transfer-va path).
# customerNo is required EXCEPT for dynamic vaType (04/05/06, feature
# 006-static-dynamic-va), where the server generates it and the request must
# send it empty.
IS_DYNAMIC_VA_TYPE=false
case "$VA_TYPE" in
04 | 05 | 06) IS_DYNAMIC_VA_TYPE=true ;;
esac
if [[ "$IS_DYNAMIC_VA_TYPE" == false ]]; then
	[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NAME" || -z "$CLIENT_SECRET" || -z "$ACCESS_TOKEN" ]] && usage
else
	[[ -n "$CUSTOMER_NO" ]] && { echo "!! -y ${VA_TYPE} is a dynamic vaType — customerNo must be empty (omit -c); the server assigns it." >&2; exit 1; }
	[[ -z "$PARTNER_SERVICE_ID" || -z "$VA_NAME" || -z "$CLIENT_SECRET" || -z "$ACCESS_TOKEN" ]] && usage
fi
# trxId becomes the row's inquiry_request_id, which is UNIQUE across the
# WHOLE va_transactions table (not just per-VA) — $(date +%s) alone would
# collide if two different VAs are created within the same second (e.g. two
# calls in one e2e script run), silently clobbering the first VA's row
# instead of creating a second one. $RANDOM avoids that regardless of timing.
[[ -z "$TRX_ID" ]] && TRX_ID="TRX-$(date +%s)$RANDOM"
# virtualAccountNo is mandatory (and MUST equal partnerServiceId+customerNo,
# per feature 008-va-number-consistency) for static vaType and legacy
# requests; default to that concatenation for convenience when omitted.
# For dynamic vaType, virtualAccountNo is OPTIONAL: leave -v unset to have the
# server derive it from partnerServiceId + the customerNo it generates, or
# pass -v to use your own value as-is (still subject to length/conflict
# checks server-side).
if [[ -z "$VA_NO" && "$IS_DYNAMIC_VA_TYPE" != true ]]; then
	VA_NO="${PARTNER_SERVICE_ID}${CUSTOMER_NO}"
fi

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
EXTERNAL_ID="$(date +%Y%m%d%H%M%S)$RANDOM"

# additionalInfo carries both the callback URL (dbUrlProcess, ASPI's own
# extension slot for VAUpsertRequest) and vaType (feature
# 006-static-dynamic-va) — merged into one object since a request may set
# either, both, or neither.
ADDITIONAL_INFO_PARTS=()
[[ -n "$NOTIFICATION_URL" ]] && ADDITIONAL_INFO_PARTS+=("\"dbUrlProcess\": \"${NOTIFICATION_URL}\"")
[[ -n "$VA_TYPE" ]] && ADDITIONAL_INFO_PARTS+=("\"vaType\": \"${VA_TYPE}\"")
ADDITIONAL_INFO_FIELD=""
if [[ ${#ADDITIONAL_INFO_PARTS[@]} -gt 0 ]]; then
	IFS=,
	ADDITIONAL_INFO_FIELD="\"additionalInfo\": {${ADDITIONAL_INFO_PARTS[*]}},"
	unset IFS
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

# "No bill" vaType (01/04, feature 006-static-dynamic-va FR-012) MUST NOT
# carry totalAmount — omit the field entirely rather than sending a zero/empty
# value, which the server would still treat as present.
TOTAL_AMOUNT_FIELD="\"totalAmount\": {\"value\": \"${AMOUNT}\", \"currency\": \"IDR\"},"
if [[ "$VA_TYPE" == "01" || "$VA_TYPE" == "04" ]]; then
	TOTAL_AMOUNT_FIELD=""
fi

# expiredDate is a top-level field on MerchantCreateVARequest
# (internal/domain/va.go:389) — only sent when -x is given, so a plain
# create-va call still sends a spec-valid payload without one.
EXPIRED_DATE_FIELD=""
[[ -n "$EXPIRED_DATE" ]] && EXPIRED_DATE_FIELD="\"expiredDate\": \"${EXPIRED_DATE}\","

BODY=$(cat <<JSON
{
  "partnerServiceId": "${PARTNER_SERVICE_ID}",
  "customerNo": "${CUSTOMER_NO}",
  "virtualAccountNo": "${VA_NO}",
  "virtualAccountName": "${VA_NAME}",
  "trxId": "${TRX_ID}",
  ${ADDITIONAL_INFO_FIELD}
  ${BILL_DETAILS_FIELD}
  ${EXPIRED_DATE_FIELD}
  ${TOTAL_AMOUNT_FIELD}
  "virtualAccountTrxType": "C"
}
JSON
)

# SNAP symmetric signature: HMAC_SHA512(clientSecret, stringToSign)
# stringToSign = HTTPMethod:EndpointUrl:AccessToken:Base64(SHA-256(minify(body))):Timestamp
# bodyHash/signature are base64-encoded (feature 012-base64-hash-encoding), not hex.
# `jq -cj .` is the MinifyJson step and is load-bearing: the server hashes the
# minified body, so hashing $BODY raw (it is pretty-printed here) yields a
# different digest and every request comes back 401. -j (not just -c)
# suppresses jq's trailing newline, which would otherwise be hashed too.
BODY_HASH="$(printf '%s' "$BODY" | jq -cj . | openssl dgst -sha256 -binary | openssl base64 -A)"
STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-flow.sh).
# Authorization and stringToSign are echoed unconditionally: unlike the vendor
# scripts (where a legacy vendor genuinely has no token), an accessToken is
# mandatory here — the usage check above already aborted without one. Printing
# both makes an -O transcript self-contained: the token in the Authorization
# header must match the one embedded in stringToSign, and seeing them side by
# side is what makes a signature mismatch diagnosable.
echo "==> POST ${BASE_URL}${ENDPOINT}" >&2
echo "==> virtualAccountNo: ${VA_NO}" >&2
echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> stringToSign: $STRING_TO_SIGN" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo "==> Request body:" >&2
echo "$BODY" | (command -v jq >/dev/null && jq . || cat) >&2
echo >&2

AUTH_HEADER=()
if [[ -n "$ACCESS_TOKEN" ]]; then
	AUTH_HEADER=(-H "Authorization: Bearer ${ACCESS_TOKEN}")
fi

# Merchant-facing endpoints (create-va/list/delete-va) require a valid
# accessToken bearer header (feature 009-transfer-va-auth) — this is
# separate from ACCESS_TOKEN's other role as a component of stringToSign
# above (a SNAP convention that predates and is unrelated to this header).
curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-H "Idempotency-Key: $(uuidgen)" \
	"${AUTH_HEADER[@]}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
