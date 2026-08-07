#!/usr/bin/env bash
#
# Live negative-case suite for the vendor-facing SNAP VA endpoints.
#
# The Go suite in test/e2e already covers these against an in-memory
# repository. This one runs the same rejections against a REAL deployment —
# real Postgres, real Redis idempotency, real access tokens — which is where
# the business rejections actually live: "not found", "paid bill", "wrong
# amount", "expired" and "inconsistent request" are all decided from stored
# state that an in-memory fake can only approximate.
#
# Every case asserts BOTH the HTTP status and the seven-digit responseCode,
# because BCA's Appendix A pins the pair, not either one alone.
#
# Usage:
#   ./scripts/e2e-negative-cases.sh -m <.env.merchant.NAME> -f <.env.vendor.channel> \
#       [-s <partnerServiceId>] [-u <base-url>] [-O <transcript-file>]
#
# -s defaults to 15975 (static fixed bill), which is what the amount-mismatch
#    and paid-bill cases need.
# -O writes a markdown transcript of every request and response. Bearer tokens
#    are redacted in it; signatures are not, since they are what a reader is
#    checking. Without -O the transcript goes nowhere and only the PASS/FAIL
#    lines are printed.
#
# Requires: curl, openssl, jq, python3
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
MERCHANT_ENV_FILE=""
VENDOR_ENV_FILE=""
PARTNER_SERVICE_ID="15975"
TRANSCRIPT=""

usage() {
	echo "Usage: $0 -m <.env.merchant.NAME> -f <.env.vendor.channel> [-s <partnerServiceId>] [-u <base-url>] [-O <transcript-file>]" >&2
	exit 1
}

while getopts "m:f:s:u:O:h" opt; do
	case "$opt" in
	m) MERCHANT_ENV_FILE="$OPTARG" ;;
	f) VENDOR_ENV_FILE="$OPTARG" ;;
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	O) TRANSCRIPT="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$MERCHANT_ENV_FILE" || -z "$VENDOR_ENV_FILE" ]] && usage
[[ -f "$MERCHANT_ENV_FILE" ]] || { echo "merchant env file not found: $MERCHANT_ENV_FILE" >&2; exit 1; }
[[ -f "$VENDOR_ENV_FILE" ]] || { echo "vendor env file not found: $VENDOR_ENV_FILE" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

read_env_var() {
	local file="$1" key="$2" line value
	line="$(grep -E "^${key}=" "$file" | tail -n1)"
	[[ -n "$line" ]] || return 1
	value="${line#*=}"
	if [[ "$value" == \"*\" && "$value" == *\" ]]; then value="${value:1:${#value}-2}"; fi
	printf '%s' "$value"
}

CLIENT_SECRET="$(read_env_var "$VENDOR_ENV_FILE" VENDOR_CLIENT_SECRET)"
VENDOR_CLIENT_ID="$(read_env_var "$VENDOR_ENV_FILE" VENDOR_CLIENT_ID || true)"
VENDOR_KEY_PATH="$(read_env_var "$VENDOR_ENV_FILE" VENDOR_PRIVATE_KEY_PATH || true)"
CHANNEL_ID="$(read_env_var "$VENDOR_ENV_FILE" VENDOR_CHANNEL_ID || true)"
PARTNER_ID="$(read_env_var "$VENDOR_ENV_FILE" VENDOR_PARTNER_ID || true)"
[[ -z "$CHANNEL_ID" ]] && CHANNEL_ID="95231"
[[ -z "$PARTNER_ID" ]] && PARTNER_ID="111111"

ACCESS_TOKEN=""
if [[ -n "$VENDOR_CLIENT_ID" && -n "$VENDOR_KEY_PATH" ]]; then
	ACCESS_TOKEN="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$VENDOR_CLIENT_ID" -p "$VENDOR_KEY_PATH" -u "$BASE_URL" 2>/dev/null | jq -r '.accessToken // empty')"
	[[ -z "$ACCESS_TOKEN" ]] && { echo "!! could not obtain a vendor accessToken" >&2; exit 1; }
fi

INQUIRY_PATH="/openapi/v1.0/transfer-va/inquiry"
PAYMENT_PATH="/openapi/v1.0/transfer-va/payment"
STATUS_PATH="/openapi/v2.0/transfer-va/status"

PASS=0
FAIL=0
SECTION=""

if [[ -n "$TRANSCRIPT" ]]; then
	mkdir -p "$(dirname "$TRANSCRIPT")"
	{
		echo "# SNAP Virtual Account — live negative-case transcript"
		echo
		echo "Produced by \`scripts/e2e-negative-cases.sh\` against a running deployment"
		echo "(real Postgres, real Redis, real access tokens). Each case shows the request"
		echo "exactly as it went on the wire and the response exactly as it came back."
		echo
		echo "- Generated: \`$(date -Is)\`"
		echo "- Commit: \`$(git -C "$SCRIPT_DIR/.." rev-parse --short HEAD 2>/dev/null || echo unknown)\`"
		echo "- Target: \`${BASE_URL}\`"
		echo
		echo "\`Authorization: Bearer\` values are redacted — they are single-use tokens and"
		echo "add nothing to a conformance review. Signatures are kept: they are computed"
		echo "over the redacted token, so they cannot be recomputed from this file, and"
		echo "seeing them is the point of a signature transcript."
		echo
	} >"$TRANSCRIPT"
fi

section() {
	SECTION="$1"
	[[ -n "$TRANSCRIPT" ]] && { echo; echo "---"; echo; echo "## $1"; echo; } >>"$TRANSCRIPT"
	echo
	echo "== $1"
}

# redact replaces the bearer token wherever it appears (header line and the
# AccessToken component of stringToSign).
redact() {
	if [[ -n "$ACCESS_TOKEN" ]]; then
		python3 -c 'import sys;t=sys.argv[1];sys.stdout.write(sys.stdin.read().replace(t,"<accessToken>"))' "$ACCESS_TOKEN"
	else
		cat
	fi
}

# snap issues a signed request and records it. Globals it reads:
#   SIGN_SECRET   secret to sign with (defaults to the vendor's)
#   SIGN_BODY     body to sign, when it must differ from the body sent
#   TS            X-TIMESTAMP (defaults to now)
#   SIG_OVERRIDE  literal X-SIGNATURE, bypassing the computation
#   EXT_ID        X-EXTERNAL-ID (defaults to a fresh one)
#   OMIT          space-separated header names to leave out
#   EXTRA         array of extra -H arguments
snap() {
	local path="$1" body="$2"
	local secret="${SIGN_SECRET:-$CLIENT_SECRET}"
	local sign_body="${SIGN_BODY:-$body}"
	local ts="${TS:-$(date +%Y-%m-%dT%H:%M:%S%:z)}"
	local ext="${EXT_ID:-$(date +%Y%m%d%H%M%S)$RANDOM}"
	local omit="${OMIT:-}"
	local hash sts sig

	# MinifyJson before hashing, as the spec requires — except for the
	# malformed-body cases, where there is nothing to minify. Hashing jq's
	# empty output there would produce a signature over the wrong bytes and
	# the request would come back 401 instead of reaching the parser, hiding
	# the very rejection the case is testing.
	if printf '%s' "$sign_body" | jq -e . >/dev/null 2>&1; then
		hash="$(printf '%s' "$sign_body" | jq -cj . | openssl dgst -sha256 -binary | xxd -p -c 256)"
	else
		hash="$(printf '%s' "$sign_body" | openssl dgst -sha256 -binary | xxd -p -c 256)"
	fi
	sts="POST:${path}:${ACCESS_TOKEN}:${hash}:${ts}"
	sig="${SIG_OVERRIDE:-$(printf '%s' "$sts" | openssl dgst -sha512 -hmac "$secret" -binary | openssl base64 -A)}"

	local args=(-sS -o /tmp/e2e-neg-body.json -w '%{http_code}' -X POST "${BASE_URL}${path}"
		-H 'Content-Type: application/json')
	local hdrs=("Content-Type: application/json")
	add_header() {
		local name="${1%%:*}"
		case " $omit " in *" $name "*) return ;; esac
		args+=(-H "$1")
		hdrs+=("$1")
	}
	[[ -n "$ACCESS_TOKEN" ]] && add_header "Authorization: Bearer ${ACCESS_TOKEN}"
	add_header "X-TIMESTAMP: ${ts}"
	add_header "X-SIGNATURE: ${sig}"
	add_header "ORIGIN: www.hostname.com"
	add_header "CHANNEL-ID: ${CHANNEL_ID}"
	add_header "X-PARTNER-ID: ${PARTNER_ID}"
	add_header "X-EXTERNAL-ID: ${ext}"
	local h
	for h in ${EXTRA[@]+"${EXTRA[@]}"}; do
		args+=(-H "$h")
		hdrs+=("$h")
	done

	HTTP_CODE="$(curl "${args[@]}" -d "$body" 2>/dev/null)"
	RESP_BODY="$(cat /tmp/e2e-neg-body.json)"
	RESP_CODE="$(printf '%s' "$RESP_BODY" | jq -r '.responseCode // empty' 2>/dev/null)"

	LAST_REQUEST="POST ${path} HTTP/1.1"$'\n'"$(printf '%s\n' "${hdrs[@]}")"$'\n'"stringToSign: ${sts}"$'\n\n'"${body}"
}

# check records the last exchange under a label and asserts status + code.
check() { # check <label> <want-http> <want-code>
	local label="$1" want_http="$2" want_code="$3" verdict
	if [[ "$HTTP_CODE" == "$want_http" && "$RESP_CODE" == "$want_code" ]]; then
		verdict=PASS
		PASS=$((PASS + 1))
		printf '  \033[32mPASS\033[0m  %-58s %s / %s\n' "$label" "$HTTP_CODE" "$RESP_CODE"
	else
		verdict=FAIL
		FAIL=$((FAIL + 1))
		printf '  \033[31mFAIL\033[0m  %-58s got %s / %s, want %s / %s\n' \
			"$label" "$HTTP_CODE" "$RESP_CODE" "$want_http" "$want_code"
	fi

	[[ -z "$TRANSCRIPT" ]] && return
	{
		echo "### ${label}"
		echo
		echo "Expected \`${want_http}\` / \`${want_code}\` — got \`${HTTP_CODE}\` / \`${RESP_CODE}\` (**${verdict}**)"
		echo
		echo "**Request**"
		echo
		echo '```http'
		printf '%s\n' "$LAST_REQUEST" | redact
		echo '```'
		echo
		echo "**Response**"
		echo
		echo '```http'
		echo "HTTP/1.1 ${HTTP_CODE}"
		echo
		printf '%s\n' "$RESP_BODY" | jq . 2>/dev/null || printf '%s\n' "$RESP_BODY"
		echo '```'
		echo
	} >>"$TRANSCRIPT"
}

# reset clears the per-call overrides so one case cannot leak into the next.
reset() {
	SIGN_SECRET=""
	SIGN_BODY=""
	TS=""
	SIG_OVERRIDE=""
	EXT_ID=""
	OMIT=""
	EXTRA=()
}
reset

now() { date +%Y-%m-%dT%H:%M:%S%:z; }

# ------------------------------------------------------------------
# Fixtures: one paid VA and one unpaid VA, both created through the
# merchant API so the business rejections below have real state to hit.
# ------------------------------------------------------------------
RUN="$(date +%H%M%S)$RANDOM"
UNPAID_CNO="03$(date +%s | tail -c 9)00000001"
PAID_CNO="03$(date +%s | tail -c 9)00000002"
UNPAID_VA="${PARTNER_SERVICE_ID}${UNPAID_CNO}"
PAID_VA="${PARTNER_SERVICE_ID}${PAID_CNO}"
BILL_AMOUNT="250000.00"

echo "== Fixtures"
"$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$UNPAID_CNO" -n "Neg Unpaid ${RUN}" \
	-y 03 -a "$BILL_AMOUNT" -t "trx-neg-unpaid-${RUN}" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL" >/dev/null 2>&1
"$SCRIPT_DIR/merchant-create-va.sh" -s "$PARTNER_SERVICE_ID" -c "$PAID_CNO" -n "Neg Paid ${RUN}" \
	-y 03 -a "$BILL_AMOUNT" -t "trx-neg-paid-${RUN}" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL" >/dev/null 2>&1
PAID_REQUEST_ID="PAY-NEG-PAID-${RUN}"
"$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$PAID_CNO" -v "$PAID_VA" \
	-a "$BILL_AMOUNT" -q "$PAID_REQUEST_ID" -f "$VENDOR_ENV_FILE" -u "$BASE_URL" >/dev/null 2>&1
echo "  unpaid VA: ${UNPAID_VA}"
echo "  paid VA:   ${PAID_VA} (paymentRequestId ${PAID_REQUEST_ID})"

inquiry_body() { # inquiry_body <customerNo> <va> <inquiryRequestId>
	cat <<JSON
{"partnerServiceId":"${PARTNER_SERVICE_ID}","customerNo":"$1","virtualAccountNo":"$2","trxDateInit":"$(now)","channelCode":6011,"inquiryRequestId":"$3"}
JSON
}

payment_body() { # payment_body <customerNo> <va> <paymentRequestId> <amount>
	cat <<JSON
{"partnerServiceId":"${PARTNER_SERVICE_ID}","customerNo":"$1","virtualAccountNo":"$2","virtualAccountName":"Payer Name","paymentRequestId":"$3","channelCode":6011,"paidAmount":{"value":"$4","currency":"IDR"},"totalAmount":{"value":"$4","currency":"IDR"},"trxDateTime":"$(now)","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
JSON
}

# ------------------------------------------------------------------
section "Authentication and signature"
# ------------------------------------------------------------------
reset; SIGN_SECRET="not-the-vendors-secret"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-1")"
check "signature computed with the wrong secret" 401 4012400

reset; SIG_OVERRIDE="this-is-not-base64-hmac"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-2")"
check "garbage signature" 401 4012400

# Signed over one body, sent with another — the tamper case. SIGN_BODY is what
# the signature covers; the second argument is what actually goes on the wire.
reset; SIGN_BODY="$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-4a")"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-4b")"
check "body tampered after signing" 401 4012400

reset; OMIT="X-SIGNATURE"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-5")"
check "X-SIGNATURE missing" 400 4002402

reset; OMIT="X-TIMESTAMP"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-6")"
check "X-TIMESTAMP missing" 400 4002402

reset; TS="not-a-timestamp"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-7")"
check "X-TIMESTAMP not parseable" 400 4002401

reset; OMIT="Authorization"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-8")"
check "Authorization missing" 401 4012401

reset; EXTRA=("Authorization: Bearer not.a.valid.token")
OMIT="Authorization"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-9")"
check "Authorization carries an unusable token" 401 4012401

# ------------------------------------------------------------------
section "X-EXTERNAL-ID"
# ------------------------------------------------------------------
reset; OMIT="X-EXTERNAL-ID"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-10")"
check "X-EXTERNAL-ID missing" 400 4002402

reset; EXT_ID="$(printf '9%.0s' {1..40})"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-11")"
check "X-EXTERNAL-ID longer than 36 characters" 400 4002401

REUSED_EXT="$(date +%Y%m%d%H%M%S)$RANDOM"
reset; EXT_ID="$REUSED_EXT"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-12")"
check "first use of an X-EXTERNAL-ID is answered normally" 200 2002400

reset; EXT_ID="$REUSED_EXT"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNPAID_CNO" "$UNPAID_VA" "INQ-NEG-${RUN}-13")"
check "same X-EXTERNAL-ID with a different payload is a Conflict" 409 4092400

# ------------------------------------------------------------------
section "Request payload"
# ------------------------------------------------------------------
reset
snap "$INQUIRY_PATH" '{"partnerServiceId": "15975", '
check "malformed JSON on inquiry" 400 4002400

reset
snap "$PAYMENT_PATH" '{"partnerServiceId": "15975", '
check "malformed JSON on payment" 400 4002500

reset
snap "$INQUIRY_PATH" "{\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"trxDateInit\":\"$(now)\",\"channelCode\":6011,\"inquiryRequestId\":\"INQ-NEG-${RUN}-14\"}"
check "inquiry without partnerServiceId" 400 4002402

reset
snap "$INQUIRY_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"channelCode\":6011,\"inquiryRequestId\":\"INQ-NEG-${RUN}-15\"}"
check "inquiry without trxDateInit (Mandatory in v2.4)" 400 4002402

reset
snap "$INQUIRY_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"trxDateInit\":\"$(now)\",\"inquiryRequestId\":\"INQ-NEG-${RUN}-16\"}"
check "inquiry without channelCode (Mandatory in v2.4)" 400 4002402

reset
snap "$INQUIRY_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"trxDateInit\":\"$(now)\",\"channelCode\":6011,\"language\":\"idn\",\"inquiryRequestId\":\"INQ-NEG-${RUN}-17\"}"
check "inquiry language longer than 2 characters" 400 4002401

reset
snap "$INQUIRY_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"trxDateInit\":\"$(now)\",\"channelCode\":6011,\"passApp\":\"wrong-value-entirely\",\"inquiryRequestId\":\"INQ-NEG-${RUN}-18\"}"
check "passApp is not a credential and must not gate the request" 200 2002400

reset
snap "$PAYMENT_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"virtualAccountName\":\"Payer Name\",\"channelCode\":6011,\"paidAmount\":{\"value\":\"${BILL_AMOUNT}\",\"currency\":\"IDR\"},\"totalAmount\":{\"value\":\"${BILL_AMOUNT}\",\"currency\":\"IDR\"},\"trxDateTime\":\"$(now)\",\"referenceNo\":\"12345678901\",\"flagAdvise\":\"N\"}"
check "payment without paymentRequestId" 400 4002502

reset
snap "$PAYMENT_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNPAID_CNO}\",\"virtualAccountNo\":\"${UNPAID_VA}\",\"virtualAccountName\":\"Payer Name\",\"paymentRequestId\":\"PAY-NEG-${RUN}-1\",\"channelCode\":6011,\"totalAmount\":{\"value\":\"${BILL_AMOUNT}\",\"currency\":\"IDR\"},\"trxDateTime\":\"$(now)\",\"referenceNo\":\"12345678901\",\"flagAdvise\":\"N\"}"
check "payment without paidAmount" 400 4002502

reset
snap "$PAYMENT_PATH" "$(payment_body "$UNPAID_CNO" "$UNPAID_VA" "PAY-NEG-${RUN}-2" "${BILL_AMOUNT}" | sed 's/"flagAdvise":"N"/"flagAdvise":"MAYBE"/')"
check "flagAdvise outside N/Y" 400 4002501

# EUR, not USD: BCA documents "IDR, SGD, USD" as the permitted set, so USD is
# accepted and would settle this bill — taking the amount-mismatch case below
# down with it.
reset
snap "$PAYMENT_PATH" "$(payment_body "$UNPAID_CNO" "$UNPAID_VA" "PAY-NEG-${RUN}-3" "${BILL_AMOUNT}" | sed 's/"currency":"IDR"/"currency":"EUR"/g')"
check "currency outside BCA's IDR/SGD/USD set" 400 4002501

reset
snap "$PAYMENT_PATH" "$(payment_body "$UNPAID_CNO" "$UNPAID_VA" "PAY-NEG-${RUN}-3b" "${BILL_AMOUNT}" | sed '0,/"currency":"IDR"/s//"currency":"USD"/')"
check "currency disagrees between paidAmount and totalAmount" 400 4002501

reset
snap "$PAYMENT_PATH" "$(payment_body "$UNPAID_CNO" "$UNPAID_VA" "PAY-NEG-${RUN}-4" "not-a-number")"
check "paidAmount is not numeric" 400 4002501

# ------------------------------------------------------------------
section "Business rejections against stored state"
# ------------------------------------------------------------------
UNKNOWN_CNO="039999999999999999"
UNKNOWN_VA="${PARTNER_SERVICE_ID}${UNKNOWN_CNO}"

reset
snap "$INQUIRY_PATH" "$(inquiry_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "INQ-NEG-${RUN}-20")"
check "inquiry on a VA that was never registered" 404 4042412

reset
snap "$PAYMENT_PATH" "$(payment_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "PAY-NEG-${RUN}-5" "${BILL_AMOUNT}")"
check "payment on a VA that was never registered" 404 4042512

reset
snap "$PAYMENT_PATH" "$(payment_body "$UNPAID_CNO" "$UNPAID_VA" "PAY-NEG-${RUN}-6" "1000.00")"
check "payment amount disagrees with the fixed bill" 404 4042513

reset
snap "$INQUIRY_PATH" "$(inquiry_body "$PAID_CNO" "$PAID_VA" "INQ-NEG-${RUN}-21")"
check "inquiry on an already-paid bill" 404 4042414

reset
snap "$PAYMENT_PATH" "$(payment_body "$PAID_CNO" "$PAID_VA" "PAY-NEG-${RUN}-7" "${BILL_AMOUNT}")"
check "second payment on an already-paid bill" 404 4042514

reset
snap "$PAYMENT_PATH" "$(payment_body "$PAID_CNO" "$PAID_VA" "$PAID_REQUEST_ID" "1.00")"
check "same paymentRequestId resubmitted with different content" 404 4042518

reset
snap "$STATUS_PATH" "{\"partnerServiceId\":\"${PARTNER_SERVICE_ID}\",\"customerNo\":\"${UNKNOWN_CNO}\",\"virtualAccountNo\":\"${UNKNOWN_VA}\",\"inquiryRequestId\":\"INQ-DOES-NOT-EXIST-${RUN}\",\"additionalInfo\":{}}"
check "status for an id that was never issued" 404 4042601

# ------------------------------------------------------------------
section "Headers outside BCA's documented set are ignored"
# ------------------------------------------------------------------
reset; EXTRA=("Idempotency-Key: 11111111-1111-1111-1111-111111111111")
snap "$INQUIRY_PATH" "$(inquiry_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "INQ-NEG-${RUN}-30")"
check "Idempotency-Key changes nothing" 404 4042412

reset; EXTRA=("X-CLIENT-KEY: ${VENDOR_CLIENT_ID:-unused}")
snap "$INQUIRY_PATH" "$(inquiry_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "INQ-NEG-${RUN}-31")"
check "X-CLIENT-KEY (an access-token header) changes nothing" 404 4042412

reset; EXTRA=("X-DEVICE-ID: dev-1" "X-IP-ADDRESS: 10.0.0.1" "X-LATITUDE: -6.2" "X-LONGITUDE: 106.8")
snap "$INQUIRY_PATH" "$(inquiry_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "INQ-NEG-${RUN}-32")"
check "other SNAP-ecosystem headers change nothing" 404 4042412

reset; OMIT="ORIGIN"
snap "$INQUIRY_PATH" "$(inquiry_body "$UNKNOWN_CNO" "$UNKNOWN_VA" "INQ-NEG-${RUN}-33")"
check "ORIGIN is optional (Mandatory N)" 404 4042412

echo
echo "=================================================================="
printf 'NEGATIVE CASES: \033[32m%d passed\033[0m, \033[31m%d failed\033[0m\n' "$PASS" "$FAIL"
[[ -n "$TRANSCRIPT" ]] && echo "Transcript: $TRANSCRIPT"
echo "=================================================================="
exit $((FAIL > 0 ? 1 : 0))
