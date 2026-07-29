#!/usr/bin/env bash
#
# End-to-end test of the static/dynamic VA feature's DYNAMIC side (feature
# 006-static-dynamic-va): dynamic no bill (vaType 04), dynamic fixed bill
# (vaType 06), and dynamic variable bill (vaType 05). For all three,
# customerNo is left empty in the create-va request and the server assigns a
# system-generated, sequential 20-digit customerNo (2-digit vaType +
# 18-digit sequence) — this script verifies that value comes back and then
# exercises the type-specific payment behavior:
#
#   - No bill (04):      any payment amount is accepted at payment time.
#   - Fixed bill (06):   a single payment for the exact totalAmount pays it off.
#   - Variable bill (05): TWO partial payments are sent; the VA must stay
#                          pending after the first and flip to fully paid
#                          ("lunas") only once the cumulative total reaches
#                          totalAmount (FR-013/SC-006).
#
# Chains together, per VA type:
#   1. curl-b2b-token.sh      POST /openapi/v1.0/access-token/b2b        (once, shared across all 3 types)
#   2. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va   (-y <vaType>, customerNo empty)
#   3. vendor-inquiry-va.sh   POST /openapi/v1.0/transfer-va/inquiry     (using the server-assigned customerNo)
#   4. vendor-payment-va.sh  x1 (no bill/fixed bill) or x2 (variable bill)
#
# Usage:
#   ./scripts/e2e-dynamic-va-flow.sh -i <client_id> -k <private_key.pem> \
#       (-e <client-secret> | -f <env-file>) [-u <base-url>]
#
# -i/-k are for the B2B access-token call (asymmetric RSA signing, see
# curl-b2b-token.sh). -e/-f are for the create-va/inquiry/payment HMAC signing
# (see merchant-create-va.sh / vendor-inquiry-va.sh / vendor-payment-va.sh) —
# -f loads VENDOR_CLIENT_SECRET straight out of a .env.<vendor>.<channel> file.
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
CLIENT_ID=""
PRIVATE_KEY_PATH=""
CLIENT_SECRET=""
ENV_FILE=""

usage() {
	echo "Usage: $0 -i <client_id> -k <private_key.pem> (-e <client-secret> | -f <env-file>) [-u <base-url>]" >&2
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

while getopts "i:k:e:f:u:h" opt; do
	case "$opt" in
	i) CLIENT_ID="$OPTARG" ;;
	k) PRIVATE_KEY_PATH="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$CLIENT_SECRET" ]] && CLIENT_SECRET="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_SECRET || true)"
	[[ -z "$CLIENT_ID" ]] && CLIENT_ID="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_ID || true)"
	[[ -z "$PRIVATE_KEY_PATH" ]] && PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" VENDOR_PRIVATE_KEY_PATH || true)"

	[[ -z "$CLIENT_SECRET" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_SECRET is empty — fill it in, or pass -e <client-secret> directly." >&2
	[[ -z "$CLIENT_ID" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_ID is empty — fill it in, or pass -i <client-id> directly." >&2
	[[ -z "$PRIVATE_KEY_PATH" ]] && echo "!! ${ENV_FILE}: VENDOR_PRIVATE_KEY_PATH is empty — fill it in, or pass -k <private-key.pem> directly." >&2
fi

[[ -z "$CLIENT_ID" || -z "$PRIVATE_KEY_PATH" || -z "$CLIENT_SECRET" ]] && usage
command -v jq >/dev/null || { echo "jq is required for this script" >&2; exit 1; }

PASS_COUNT=0
FAIL_COUNT=0

check() {
	local label="$1" condition="$2"
	if [[ "$condition" == "true" ]]; then
		echo "   [PASS] ${label}"
		PASS_COUNT=$((PASS_COUNT + 1))
	else
		echo "   [FAIL] ${label}" >&2
		FAIL_COUNT=$((FAIL_COUNT + 1))
	fi
}

echo "=================================================================="
echo "Step 0: POST /openapi/v1.0/access-token/b2b"
echo "=================================================================="
TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$CLIENT_ID" -p "$PRIVATE_KEY_PATH" -u "$BASE_URL" || true)"
echo "$TOKEN_RESPONSE" | jq . 2>/dev/null || echo "$TOKEN_RESPONSE"

ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty' 2>/dev/null || true)"
if [[ -z "$ACCESS_TOKEN" ]]; then
	# The B2B client may not be registered/onboarded in this environment's
	# database (e.g. a fresh/shared test deployment) — the merchant create-va/
	# inquiry/payment endpoints exercised below don't require a valid
	# accessToken to be present for this feature's own logic to run, so this
	# is a warning, not a hard failure. If your deployment DOES enforce SNAP
	# auth on those routes, this run will instead fail loudly at Test 1 below.
	echo "!! Could not obtain accessToken (client not onboarded in this environment?) — continuing without one." >&2
else
	echo "==> accessToken acquired: ${ACCESS_TOKEN:0:12}..."
fi
echo

RUN_ID="$(date +%s)$RANDOM"

# ------------------------------------------------------------------
# Test 1: Dynamic No Bill — partnerServiceId 15973, vaType 04
# ------------------------------------------------------------------
echo "=================================================================="
echo "Test 1/3: Dynamic No Bill (partnerServiceId=15973, vaType=04)"
echo "=================================================================="
CREATE_RESP_1="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15973 -n "Dynamic NoBill ${RUN_ID}" -y 04 -t "trx-dyn-nobill-${RUN_ID}" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$CREATE_RESP_1" | jq .

RESP_CODE_1="$(echo "$CREATE_RESP_1" | jq -r '.responseCode // empty')"
CUSTOMER_NO_1="$(echo "$CREATE_RESP_1" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_1="$(echo "$CREATE_RESP_1" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_1" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 20 digits starting with 04" "$([[ "$CUSTOMER_NO_1" =~ ^04[0-9]{18}$ ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_1" == "15973${CUSTOMER_NO_1}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_1}"
echo "==> derived virtualAccountNo: ${VA_NO_1}"
echo

echo "--- inquiry ---"
INQUIRY_RESP_1="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15973 -c "$CUSTOMER_NO_1" -v "$VA_NO_1" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$INQUIRY_RESP_1" | jq .
echo

echo "--- payment (any amount accepted for no-bill) ---"
PAYMENT_RESP_1="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15973 -c "$CUSTOMER_NO_1" -v "$VA_NO_1" -a "77000.00" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$PAYMENT_RESP_1" | jq .
PAY_CODE_1="$(echo "$PAYMENT_RESP_1" | jq -r '.responseCode // empty')"
check "payment succeeded (responseCode 2xx)" "$([[ "$PAY_CODE_1" == 2* ]] && echo true || echo false)"
echo

# ------------------------------------------------------------------
# Test 2: Dynamic Fixed Bill — partnerServiceId 15975, vaType 06
# ------------------------------------------------------------------
echo "=================================================================="
echo "Test 2/3: Dynamic Fixed Bill (partnerServiceId=15975, vaType=06)"
echo "=================================================================="
FIXED_AMOUNT="150000.00"
CREATE_RESP_2="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15975 -n "Dynamic Fixed ${RUN_ID}" -y 06 -a "$FIXED_AMOUNT" -t "trx-dyn-fixed-${RUN_ID}" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$CREATE_RESP_2" | jq .

RESP_CODE_2="$(echo "$CREATE_RESP_2" | jq -r '.responseCode // empty')"
CUSTOMER_NO_2="$(echo "$CREATE_RESP_2" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_2="$(echo "$CREATE_RESP_2" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_2" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 20 digits starting with 06" "$([[ "$CUSTOMER_NO_2" =~ ^06[0-9]{18}$ ]] && echo true || echo false)"
check "customerNo differs from Test 1's (per-vaType sequence, not shared)" "$([[ "$CUSTOMER_NO_2" != "$CUSTOMER_NO_1" ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_2" == "15975${CUSTOMER_NO_2}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_2}"
echo "==> derived virtualAccountNo: ${VA_NO_2}"
echo

echo "--- inquiry ---"
INQUIRY_RESP_2="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15975 -c "$CUSTOMER_NO_2" -v "$VA_NO_2" -a "$FIXED_AMOUNT" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$INQUIRY_RESP_2" | jq .
echo

echo "--- payment (exact fixed amount) ---"
PAYMENT_RESP_2="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15975 -c "$CUSTOMER_NO_2" -v "$VA_NO_2" -a "$FIXED_AMOUNT" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$PAYMENT_RESP_2" | jq .
PAY_CODE_2="$(echo "$PAYMENT_RESP_2" | jq -r '.responseCode // empty')"
PAY_STATUS_2="$(echo "$PAYMENT_RESP_2" | jq -r '.virtualAccountData.paymentFlagStatus // empty')"
check "payment succeeded (responseCode 2xx)" "$([[ "$PAY_CODE_2" == 2* ]] && echo true || echo false)"
check "paymentFlagStatus is 00 (paid) after the single exact payment" "$([[ "$PAY_STATUS_2" == "00" ]] && echo true || echo false)"
echo

# ------------------------------------------------------------------
# Test 3: Dynamic Variable Bill — partnerServiceId 15974, vaType 05
# ------------------------------------------------------------------
echo "=================================================================="
echo "Test 3/3: Dynamic Variable Bill (partnerServiceId=15974, vaType=05)"
echo "=================================================================="
TARGET_AMOUNT="100000.00"
CREATE_RESP_3="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15974 -n "Dynamic Variable ${RUN_ID}" -y 05 -a "$TARGET_AMOUNT" -t "trx-dyn-variable-${RUN_ID}" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$CREATE_RESP_3" | jq .

RESP_CODE_3="$(echo "$CREATE_RESP_3" | jq -r '.responseCode // empty')"
CUSTOMER_NO_3="$(echo "$CREATE_RESP_3" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_3="$(echo "$CREATE_RESP_3" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_3" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 20 digits starting with 05" "$([[ "$CUSTOMER_NO_3" =~ ^05[0-9]{18}$ ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_3" == "15974${CUSTOMER_NO_3}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_3}"
echo "==> derived virtualAccountNo: ${VA_NO_3}"
echo

echo "--- inquiry ---"
INQUIRY_RESP_3="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "$TARGET_AMOUNT" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$INQUIRY_RESP_3" | jq .
echo

echo "--- payment 1/2: partial payment (60000.00 of 100000.00 target) ---"
PAYMENT_RESP_3A="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "60000.00" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$PAYMENT_RESP_3A" | jq .
PAY_CODE_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.responseCode // empty')"
PAY_STATUS_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.virtualAccountData.paymentFlagStatus // empty')"
PAID_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.virtualAccountData.paidAmount.value // empty')"
check "partial payment succeeded (responseCode 2xx)" "$([[ "$PAY_CODE_3A" == 2* ]] && echo true || echo false)"
check "paymentFlagStatus stays 03 (pending) after partial payment" "$([[ "$PAY_STATUS_3A" == "03" ]] && echo true || echo false)"
check "cumulative paidAmount reflects the first payment (60000.00)" "$([[ "$PAID_3A" == "60000.00" ]] && echo true || echo false)"
echo

echo "--- payment 2/2: remaining payment (40000.00, reaches 100000.00 target) ---"
PAYMENT_RESP_3B="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "40000.00" -e "$CLIENT_SECRET" -u "$BASE_URL")"
echo "$PAYMENT_RESP_3B" | jq .
PAY_CODE_3B="$(echo "$PAYMENT_RESP_3B" | jq -r '.responseCode // empty')"
PAY_STATUS_3B="$(echo "$PAYMENT_RESP_3B" | jq -r '.virtualAccountData.paymentFlagStatus // empty')"
PAID_3B="$(echo "$PAYMENT_RESP_3B" | jq -r '.virtualAccountData.paidAmount.value // empty')"
check "second payment succeeded (responseCode 2xx)" "$([[ "$PAY_CODE_3B" == 2* ]] && echo true || echo false)"
check "paymentFlagStatus flips to 00 (lunas) once cumulative total is reached" "$([[ "$PAY_STATUS_3B" == "00" ]] && echo true || echo false)"
check "cumulative paidAmount reflects both payments (100000.00)" "$([[ "$PAID_3B" == "100000.00" ]] && echo true || echo false)"
echo

echo "=================================================================="
echo "Summary: ${PASS_COUNT} passed, ${FAIL_COUNT} failed"
echo "=================================================================="
[[ "$FAIL_COUNT" -eq 0 ]]
