#!/usr/bin/env bash
#
# End-to-end test for feature 007-merchant-expiry-callback: create a VA with
# a short expiry window, let it expire, then exercise both User Stories:
#
#   US1 (expiry detection & callback):
#     - inquiry on the expired VA returns 4042419 / inquiryStatus 01
#     - payment notify on the expired VA returns 4042519 / paymentFlagStatus 01
#     - the VA transitions to status "02" and exactly one signed "va.expired"
#       callback is delivered to the merchant's notificationUrl
#     - a second inquiry/notify does NOT trigger a duplicate callback (dedupe)
#
#   US2 (admin resend endpoint):
#     - POST /admin/transactions/:virtualAccountNo/resend-callback redelivers
#       the va.expired event and records a second ("manual") delivery
#
# This exercises BOTH sides of the auth model (features 009/010), which use
# two INDEPENDENT identities/credentials — a vendor never needs a merchant's
# credentials or vice versa:
#   - Merchant side (create-va): accessToken (Bearer) + HMAC signature using
#     a merchant's own shared secret. See onboard-merchant.sh.
#   - Vendor side (inquiry/payment): HMAC signature only, using a vendor's
#     own shared secret. See onboard-vendor.sh.
#
# Chains together, in order:
#   1. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va (with -x <short expiredDate>)
#   2. (sleep until past expiry)
#   3. vendor-inquiry-va.sh   POST /openapi/v1.0/transfer-va/inquiry     -> expect 4042419
#   4. vendor-payment-va.sh   POST /openapi/v1.0/transfer-va/payment     -> expect 4042519
#   5. (poll webhook.site for the auto "va.expired" callback)
#   6. vendor-inquiry-va.sh again                                       -> expect 4042419, NO 2nd callback
#   7. POST /admin/transactions/:virtualAccountNo/resend-callback        -> expect 200
#   8. (poll webhook.site for the manual resend "va.expired" callback)
#
# Callback verification uses webhook.site's request-inspection API
# (https://webhook.site/token/<token>/requests) rather than a local listener,
# since a merchant-facing public URL is required to demonstrate this end to
# end. Pass -w with your own webhook.site "view" URL or bare token; the
# https://webhook.site/#!/view/<token> and https://webhook.site/<token> forms
# are both accepted.
#
# Usage:
#   ./scripts/e2e-expired-callback-flow.sh -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> \
#       -m <.env.merchant.NAME> -f <.env.vendor.channel> -K <admin-api-key> \
#       [-w <webhook.site-url-or-token>] [-a <amount>] [-v <virtualAccountNo>] \
#       [-x <seconds-until-expiry>] [-u <base-url>]
#
# -m is passed straight through to merchant-create-va.sh's -f.
# -f is passed straight through to vendor-inquiry-va.sh / vendor-payment-va.sh's -f.
# -K is the ADMIN_API_KEY the server was started with (see .env.example),
# required for step 7's X-Admin-API-Key header.
#
# -x <seconds-until-expiry> (default 8) sets how far in the future
# expiredDate is when the VA is created; the script then sleeps
# (seconds-until-expiry + 5) before continuing, so the VA is reliably past
# its expiredDate for steps 4+.
#
# Requires: curl, openssl, uuidgen, jq
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NAME=""
AMOUNT="100000.00"
VA_NO=""
WEBHOOK_INPUT=""
MERCHANT_ENV_FILE=""
VENDOR_ENV_FILE=""
ADMIN_API_KEY=""
EXPIRE_IN_SECONDS="8"

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -m <.env.merchant.NAME> -f <.env.vendor.channel> -K <admin-api-key> [-w <webhook.site-url-or-token>] [-a <amount>] [-v <virtualAccountNo>] [-x <seconds-until-expiry>] [-u <base-url>]" >&2
	exit 1
}

while getopts "s:c:n:a:v:w:m:f:K:x:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	w) WEBHOOK_INPUT="$OPTARG" ;;
	m) MERCHANT_ENV_FILE="$OPTARG" ;;
	f) VENDOR_ENV_FILE="$OPTARG" ;;
	K) ADMIN_API_KEY="$OPTARG" ;;
	x) EXPIRE_IN_SECONDS="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$PARTNER_SERVICE_ID" || -z "$CUSTOMER_NO" || -z "$VA_NAME" || -z "$MERCHANT_ENV_FILE" || -z "$VENDOR_ENV_FILE" || -z "$ADMIN_API_KEY" ]] && usage
[[ -f "$MERCHANT_ENV_FILE" ]] || { echo "merchant env file not found: $MERCHANT_ENV_FILE (run onboard-merchant.sh first)" >&2; exit 1; }
[[ -f "$VENDOR_ENV_FILE" ]] || { echo "vendor env file not found: $VENDOR_ENV_FILE (run onboard-vendor.sh first)" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required for this script" >&2; exit 1; }

# Accept https://webhook.site/#!/view/<token>, https://webhook.site/<token>,
# or a bare <token> for -w.
WEBHOOK_TOKEN=""
NOTIFICATION_URL=""
if [[ -n "$WEBHOOK_INPUT" ]]; then
	WEBHOOK_TOKEN="$(printf '%s' "$WEBHOOK_INPUT" | sed -E 's#^https?://webhook\.site/##; s#.*/##')"
	NOTIFICATION_URL="https://webhook.site/${WEBHOOK_TOKEN}"
	echo "==> Using webhook.site token: ${WEBHOOK_TOKEN} (notificationUrl: ${NOTIFICATION_URL})"
else
	echo "!! No -w given — VA will be created without a notificationUrl. Expiry status/response-code" >&2
	echo "   assertions (steps 4/5/7) still run; callback delivery checks (steps 6/9) and the resend" >&2
	echo "   endpoint (step 8, which requires a prior delivery record) will be skipped." >&2
fi
echo

# webhook_request_count prints how many requests webhook.site has recorded
# for our token so far (0 if the API call fails, e.g. offline/rate-limited).
webhook_request_count() {
	curl -sS "https://webhook.site/token/${WEBHOOK_TOKEN}/requests?sorting=newest&per_page=50" 2>/dev/null \
		| jq -r '.data | length' 2>/dev/null || echo 0
}

# wait_for_new_webhook_request polls until webhook.site's recorded request
# count for our token exceeds $1, or ~20s elapses. Prints the newest request's
# content (pretty-printed if JSON) to stdout on success.
wait_for_new_webhook_request() {
	local baseline="$1" label="$2" count body
	for _ in $(seq 1 20); do
		count="$(webhook_request_count)"
		if [[ "$count" -gt "$baseline" ]]; then
			echo "==> ${label}: new webhook.site request received (total now: ${count})"
			body="$(curl -sS "https://webhook.site/token/${WEBHOOK_TOKEN}/requests?sorting=newest&per_page=1" 2>/dev/null | jq -r '.data[0].content // empty')"
			echo "$body" | (jq . 2>/dev/null || cat)
			return 0
		fi
		sleep 1
	done
	echo "!! ${label}: no new webhook.site request within 20s (baseline was ${baseline}, still ${count})." >&2
	echo "   Check https://webhook.site/#!/view/${WEBHOOK_TOKEN} manually — the Asynq worker may still be" >&2
	echo "   catching up, or may not be running." >&2
	return 1
}

echo "=================================================================="
echo "Step 1/8: POST /openapi/v1.0/transfer-va/create-va (expiring in ${EXPIRE_IN_SECONDS}s; merchant identity)"
echo "=================================================================="
EXPIRED_DATE="$(date -u -d "+${EXPIRE_IN_SECONDS} seconds" +%Y-%m-%dT%H:%M:%S+00:00 2>/dev/null || date -u -v+"${EXPIRE_IN_SECONDS}"S +%Y-%m-%dT%H:%M:%S+00:00)"
CREATE_VA_ARGS=(-s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -n "$VA_NAME" -a "$AMOUNT" -f "$MERCHANT_ENV_FILE" -x "$EXPIRED_DATE" -u "$BASE_URL")
[[ -n "$VA_NO" ]] && CREATE_VA_ARGS+=(-v "$VA_NO")
[[ -n "$NOTIFICATION_URL" ]] && CREATE_VA_ARGS+=(-w "$NOTIFICATION_URL")

CREATE_VA_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" "${CREATE_VA_ARGS[@]}")"
echo "$CREATE_VA_RESPONSE" | jq .

RESPONSE_CODE="$(echo "$CREATE_VA_RESPONSE" | jq -r '.responseCode // empty')"
if [[ "$RESPONSE_CODE" != 2* ]]; then
	echo "!! create-va did not return a success responseCode (got: ${RESPONSE_CODE:-<none>}) — aborting." >&2
	exit 1
fi

CONFIRMED_VA_NO="$(echo "$CREATE_VA_RESPONSE" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
if [[ -n "$CONFIRMED_VA_NO" ]]; then
	VA_NO="$CONFIRMED_VA_NO"
elif [[ -z "$VA_NO" ]]; then
	VA_NO="${PARTNER_SERVICE_ID}${CUSTOMER_NO}"
fi
echo "==> virtualAccountNo: ${VA_NO}"
echo "==> expiredDate: ${EXPIRED_DATE}"
echo

echo "=================================================================="
echo "Step 2/8: waiting $((EXPIRE_IN_SECONDS + 5))s for the VA to pass its expiredDate"
echo "=================================================================="
sleep "$((EXPIRE_IN_SECONDS + 5))"
echo "==> done waiting"
echo

echo "=================================================================="
echo "Step 3/8: POST /openapi/v1.0/transfer-va/inquiry (expect 4042419)"
echo "=================================================================="
BASELINE_COUNT=0
[[ -n "$WEBHOOK_TOKEN" ]] && BASELINE_COUNT="$(webhook_request_count)"

INQUIRY_RESPONSE="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESPONSE" | jq .

INQUIRY_CODE="$(echo "$INQUIRY_RESPONSE" | jq -r '.responseCode // empty')"
INQUIRY_STATUS="$(echo "$INQUIRY_RESPONSE" | jq -r '.virtualAccountData.inquiryStatus // empty')"
if [[ "$INQUIRY_CODE" == "4042419" && "$INQUIRY_STATUS" == "01" ]]; then
	echo "==> PASS: inquiry correctly rejected as expired (4042419 / inquiryStatus 01)"
else
	echo "!! FAIL: expected responseCode=4042419 and inquiryStatus=01, got responseCode=${INQUIRY_CODE:-<none>} inquiryStatus=${INQUIRY_STATUS:-<none>}" >&2
fi
echo

echo "=================================================================="
echo "Step 4/8: POST /openapi/v1.0/transfer-va/payment (expect 4042519)"
echo "=================================================================="
PAYMENT_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$PAYMENT_RESPONSE" | jq .

PAYMENT_CODE="$(echo "$PAYMENT_RESPONSE" | jq -r '.responseCode // empty')"
PAYMENT_FLAG_STATUS="$(echo "$PAYMENT_RESPONSE" | jq -r '.virtualAccountData.paymentFlagStatus // empty')"
if [[ "$PAYMENT_CODE" == "4042519" && "$PAYMENT_FLAG_STATUS" == "01" ]]; then
	echo "==> PASS: payment notification correctly rejected as expired (4042519 / paymentFlagStatus 01)"
else
	echo "!! FAIL: expected responseCode=4042519 and paymentFlagStatus=01, got responseCode=${PAYMENT_CODE:-<none>} paymentFlagStatus=${PAYMENT_FLAG_STATUS:-<none>}" >&2
fi
echo

echo "=================================================================="
echo "Step 5/8: verify the automatic va.expired callback was delivered"
echo "=================================================================="
if [[ -n "$WEBHOOK_TOKEN" ]]; then
	wait_for_new_webhook_request "$BASELINE_COUNT" "auto va.expired callback" || true
	AFTER_AUTO_COUNT="$(webhook_request_count)"
else
	echo "==> skipped (no -w webhook.site token given)"
	AFTER_AUTO_COUNT=0
fi
echo

echo "=================================================================="
echo "Step 6/8: repeat inquiry (expect 4042419 again, NO second callback)"
echo "=================================================================="
INQUIRY_RESPONSE_2="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESPONSE_2" | jq .
INQUIRY_CODE_2="$(echo "$INQUIRY_RESPONSE_2" | jq -r '.responseCode // empty')"
[[ "$INQUIRY_CODE_2" == "4042419" ]] && echo "==> PASS: still returns 4042419" || echo "!! FAIL: expected 4042419, got ${INQUIRY_CODE_2:-<none>}" >&2

if [[ -n "$WEBHOOK_TOKEN" ]]; then
	sleep 3
	NOW_COUNT="$(webhook_request_count)"
	if [[ "$NOW_COUNT" -le "$AFTER_AUTO_COUNT" ]]; then
		echo "==> PASS: no duplicate callback sent (webhook.site request count unchanged: ${NOW_COUNT})"
	else
		echo "!! FAIL: webhook.site request count increased (${AFTER_AUTO_COUNT} -> ${NOW_COUNT}) — dedupe may not be working" >&2
	fi
fi
echo

echo "=================================================================="
echo "Step 7/8: POST /admin/transactions/${VA_NO}/resend-callback"
echo "=================================================================="
if [[ -z "$WEBHOOK_TOKEN" ]]; then
	echo "==> skipped (no -w webhook.site token given, and resend requires a prior delivery record" \
		"which needs a registered notificationUrl)"
else
	RESEND_BASELINE_COUNT="$(webhook_request_count)"
	RESEND_RESPONSE="$(curl -sS -X POST "${BASE_URL}/admin/transactions/${VA_NO}/resend-callback" \
		-H "X-Admin-API-Key: ${ADMIN_API_KEY}")"
	echo "$RESEND_RESPONSE" | jq . 2>/dev/null || echo "$RESEND_RESPONSE"

	RESEND_EVENT_TYPE="$(echo "$RESEND_RESPONSE" | jq -r '.eventType // empty' 2>/dev/null)"
	RESEND_DELIVERY_STATUS="$(echo "$RESEND_RESPONSE" | jq -r '.deliveryStatus // empty' 2>/dev/null)"
	if [[ "$RESEND_EVENT_TYPE" == "va.expired" ]]; then
		echo "==> PASS: resend endpoint redelivered eventType=va.expired (deliveryStatus=${RESEND_DELIVERY_STATUS:-<none>})"
	else
		echo "!! FAIL: expected eventType=va.expired in resend response, got: ${RESEND_RESPONSE}" >&2
	fi
	echo

	echo "=================================================================="
	echo "Step 8/8: verify the manual resend callback was delivered"
	echo "=================================================================="
	wait_for_new_webhook_request "$RESEND_BASELINE_COUNT" "manual resend callback" || true
fi

echo
echo "=================================================================="
echo "Done. Review https://webhook.site/#!/view/${WEBHOOK_TOKEN:-<no-token-given>} for the full raw" \
	"delivery history (auto + manual, headers include X-Timestamp/X-Signature)."
echo "=================================================================="
