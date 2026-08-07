#!/usr/bin/env bash
#
# End-to-end test of the static/dynamic VA feature's DYNAMIC side (feature
# 006-static-dynamic-va): dynamic no bill (vaType 04), dynamic fixed bill
# (vaType 06), and dynamic variable bill (vaType 05). For all three,
# customerNo is left empty in the create-va request and the server assigns a
# system-generated, sequential 18-digit customerNo (2-digit vaType +
# 16-digit sequence) — this script verifies that value comes back and then
# exercises the type-specific payment behavior:
#
#   - No bill (04):      any payment amount is accepted at payment time.
#   - Fixed bill (06):   a single payment for the exact totalAmount pays it off.
#   - Variable bill (05): TWO partial payments are sent; the VA must stay
#                          pending after the first and flip to fully paid
#                          ("lunas") only once the cumulative total reaches
#                          totalAmount (FR-013/SC-006).
#
# This exercises BOTH sides of the auth model (features 009/010), which use
# two INDEPENDENT identities/credentials — a vendor never needs a merchant's
# credentials or vice versa:
#   - Merchant side (create-va): accessToken (Bearer) + HMAC signature using
#     a merchant's own shared secret. See onboard-merchant.sh.
#   - Vendor side (inquiry/payment): HMAC signature only, using a vendor's
#     own shared secret. See onboard-vendor.sh.
#
# Chains together, per VA type:
#   1. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va   (-y <vaType>, customerNo empty)
#   2. vendor-inquiry-va.sh   POST /openapi/v1.0/transfer-va/inquiry     (using the server-assigned customerNo)
#   3. vendor-payment-va.sh  x1 (no bill/fixed bill) or x2 (variable bill)
#
# Usage:
#   ./scripts/e2e-dynamic-va-flow.sh -m <.env.merchant.NAME> -f <.env.vendor.channel> [-u <base-url>]
#
# -m is passed straight through to merchant-create-va.sh's -f.
# -f is passed straight through to vendor-inquiry-va.sh / vendor-payment-va.sh's -f.
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
MERCHANT_ENV_FILE=""
VENDOR_ENV_FILE=""

usage() {
	echo "Usage: $0 -m <.env.merchant.NAME> -f <.env.vendor.channel> [-u <base-url>]" >&2
	exit 1
}

while getopts "m:f:u:h" opt; do
	case "$opt" in
	m) MERCHANT_ENV_FILE="$OPTARG" ;;
	f) VENDOR_ENV_FILE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$MERCHANT_ENV_FILE" || -z "$VENDOR_ENV_FILE" ]] && usage
[[ -f "$MERCHANT_ENV_FILE" ]] || { echo "merchant env file not found: $MERCHANT_ENV_FILE (run onboard-merchant.sh first)" >&2; exit 1; }
[[ -f "$VENDOR_ENV_FILE" ]] || { echo "vendor env file not found: $VENDOR_ENV_FILE (run onboard-vendor.sh first)" >&2; exit 1; }
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

RUN_ID="$(date +%s)$RANDOM"

# ------------------------------------------------------------------
# Test 1: Dynamic No Bill — partnerServiceId 15973, vaType 04
# ------------------------------------------------------------------
echo "=================================================================="
echo "Test 1/3: Dynamic No Bill (partnerServiceId=15973, vaType=04)"
echo "=================================================================="
CREATE_RESP_1="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15973 -n "Dyn NoBill ${RUN_ID}" -y 04 -t "trx-dyn-nobill-${RUN_ID}" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$CREATE_RESP_1" | jq .

RESP_CODE_1="$(echo "$CREATE_RESP_1" | jq -r '.responseCode // empty')"
CUSTOMER_NO_1="$(echo "$CREATE_RESP_1" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_1="$(echo "$CREATE_RESP_1" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_1" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 18 digits starting with 04" "$([[ "$CUSTOMER_NO_1" =~ ^04[0-9]{16}$ ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_1" == "15973${CUSTOMER_NO_1}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_1}"
echo "==> derived virtualAccountNo: ${VA_NO_1}"
echo

echo "--- inquiry (vendor identity) ---"
INQUIRY_RESP_1="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15973 -c "$CUSTOMER_NO_1" -v "$VA_NO_1" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESP_1" | jq .
echo

echo "--- payment (any amount accepted for no-bill; vendor identity) ---"
PAYMENT_RESP_1="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15973 -c "$CUSTOMER_NO_1" -v "$VA_NO_1" -a "77000.00" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
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
CREATE_RESP_2="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15975 -n "Dyn Fixed ${RUN_ID}" -y 06 -a "$FIXED_AMOUNT" -t "trx-dyn-fixed-${RUN_ID}" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$CREATE_RESP_2" | jq .

RESP_CODE_2="$(echo "$CREATE_RESP_2" | jq -r '.responseCode // empty')"
CUSTOMER_NO_2="$(echo "$CREATE_RESP_2" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_2="$(echo "$CREATE_RESP_2" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_2" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 18 digits starting with 06" "$([[ "$CUSTOMER_NO_2" =~ ^06[0-9]{16}$ ]] && echo true || echo false)"
check "customerNo differs from Test 1's (per-vaType sequence, not shared)" "$([[ "$CUSTOMER_NO_2" != "$CUSTOMER_NO_1" ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_2" == "15975${CUSTOMER_NO_2}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_2}"
echo "==> derived virtualAccountNo: ${VA_NO_2}"
echo

echo "--- inquiry (vendor identity) ---"
INQUIRY_RESP_2="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15975 -c "$CUSTOMER_NO_2" -v "$VA_NO_2" -a "$FIXED_AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESP_2" | jq .
echo

echo "--- payment (exact fixed amount; vendor identity) ---"
PAYMENT_RESP_2="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15975 -c "$CUSTOMER_NO_2" -v "$VA_NO_2" -a "$FIXED_AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
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
CREATE_RESP_3="$("$SCRIPT_DIR/merchant-create-va.sh" -s 15974 -n "Dyn Var ${RUN_ID}" -y 05 -a "$TARGET_AMOUNT" -t "trx-dyn-variable-${RUN_ID}" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$CREATE_RESP_3" | jq .

RESP_CODE_3="$(echo "$CREATE_RESP_3" | jq -r '.responseCode // empty')"
CUSTOMER_NO_3="$(echo "$CREATE_RESP_3" | jq -r '.virtualAccountData.customerNo // empty')"
VA_NO_3="$(echo "$CREATE_RESP_3" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
check "create-va succeeded (responseCode 2xx)" "$([[ "$RESP_CODE_3" == 2* ]] && echo true || echo false)"
check "server-generated customerNo is 18 digits starting with 05" "$([[ "$CUSTOMER_NO_3" =~ ^05[0-9]{16}$ ]] && echo true || echo false)"
check "server-derived virtualAccountNo equals partnerServiceId+customerNo" "$([[ "$VA_NO_3" == "15974${CUSTOMER_NO_3}" ]] && echo true || echo false)"
echo "==> generated customerNo: ${CUSTOMER_NO_3}"
echo "==> derived virtualAccountNo: ${VA_NO_3}"
echo

echo "--- inquiry (vendor identity) ---"
INQUIRY_RESP_3="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "$TARGET_AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESP_3" | jq .
echo

echo "--- payment 1/2: partial payment (60000.00 of 100000.00 target; vendor identity) ---"
PAYMENT_RESP_3A="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "60000.00" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$PAYMENT_RESP_3A" | jq .
PAY_CODE_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.responseCode // empty')"
PAY_STATUS_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.virtualAccountData.paymentFlagStatus // empty')"
PAID_3A="$(echo "$PAYMENT_RESP_3A" | jq -r '.virtualAccountData.paidAmount.value // empty')"
check "partial payment succeeded (responseCode 2xx)" "$([[ "$PAY_CODE_3A" == 2* ]] && echo true || echo false)"
# 00, not 03. BCA's payment service (25) enumerates only 00/01/02 and states
# "Payment flag status other than 00,01,02 will be considered as 01" — so
# reporting an accepted instalment as 03 told the channel the payment was
# REJECTED while the money had in fact been recorded. 03 = Pending is valid
# only on the inquiry-status service (26); the bill's own pending-ness is
# carried by the transaction, not by this flag.
check "partial payment is flagged accepted (00) — 03 is not a payment-service value" "$([[ "$PAY_STATUS_3A" == "00" ]] && echo true || echo false)"
check "cumulative paidAmount reflects the first payment (60000.00)" "$([[ "$PAID_3A" == "60000.00" ]] && echo true || echo false)"
echo

echo "--- payment 2/2: remaining payment (40000.00, reaches 100000.00 target; vendor identity) ---"
PAYMENT_RESP_3B="$("$SCRIPT_DIR/vendor-payment-va.sh" -s 15974 -c "$CUSTOMER_NO_3" -v "$VA_NO_3" -a "40000.00" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
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
