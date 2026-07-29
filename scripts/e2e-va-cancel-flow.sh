#!/usr/bin/env bash
#
# End-to-end VA cancel flow: proves (a) a still-pending VA CAN be cancelled
# and its number reused afterward, and (b) a PAID VA can neither be cancelled
# nor have its payment silently overwritten.
#
# This exercises BOTH sides of the auth model (features 009/010), which use
# two INDEPENDENT identities/credentials — a vendor never needs a merchant's
# credentials or vice versa:
#   - Merchant side (create-va/delete-va): accessToken (Bearer) + HMAC
#     signature using a merchant's own shared secret. See onboard-merchant.sh.
#   - Vendor side (payment): HMAC signature only, using a vendor's own shared
#     secret. See onboard-vendor.sh.
#
# Chains together, in order:
#   1. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va   (VA #1: to be cancelled)
#   2. merchant-delete-va.sh  DELETE /openapi/v1.0/transfer-va/delete-va (cancel VA #1 while pending -> expect success)
#   3. merchant-create-va.sh  (re-create VA #1's number -> expect success: a cancelled/deleted
#      number is free to reuse, same as a paid one)
#   4. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va   (VA #2: to be paid)
#   5. vendor-payment-va.sh   POST /openapi/v1.0/transfer-va/payment     (pay VA #2 -> status becomes "00")
#   6. merchant-delete-va.sh  DELETE /openapi/v1.0/transfer-va/delete-va (try cancelling VA #2 -> expect
#      REJECTION 4053101: a paid transaction cannot be cancelled)
#   7. vendor-payment-va.sh   (try paying VA #2 again with a NEW paymentRequestId -> expect
#      REJECTION 4092500: a paid transaction cannot be overwritten by a second payment)
#
# Usage:
#   ./scripts/e2e-va-cancel-flow.sh -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> \
#       -m <.env.merchant.NAME> -f <.env.vendor.channel> [-a <amount>] [-u <base-url>]
#
# -m is passed straight through to merchant-create-va.sh / merchant-delete-va.sh's -f.
# -f is passed straight through to vendor-payment-va.sh's -f.
#
# Two distinct virtualAccountNo values are derived from -c (suffixed 1/2) so
# the "cancel while pending" and "cancel while paid" scenarios don't collide
# with each other's state; -c should leave room for the suffix (customerNo
# max length is 20 per ASPI VAIdentity).
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NAME=""
AMOUNT="100000.00"
MERCHANT_ENV_FILE=""
VENDOR_ENV_FILE=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -m <.env.merchant.NAME> -f <.env.vendor.channel> [-a <amount>] [-u <base-url>]" >&2
	exit 1
}

while getopts "s:c:n:a:m:f:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	m) MERCHANT_ENV_FILE="$OPTARG" ;;
	f) VENDOR_ENV_FILE="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NAME" || -z "$MERCHANT_ENV_FILE" || -z "$VENDOR_ENV_FILE" ]] && usage
[[ -f "$MERCHANT_ENV_FILE" ]] || { echo "merchant env file not found: $MERCHANT_ENV_FILE (run onboard-merchant.sh first)" >&2; exit 1; }
[[ -f "$VENDOR_ENV_FILE" ]] || { echo "vendor env file not found: $VENDOR_ENV_FILE (run onboard-vendor.sh first)" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required for this script" >&2; exit 1; }

CUSTOMER_NO_CANCEL="${CUSTOMER_NO}1"
CUSTOMER_NO_PAID="${CUSTOMER_NO}2"
VA_NO_CANCEL="${PARTNER_SERVICE_ID}${CUSTOMER_NO_CANCEL}"
VA_NO_PAID="${PARTNER_SERVICE_ID}${CUSTOMER_NO_PAID}"

# expect_code STEP_LABEL RESPONSE PREFIX
# Asserts responseCode starts with PREFIX; exits non-zero with a clear message otherwise.
expect_code() {
	local label="$1" response="$2" prefix="$3" code
	code="$(echo "$response" | jq -r '.responseCode // empty')"
	if [[ "$code" != "$prefix"* ]]; then
		echo "!! [$label] expected responseCode starting with '$prefix', got '${code:-<none>}' — aborting." >&2
		echo "$response" | jq . >&2
		exit 1
	fi
	echo "==> [$label] OK (responseCode: $code)"
}

echo "=================================================================="
echo "Step 1/7: Create VA #1 (to be cancelled while pending; merchant identity)"
echo "=================================================================="
CREATE1_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -n "$VA_NAME" -v "$VA_NO_CANCEL" -a "$AMOUNT" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$CREATE1_RESPONSE" | jq .
expect_code "create VA#1" "$CREATE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 2/7: Cancel VA #1 while still pending -> expect success (merchant identity)"
echo "=================================================================="
DELETE1_RESPONSE="$("$SCRIPT_DIR/merchant-delete-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -v "$VA_NO_CANCEL" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$DELETE1_RESPONSE" | jq .
expect_code "cancel pending VA#1" "$DELETE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 3/7: Re-create VA #1's number -> expect success (cancelled VAs are reusable)"
echo "=================================================================="
RECREATE1_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -n "$VA_NAME" -v "$VA_NO_CANCEL" -a "$AMOUNT" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$RECREATE1_RESPONSE" | jq .
expect_code "re-create VA#1 after cancel" "$RECREATE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 4/7: Create VA #2 (to be paid; merchant identity)"
echo "=================================================================="
CREATE2_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -n "$VA_NAME" -v "$VA_NO_PAID" -a "$AMOUNT" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$CREATE2_RESPONSE" | jq .
expect_code "create VA#2" "$CREATE2_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 5/7: Pay VA #2 -> status becomes paid (00; vendor identity)"
echo "=================================================================="
PAYMENT_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$PAYMENT_RESPONSE" | jq .
expect_code "pay VA#2" "$PAYMENT_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 6/7: Try cancelling VA #2 (now paid) -> expect REJECTION (merchant identity)"
echo "=================================================================="
DELETE2_RESPONSE="$("$SCRIPT_DIR/merchant-delete-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")"
echo "$DELETE2_RESPONSE" | jq .
expect_code "cancel paid VA#2 (must be rejected)" "$DELETE2_RESPONSE" "405"
echo

echo "=================================================================="
echo "Step 7/7: Try paying VA #2 again with a NEW paymentRequestId -> expect REJECTION (vendor identity)"
echo "=================================================================="
PAYMENT2_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -a "999999.00" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$PAYMENT2_RESPONSE" | jq .
expect_code "re-pay already-paid VA#2 (must be rejected)" "$PAYMENT2_RESPONSE" "409"
echo

echo "=================================================================="
echo "Done: pending VA cancel + reuse works; paid VA can neither be cancelled"
echo "      nor have its payment silently overwritten."
echo "=================================================================="
