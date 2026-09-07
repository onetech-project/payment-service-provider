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
#       .env.<vendor>.<channel> for inquiry/payment/status; either works for
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
#   -C  channelCode (inquiry/payment, default 6011 = ATM)
#   -A  flagAdvise (payment, default N; Y = advice/retry)
#   -R  referenceNo (payment, numeric String(11); default: generated)
#   -L  language (inquiry, ISO-639-1, default empty)
#   -B  sourceBankCode (inquiry/payment, default 014 = BCA)
#   -H  body-hash encoding: hex | base64 (default: the env file's
#       VENDOR_BODY_HASH_ENCODING, else base64)
#
#   Simulator routing headers (inquiry/payment/status only):
#   -m  company-code (default: partnerServiceId with its padding trimmed)
#   -K  client-id    (default: the env file's clientId)
#   -P  product-id   (default: per endpoint, see DEFAULT_PRODUCT_ID below)
#   -X  xml-response (default N — ask for a JSON reply)
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
# Tracks whether -q was passed, as opposed to a paymentRequestId this script
# generated. The status payload only carries the field when the caller has a
# real one to quote: VA-Payment-Status V2 v1.0's request table lists four
# fields and paymentRequestId is not among them.
PAYMENT_REQUEST_ID_GIVEN=0
CHANNEL_ID="95231"
PARTNER_ID="1-MANJO-SNAP"
# Fields BCA's PaymentRequest table marks Mandatory (Y) that the wider SNAP
# standard leaves optional. A vendor configured with
# VENDOR_STRICT_MANDATORY_FIELDS=true — which is what .env.bca.va sets, and
# therefore what BCA conformance actually looks like — rejects a payment
# without them (4002502). channelCode is Mandatory on the v2.4 inquiry payload
# too, so the same value serves both.
CHANNEL_CODE="6011"
FLAG_ADVISE="N"
REFERENCE_NO=""
BODY_HASH_ENCODING=""
# The Optional half of BCA's payloads. They are sent — as empty strings, or as
# the sample's own values — rather than omitted, because BCA's request samples
# spell every one of them out and a simulator that emits a shorter object
# teaches an integrator a body shape the real channel never sends. Everything
# here is Optional (N), so a blank value is as conformant as an absent key.
LANGUAGE=""
SOURCE_BANK_CODE="014"
HASHED_SOURCE_ACCOUNT_NO=""
PASS_APP=""
# Sub-company 00000 is BCA's documented default ("the default sub-company code
# (00000) based on data recorded in BCA", VA-Payment-Flag v2.3), and is what
# subCompanyForVA falls back to on this side too.
SUB_COMPANY="00000"

# Headers the ASPI/BCA client simulator routes on. They appear in no BCA field
# table — the simulator uses them to pick which biller and which service the
# call belongs to, then generates Authorization/X-TIMESTAMP/X-SIGNATURE itself
# from the client-id's onboarded key.
COMPANY_CODE=""
CLIENT_ID_HEADER=""
PRODUCT_ID=""
XML_RESPONSE="N"
SIM_HEADERS=()

usage() {
	echo "Usage: $0 -e <token|create-va|inquiry|payment|status|delete-va> -f <env-file> [-u <base-url>] [-s <partnerServiceId>] [-c <customerNo>] [-v <virtualAccountNo>] [-n <name>] [-a <amount>] [-t <trxId>] [-q <paymentRequestId>] [-r <inquiryRequestId>] [-C <channelCode>] [-A <flagAdvise>] [-R <referenceNo>] [-L <language>] [-B <sourceBankCode>] [-H <hex|base64>] [-m <company-code>] [-K <client-id>] [-P <product-id>] [-X <Y|N>]" >&2
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

while getopts "e:f:u:s:c:v:n:a:t:q:r:i:p:C:A:R:L:B:H:m:K:P:X:h" opt; do
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
	q) PAYMENT_REQUEST_ID="$OPTARG"; PAYMENT_REQUEST_ID_GIVEN=1 ;;
	r) INQUIRY_REQUEST_ID="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	C) CHANNEL_CODE="$OPTARG" ;;
	A) FLAG_ADVISE="$OPTARG" ;;
	R) REFERENCE_NO="$OPTARG" ;;
	L) LANGUAGE="$OPTARG" ;;
	B) SOURCE_BANK_CODE="$OPTARG" ;;
	H) BODY_HASH_ENCODING="$OPTARG" ;;
	m) COMPANY_CODE="$OPTARG" ;;
	K) CLIENT_ID_HEADER="$OPTARG" ;;
	P) PRODUCT_ID="$OPTARG" ;;
	X) XML_RESPONSE="$OPTARG" ;;
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

# The RequestBody component of stringToSign is encoded as lowercase hex per
# BCA's Signature Symmetric spec, or base64 per feature 012-base64-hash-encoding
# — whichever the side verifying this request was onboarded with. That is a
# property of the counterparty, not of the endpoint, so it is chosen explicitly
# here. Picking the wrong one surfaces as a permanent
# "Unauthorized. [Invalid signature]" and never as a field error, which is why
# -H exists: flipping it is the first thing to try against such a 401.
#
# Precedence: -H, then the env file's own VENDOR_BODY_HASH_ENCODING (so a
# vendor file signs the way that vendor is configured to verify), then base64 —
# what MerchantAuthMiddleware always uses for create-va/delete-va, and what
# .env.bca.va sets. X-SIGNATURE itself is always base64, regardless.
#
# Resolved before the token fetch below so a bad -H fails instantly rather than
# after a network round trip.
[[ -z "$BODY_HASH_ENCODING" ]] && BODY_HASH_ENCODING="$(read_env_var "$ENV_FILE" VENDOR_BODY_HASH_ENCODING || true)"
BODY_HASH_ENCODING="${BODY_HASH_ENCODING:-base64}"
# Matched case-insensitively, the way crypto.HashRequestBody compares it. An
# unrecognized value is rejected rather than silently defaulted: the server
# would fall back to hex, and a typo that quietly changes the digest is the
# hardest possible way to debug a 401.
case "${BODY_HASH_ENCODING,,}" in
hex)    BODY_HASH_ENCODER="xxd -p -c 256" ;;
base64) BODY_HASH_ENCODER="openssl base64 -A" ;;
*)
	echo "!! unknown body-hash encoding: ${BODY_HASH_ENCODING} (expected hex or base64)" >&2
	exit 1
	;;
esac

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
	# trxDateInit, not txnDateInit — BCA renamed the field at VA-BillPresentment
	# v1.6 and both it and channelCode are Mandatory (Y) in v2.4, so a vendor
	# with VENDOR_STRICT_MANDATORY_FIELDS answers 4002402 without them.
	#
	# Field ORDER below is BCA's own request sample, verbatim (v2.4 p.14):
	# identity → trxDateInit → channelCode → language → amount →
	# hashedSourceAccountNo → sourceBankCode → additionalInfo → passApp →
	# inquiryRequestId. Order is cosmetic to the server (JSON objects are
	# unordered, and the signature covers whatever bytes are emitted here) but
	# not to a human diffing this against the PDF, which is the entire point of
	# a copy-paste simulator payload.
	#
	# amount stays an object here rather than the sample's null: the sample
	# shows a channel that carries no customer-entered amount, while -a exists
	# precisely to send one. Pass -a "" for the null form.
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg d "$TIMESTAMP" --arg a "$AMOUNT" --arg r "$INQUIRY_REQUEST_ID" \
		--argjson ch "$CHANNEL_CODE" --arg lang "$LANGUAGE" \
		--arg hs "$HASHED_SOURCE_ACCOUNT_NO" --arg sb "$SOURCE_BANK_CODE" \
		--arg pa "$PASS_APP" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,trxDateInit:$d,channelCode:$ch,language:$lang}
		 + {amount:(if $a == "" then null else {value:$a,currency:"IDR"} end)}
		 + {hashedSourceAccountNo:$hs,sourceBankCode:$sb,additionalInfo:{},passApp:$pa,inquiryRequestId:$r}')"
	;;
payment)
	EP="/openapi/v1.0/transfer-va/payment"
	# paymentRequestId must equal the inquiry's inquiryRequestId when the
	# payment follows an inquiry (ASPI PaymentRequest); trxId is mandatory when
	# it follows a create-VA. Both come from the earlier responses via -q/-t.
	#
	# virtualAccountName, channelCode and flagAdvise are here because BCA marks
	# them Mandatory (Y) on service 25 and domain.ValidatePaymentRequest
	# enforces that set for any vendor with VENDOR_STRICT_MANDATORY_FIELDS=true
	# — .env.bca.va does. Without them the request signs and authenticates
	# fine and is then rejected 4002502, which reads like a spec disagreement
	# rather than a missing field the simulator never sent.
	[[ -z "$PAYMENT_REQUEST_ID" ]] && PAYMENT_REQUEST_ID="PAY-$(date +%s)$((RANDOM % 9000 + 1000))"
	# referenceNo is String(11) Fixed, NUMERIC — "Payment auth code generated by
	# BCA", Mandatory for a non-multibill transaction. It used to be generated
	# as "R" + epoch, which is 10 characters and not a number: short enough to
	# clear the length check here and wrong in exactly the way a channel's own
	# reference never is. Eleven digits from the clock, zero-padded.
	[[ -z "$REFERENCE_NO" ]] && REFERENCE_NO="$(printf '%011d' "$(( ($(date +%s) % 100000000) * 100 + RANDOM % 100 ))")"
	#
	# Field order follows VA-Payment-Flag v2.3's request sample (p.20-21), which
	# also sends every Optional field as an empty string rather than omitting
	# it — trxId included, since BCA's example payment did not come from a
	# create-VA. Sent the same way here: "" and an absent key both decode to the
	# empty string, so the create-VA link is carried by -t's VALUE and nothing
	# is lost by always emitting the key ("Mandatory if payment comes from the
	# Create VA Request" — domain.ValidatePaymentRequest leaves it optional).
	#
	# cumulativePaymentAmount is null, not {} — the sample's own value, and the
	# shape *Amount unmarshals cleanly from.
	#
	# billDetails is [null] rather than [] or a populated array. That is what
	# the simulator emits for a non-multibill payment, and reproducing it is the
	# point: a JSON null in a non-pointer struct slice decodes to a zero-value
	# element, so len() reports 1 where the channel meant 0 — the exact case
	# VAPaymentRequest.NormalizeBillDetails exists to collapse. Emitting []
	# would sidestep that path and test a shape real traffic never sends.
	# Inventing a bill instead is worse still: its billNo would contradict the
	# bill the PSP actually holds for this VA.
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg t "$TRX_ID" --arg q "$PAYMENT_REQUEST_ID" --arg a "$AMOUNT" \
		--arg d "$TIMESTAMP" --arg n "$REFERENCE_NO" \
		--arg vn "$VA_NAME" --argjson ch "$CHANNEL_CODE" --arg fa "$FLAG_ADVISE" \
		--arg hs "$HASHED_SOURCE_ACCOUNT_NO" --arg sb "$SOURCE_BANK_CODE" \
		--arg sc "$SUB_COMPANY" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,virtualAccountName:$vn,virtualAccountEmail:"",virtualAccountPhone:"",trxId:$t,
		    paymentRequestId:$q,channelCode:$ch,hashedSourceAccountNo:$hs,sourceBankCode:$sb,
		    paidAmount:{value:$a,currency:"IDR"},cumulativePaymentAmount:null,paidBills:"",
		    totalAmount:{value:$a,currency:"IDR"},trxDateTime:$d,referenceNo:$n,
		    journalNum:"",paymentType:"",flagAdvise:$fa,subCompany:$sc,
		    billDetails:[null],freeTexts:[],additionalInfo:{}}')"
	;;
status)
	EP="/openapi/v1.0/transfer-va/status"
	[[ -z "$INQUIRY_REQUEST_ID" ]] && { echo "!! -r <inquiryRequestId> is required for status" >&2; exit 1; }
	# VA-Payment-Status V2 v1.0's request table is four fields —
	# partnerServiceId, customerNo, virtualAccountNo, inquiryRequestId — plus
	# additionalInfo. paymentRequestId is NOT among them, and
	# ValidateStatusRequest does not read it, so it is emitted only when -q
	# named a real one; it used to be defaulted to the inquiryRequestId, which
	# put a field in the body that no channel sends and that the PSP resolves
	# for itself.
	BODY="$(jq -cn --arg p "$PARTNER_SERVICE_ID" --arg c "$CUSTOMER_NO" --arg v "$VA_NO" \
		--arg r "$INQUIRY_REQUEST_ID" --arg q "$PAYMENT_REQUEST_ID" \
		--argjson pq "$PAYMENT_REQUEST_ID_GIVEN" \
		'{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,inquiryRequestId:$r}
		 + (if $pq == 1 then {paymentRequestId:$q} else {} end)
		 + {additionalInfo:{}}')"
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
# `jq -cj .` is the MinifyJson step and is load-bearing: the server hashes the
# minified body, so hashing $BODY raw (it is pretty-printed here) yields a
# different digest and every request comes back 401. -j (not just -c)
# suppresses jq's trailing newline, which would otherwise be hashed too.
BODY_HASH="$(printf '%s' "$BODY" | jq -cj . | openssl dgst -sha256 -binary | ${BODY_HASH_ENCODER})"
STRING_TO_SIGN="${METHOD}:${EP}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

# ------------------------------------------------- simulator routing headers
#
# The ASPI/BCA client simulator needs four headers that appear in no BCA field
# table and that this PSP never reads: company-code and product-id tell it which
# biller and which service the call is for, client-id selects the onboarded key
# it signs with, and xml-response: N asks for a JSON reply rather than XML.
#
# They are emitted for the three transfer-va services only. create-va/delete-va
# are merchant-side routes on this PSP, not services the simulator fronts, and
# `-e token` returns above before reaching here.
#
# Sending them costs nothing on the wire: SNAPAuthMiddleware's mandatory-header
# check walks the DOCUMENTED set and skips anything outside it
# (isDocumentedTransferVAHeader), and X-SIGNATURE covers the body and four
# named components — never the full header set. So one emitted request pastes
# into the simulator and curls straight at this PSP unchanged.
#
# The two values below are transcribed from real simulator calls:
# OPENAPI.VA-BILLPRESENTMENT for inquiry and OPENAPI.VA-PAYMENT for payment —
# note the latter is NOT "OPENAPI.VA-PAYMENT-FLAG", even though the document
# describing that service is called VA-Payment-Flag. The status value has not
# been seen on the wire and is inferred from the same pattern, so confirm it
# against a real simulator call before trusting `-e status`; -P overrides it.
case "$ENDPOINT_NAME" in
inquiry) DEFAULT_PRODUCT_ID="OPENAPI.VA-BILLPRESENTMENT" ;;
payment) DEFAULT_PRODUCT_ID="OPENAPI.VA-PAYMENT" ;;
status)  DEFAULT_PRODUCT_ID="OPENAPI.VA-PAYMENT-STATUS" ;; # unconfirmed
*)       DEFAULT_PRODUCT_ID="" ;;
esac

if [[ -n "$DEFAULT_PRODUCT_ID" ]]; then
	# company-code is the biller's code — partnerServiceId without the 8-char
	# left space padding that the body keeps.
	[[ -z "$COMPANY_CODE" ]] && COMPANY_CODE="$(printf '%s' "$PARTNER_SERVICE_ID" | tr -d '[:space:]')"
	[[ -z "$CLIENT_ID_HEADER" ]] && CLIENT_ID_HEADER="$CLIENT_ID"
	[[ -z "$PRODUCT_ID" ]] && PRODUCT_ID="$DEFAULT_PRODUCT_ID"
	SIM_HEADERS=(
		"xml-response: ${XML_RESPONSE}"
		"company-code: ${COMPANY_CODE}"
		"product-id: ${PRODUCT_ID}"
		"client-id: ${CLIENT_ID_HEADER}"
	)
fi

# No X-CLIENT-KEY here. It belongs to the access-token endpoint alone (emitted
# above for `-e token`); BCA's transfer-va header tables are closed sets and do
# not list it, so a simulator that emits it teaches an integrator to send a
# header the real channel never sends.
emit "$METHOD" "${BASE_URL}${EP}" "$BODY" \
	"Content-Type: application/json" \
	"Authorization: Bearer ${ACCESS_TOKEN}" \
	"X-TIMESTAMP: ${TIMESTAMP}" \
	"X-SIGNATURE: ${SIGNATURE}" \
	"X-PARTNER-ID: ${PARTNER_ID}" \
	"X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	"CHANNEL-ID: ${CHANNEL_ID}" \
	"${SIM_HEADERS[@]}"

echo
echo "stringToSign (for debugging a signature mismatch, body hash encoded as ${BODY_HASH_ENCODING,,}):"
echo "${STRING_TO_SIGN}"
