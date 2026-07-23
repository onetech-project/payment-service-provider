#!/usr/bin/env bash
#
# End-to-end VA cancel flow: proves (a) a still-pending VA CAN be cancelled
# and its number reused afterward, and (b) a PAID VA can neither be cancelled
# nor have its payment silently overwritten.
#
# Chains together, in order:
#   1. curl-b2b-token.sh      POST /v1.0/access-token/b2b        (get accessToken)
#   2. merchant-create-va.sh  POST /v1.0/transfer-va/create-va   (VA #1: to be cancelled)
#   3. merchant-delete-va.sh  DELETE /v1.0/transfer-va/delete-va (cancel VA #1 while pending -> expect success)
#   4. merchant-create-va.sh  (re-create VA #1's number -> expect success: a cancelled/deleted
#      number is free to reuse, same as a paid one)
#   5. merchant-create-va.sh  POST /v1.0/transfer-va/create-va   (VA #2: to be paid)
#   6. vendor-payment-va.sh   POST /v1.0/transfer-va/payment     (pay VA #2 -> status becomes "00")
#   7. merchant-delete-va.sh  DELETE /v1.0/transfer-va/delete-va (try cancelling VA #2 -> expect
#      REJECTION 4053101: a paid transaction cannot be cancelled)
#   8. vendor-payment-va.sh   (try paying VA #2 again with a NEW paymentRequestId -> expect
#      REJECTION 4092500: a paid transaction cannot be overwritten by a second payment)
#
# Each step's JSON response is captured (diagnostics from the sub-scripts go
# to stderr, so stdout stays clean JSON) and the accessToken is threaded
# automatically into every step — no manual copy-pasting.
#
# Usage:
#   ./scripts/e2e-va-cancel-flow.sh -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> \
#       -i <client_id> -k <private_key.pem> (-e <client-secret> | -f <env-file>) \
#       [-a <amount>] [-u <base-url>]
#
# -i/-k are for the B2B access-token call (asymmetric RSA signing, see
# curl-b2b-token.sh). -e/-f are for the create-va/payment HMAC signing (see
# merchant-create-va.sh / vendor-payment-va.sh) — -f loads VENDOR_CLIENT_SECRET
# straight out of a .env.<vendor>.<channel> file instead of typing it raw.
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
CLIENT_ID=""
PRIVATE_KEY_PATH=""
CLIENT_SECRET=""
ENV_FILE=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -i <client_id> -k <private_key.pem> (-e <client-secret> | -f <env-file>) [-a <amount>] [-u <base-url>]" >&2
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

while getopts "s:c:n:a:i:k:e:f:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
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

	# read_env_var succeeds (and prints "") when the key exists but its value is
	# blank, e.g. "VENDOR_CLIENT_SECRET=" — distinguish that from "missing
	# entirely" so the error below actually points at the fix.
	[[ -z "$CLIENT_SECRET" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_SECRET is empty — fill it in, or pass -e <client-secret> directly." >&2
	[[ -z "$CLIENT_ID" ]] && echo "!! ${ENV_FILE}: VENDOR_CLIENT_ID is empty — fill it in, or pass -i <client-id> directly." >&2
	[[ -z "$PRIVATE_KEY_PATH" ]] && echo "!! ${ENV_FILE}: VENDOR_PRIVATE_KEY_PATH is empty — fill it in, or pass -k <private-key.pem> directly." >&2
fi

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NAME" || -z "$CLIENT_ID" || -z "$PRIVATE_KEY_PATH" || -z "$CLIENT_SECRET" ]] && usage
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
echo "Step 1/8: POST /v1.0/access-token/b2b"
echo "=================================================================="
TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$CLIENT_ID" -p "$PRIVATE_KEY_PATH" -u "$BASE_URL")"
ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty')"
if [[ -z "$ACCESS_TOKEN" ]]; then
	echo "!! Failed to obtain accessToken from step 1 response below — aborting." >&2
	echo "$TOKEN_RESPONSE" | jq . >&2
	exit 1
fi
echo "==> accessToken acquired: ${ACCESS_TOKEN:0:12}..."
echo

echo "=================================================================="
echo "Step 2/8: Create VA #1 (to be cancelled while pending)"
echo "=================================================================="
CREATE1_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -n "$VA_NAME" -v "$VA_NO_CANCEL" -a "$AMOUNT" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$CREATE1_RESPONSE" | jq .
expect_code "create VA#1" "$CREATE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 3/8: Cancel VA #1 while still pending -> expect success"
echo "=================================================================="
DELETE1_RESPONSE="$("$SCRIPT_DIR/merchant-delete-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -v "$VA_NO_CANCEL" -u "$BASE_URL")"
echo "$DELETE1_RESPONSE" | jq .
expect_code "cancel pending VA#1" "$DELETE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 4/8: Re-create VA #1's number -> expect success (cancelled VAs are reusable)"
echo "=================================================================="
RECREATE1_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_CANCEL" -n "$VA_NAME" -v "$VA_NO_CANCEL" -a "$AMOUNT" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$RECREATE1_RESPONSE" | jq .
expect_code "re-create VA#1 after cancel" "$RECREATE1_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 5/8: Create VA #2 (to be paid)"
echo "=================================================================="
CREATE2_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -n "$VA_NAME" -v "$VA_NO_PAID" -a "$AMOUNT" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$CREATE2_RESPONSE" | jq .
expect_code "create VA#2" "$CREATE2_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 6/8: Pay VA #2 -> status becomes paid (00)"
echo "=================================================================="
PAYMENT_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -a "$AMOUNT" -e "$CLIENT_SECRET" -t "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$PAYMENT_RESPONSE" | jq .
expect_code "pay VA#2" "$PAYMENT_RESPONSE" "200"
echo

echo "=================================================================="
echo "Step 7/8: Try cancelling VA #2 (now paid) -> expect REJECTION"
echo "=================================================================="
DELETE2_RESPONSE="$("$SCRIPT_DIR/merchant-delete-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -u "$BASE_URL")"
echo "$DELETE2_RESPONSE" | jq .
expect_code "cancel paid VA#2 (must be rejected)" "$DELETE2_RESPONSE" "405"
echo

echo "=================================================================="
echo "Step 8/8: Try paying VA #2 again with a NEW paymentRequestId -> expect REJECTION"
echo "=================================================================="
PAYMENT2_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO_PAID" -v "$VA_NO_PAID" -a "999999.00" -e "$CLIENT_SECRET" -t "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$PAYMENT2_RESPONSE" | jq .
expect_code "re-pay already-paid VA#2 (must be rejected)" "$PAYMENT2_RESPONSE" "409"
echo

echo "=================================================================="
echo "Done: pending VA cancel + reuse works; paid VA can neither be cancelled"
echo "      nor have its payment silently overwritten."
echo "=================================================================="
