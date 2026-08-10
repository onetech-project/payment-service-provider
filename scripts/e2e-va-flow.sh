#!/usr/bin/env bash
#
# End-to-end VA flow: create a VA, inquire it, pay it, and show the merchant
# payment callback arriving.
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
#   1. merchant-create-va.sh  POST /openapi/v1.0/transfer-va/create-va   (create the VA; fetches its own accessToken)
#   2. vendor-inquiry-va.sh   POST /openapi/v1.0/transfer-va/inquiry     (inquire the VA)
#   3. vendor-payment-va.sh   POST /openapi/v1.0/transfer-va/payment     (pay the VA)
#   4. (local callback listener) shows the async merchant notification
#      (internal/usecase/va_usecase.go notifyMerchantWithVA -> Asynq queue ->
#      payment_notification_worker) actually being delivered.
#
# Usage:
#   ./scripts/e2e-va-flow.sh -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> \
#       -m <.env.merchant.NAME> -f <.env.vendor.channel> \
#       [-a <amount>] [-v <virtualAccountNo>] [-t <trxId>] [-w <notificationUrl>] \
#       [-b <billNo>] [-d <billName>] [-y <vaType>] [-N <payments>] \
#       [-L <listener-port>] [-u <base-url>] [-O <transcript-file>]
#
# -y <vaType> routes the create-va through one of the six static/dynamic VA
# type combinations (feature 006-static-dynamic-va). Omit it for an unmanaged
# (legacy) VA on a non-reserved partnerServiceId.
#
# For NO-BILL types (01 static, 04 dynamic) the flow changes shape, per feature
# 013-no-bill-payment-transaction:
#   - -a is dropped from create-va: a no-bill VA carries no bill, and sending
#     totalAmount is rejected with 4002706.
#   - create-va writes only the VA registration — no transaction — so this
#     script asserts a transaction count of 0 before any payment.
#   - -N <payments> (default 3 for no-bill, 1 otherwise) repeats step 3 with a
#     distinct paymentRequestId each time, proving the same VA number stays
#     payable. Each payment becomes its own transaction.
# The per-VA and per-payment counts are then read back through the merchant
# listings, so the assertions come from the API rather than from the database.
#
# -O writes a full transcript of every request (URL, headers, stringToSign,
# body) and response to <transcript-file> while still printing to the
# terminal — useful as evidence when comparing behaviour against the ASPI
# spec or when reporting an issue to a vendor. NOTE: it contains live
# accessTokens; treat the file as sensitive.
#
# -m is passed straight through to merchant-create-va.sh's -f.
# -f is passed straight through to vendor-inquiry-va.sh / vendor-payment-va.sh's -f.
#
# -b/-d attach one billDetails entry to the create-va call (see
# merchant-create-va.sh), so this flow can also exercise bill-detail
# persistence (SaveBillDetails) end-to-end, not just the VA row itself.
#
# Callback verification: if -w is NOT given, this script starts a throwaway
# local HTTP listener (python3) and registers ITS URL as the VA's
# notificationUrl, so the merchant callback that vendor-payment-va.sh
# triggers has somewhere of ours to land — its raw payload is then printed to
# the terminal in step 4. If the PSP API runs in a container/host that can't
# reach 127.0.0.1 on this machine (e.g. a separate Docker network), pass your
# own reachable -w instead; step 4 will then just remind you to check it
# manually instead of polling. -L overrides the local listener's port
# (default 8099).
#
# Requires: curl, openssl, uuidgen, jq, python3 (only for the local listener)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="http://localhost:8080"
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NAME=""
AMOUNT="100000.00"
VA_NO=""
TRX_ID=""
NOTIFICATION_URL=""
BILL_NO=""
BILL_NAME=""
MERCHANT_ENV_FILE=""
VENDOR_ENV_FILE=""
LISTENER_PORT="8099"
OUTPUT_FILE=""
VA_TYPE=""
PAYMENT_COUNT=""

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -m <.env.merchant.NAME> -f <.env.vendor.channel> [-a <amount>] [-v <virtualAccountNo>] [-t <trxId>] [-w <notificationUrl>] [-b <billNo>] [-d <billName>] [-y <vaType>] [-N <payments>] [-L <listener-port>] [-u <base-url>] [-O <transcript-file>]" >&2
	exit 1
}

while getopts "s:c:n:a:v:t:w:m:f:b:d:y:N:L:u:O:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	w) NOTIFICATION_URL="$OPTARG" ;;
	m) MERCHANT_ENV_FILE="$OPTARG" ;;
	f) VENDOR_ENV_FILE="$OPTARG" ;;
	b) BILL_NO="$OPTARG" ;;
	d) BILL_NAME="$OPTARG" ;;
	y) VA_TYPE="$OPTARG" ;;
	N) PAYMENT_COUNT="$OPTARG" ;;
	L) LISTENER_PORT="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	O) OUTPUT_FILE="$OPTARG" ;;
	h | *) usage ;;
	esac
done

# No-bill VA types (feature 013-no-bill-payment-transaction). These are the
# only types where create-va writes no transaction and the VA stays payable
# indefinitely, so they take a different shape through this script.
IS_NO_BILL=false
case "$VA_TYPE" in
01 | 04) IS_NO_BILL=true ;;
esac

# Dynamic VA types generate customerNo server-side, so -c must be EMPTY on the
# create-va call (4002703 otherwise). The generated value is read back from the
# response and used for the inquiry/payment steps.
IS_DYNAMIC=false
case "$VA_TYPE" in
04 | 05 | 06) IS_DYNAMIC=true ;;
esac

if [[ -z "$PAYMENT_COUNT" ]]; then
	# Repeat payments by default for no-bill: a single payment would not
	# demonstrate the property that actually matters here.
	if [[ "$IS_NO_BILL" == true ]]; then PAYMENT_COUNT=3; else PAYMENT_COUNT=1; fi
fi

CHECKS_PASSED=0
CHECKS_FAILED=0
check() {
	local label="$1" ok="$2"
	if [[ "$ok" == true ]]; then
		echo "   [PASS] $label"
		CHECKS_PASSED=$((CHECKS_PASSED + 1))
	else
		echo "   [FAIL] $label"
		CHECKS_FAILED=$((CHECKS_FAILED + 1))
	fi
}

# transaction_count_for <virtualAccountNo> — asks the merchant transaction
# listing how many transactions exist for a VA. Used instead of a direct
# database query so the assertions run against the real API surface.
transaction_count_for() {
	"$SCRIPT_DIR/merchant-list-transactions.sh" -v "$1" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL" 2>/dev/null |
		jq -r '.pagination.totalRows // "?"'
}

# -O captures the whole run — every request (URL, headers, stringToSign, body)
# and every response — into one transcript, while still streaming to the
# terminal. The sub-scripts already print their request diagnostics to stderr
# and their response JSON to stdout, so both streams are merged here rather
# than reimplementing the logging inside each script.
#
# The transcript contains live bearer accessTokens (inside stringToSign and
# the Authorization diagnostics). They expire in 15 minutes, but treat the
# file as sensitive — don't paste it into a public issue verbatim.
if [[ -n "$OUTPUT_FILE" ]]; then
	: >"$OUTPUT_FILE" || { echo "cannot write transcript: $OUTPUT_FILE" >&2; exit 1; }
	exec > >(tee -a "$OUTPUT_FILE") 2>&1
fi

# customerNo is deliberately exempt for dynamic types: the server generates it.
[[ -z "$PARTNER_SERVICE_ID" || -z "$VA_NAME" || -z "$MERCHANT_ENV_FILE" || -z "$VENDOR_ENV_FILE" ]] && usage
[[ "$IS_DYNAMIC" == false && -z "$CUSTOMER_NO" ]] && usage
[[ -f "$MERCHANT_ENV_FILE" ]] || { echo "merchant env file not found: $MERCHANT_ENV_FILE (run onboard-merchant.sh first)" >&2; exit 1; }
[[ -f "$VENDOR_ENV_FILE" ]] || { echo "vendor env file not found: $VENDOR_ENV_FILE (run onboard-vendor.sh first)" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required for this script" >&2; exit 1; }

USING_LOCAL_LISTENER=false
CALLBACK_LOG=""
LISTENER_PID=""

cleanup() {
	if [[ -n "$LISTENER_PID" ]]; then
		kill "$LISTENER_PID" 2>/dev/null || true
		wait "$LISTENER_PID" 2>/dev/null || true
	fi
	# Must be an `if`, not `[[ ... ]] && rm`: an EXIT trap's final command
	# status becomes the script's exit status, so the && form returned 1 —
	# and reported failure — on every otherwise-successful run that passed
	# -w (no local listener, so CALLBACK_LOG is empty and the test is false).
	if [[ -n "$CALLBACK_LOG" && -f "$CALLBACK_LOG" ]]; then
		rm -f "$CALLBACK_LOG"
	fi
}
trap cleanup EXIT

if [[ -z "$NOTIFICATION_URL" ]]; then
	if command -v python3 >/dev/null; then
		CALLBACK_LOG="$(mktemp)"
		python3 - "$LISTENER_PORT" "$CALLBACK_LOG" >/dev/null 2>&1 <<'PYEOF' &
import sys, http.server

port = int(sys.argv[1])
logfile = sys.argv[2]

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length).decode('utf-8', errors='replace')
        with open(logfile, 'a') as f:
            f.write(body + "\n")
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(b'{"status":"received"}')

    def log_message(self, format, *args):
        pass

http.server.HTTPServer(('127.0.0.1', port), Handler).serve_forever()
PYEOF
		LISTENER_PID=$!
		sleep 0.3
		if kill -0 "$LISTENER_PID" 2>/dev/null; then
			NOTIFICATION_URL="http://127.0.0.1:${LISTENER_PORT}/callback"
			USING_LOCAL_LISTENER=true
			echo "==> Local callback listener started: ${NOTIFICATION_URL} (pid ${LISTENER_PID})"
			echo "    (only reachable if the PSP API can reach 127.0.0.1 on this machine —"
			echo "     pass your own -w <notificationUrl> if the API runs elsewhere, e.g. Docker)"
		else
			echo "!! Local callback listener failed to start on port ${LISTENER_PORT} (in use?) — proceeding without one." >&2
			LISTENER_PID=""
		fi
	else
		echo "!! python3 not found — cannot start a local callback listener. Pass -w <notificationUrl> if you want to verify the callback yourself." >&2
	fi
fi
echo

echo "=================================================================="
echo "Step 1/4: POST /openapi/v1.0/transfer-va/create-va (merchant identity)"
echo "=================================================================="
CREATE_VA_ARGS=(-s "$PARTNER_SERVICE_ID" -n "$VA_NAME" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL")
# Dynamic types must send an EMPTY customerNo — the server assigns it.
[[ "$IS_DYNAMIC" == false ]] && CREATE_VA_ARGS+=(-c "$CUSTOMER_NO")
# A no-bill VA carries no bill: sending totalAmount is rejected with 4002706.
[[ "$IS_NO_BILL" == false ]] && CREATE_VA_ARGS+=(-a "$AMOUNT")
[[ -n "$VA_TYPE" ]] && CREATE_VA_ARGS+=(-y "$VA_TYPE")
[[ -n "$VA_NO" ]] && CREATE_VA_ARGS+=(-v "$VA_NO")
[[ -n "$TRX_ID" ]] && CREATE_VA_ARGS+=(-t "$TRX_ID")
[[ -n "$NOTIFICATION_URL" ]] && CREATE_VA_ARGS+=(-w "$NOTIFICATION_URL")
[[ -n "$BILL_NO" ]] && CREATE_VA_ARGS+=(-b "$BILL_NO")
[[ -n "$BILL_NAME" ]] && CREATE_VA_ARGS+=(-d "$BILL_NAME")

CREATE_VA_RESPONSE="$("$SCRIPT_DIR/merchant-create-va.sh" "${CREATE_VA_ARGS[@]}")"
echo "$CREATE_VA_RESPONSE" | jq .

RESPONSE_CODE="$(echo "$CREATE_VA_RESPONSE" | jq -r '.responseCode // empty')"
if [[ "$RESPONSE_CODE" != 2* ]]; then
	echo "!! create-va did not return a success responseCode (got: ${RESPONSE_CODE:-<none>}) — aborting." >&2
	exit 1
fi

# Prefer the server-confirmed virtualAccountNo over our local default, since
# the server is authoritative on what was actually persisted.
CONFIRMED_VA_NO="$(echo "$CREATE_VA_RESPONSE" | jq -r '.virtualAccountData.virtualAccountNo // empty')"
if [[ -n "$CONFIRMED_VA_NO" ]]; then
	VA_NO="$CONFIRMED_VA_NO"
elif [[ -z "$VA_NO" ]]; then
	VA_NO="${PARTNER_SERVICE_ID}${CUSTOMER_NO}"
fi
echo "==> virtualAccountNo: ${VA_NO}"

# For dynamic types the server assigned the customerNo — the inquiry and
# payment steps must use that, not our (empty) input.
CONFIRMED_CUSTOMER_NO="$(echo "$CREATE_VA_RESPONSE" | jq -r '.virtualAccountData.customerNo // empty')"
if [[ -n "$CONFIRMED_CUSTOMER_NO" ]]; then
	CUSTOMER_NO="$CONFIRMED_CUSTOMER_NO"
	echo "==> customerNo: ${CUSTOMER_NO}"
fi

# The create-va trxId is what step 3 must send back as PaymentRequest.trxId —
# per ASPI that field is "Mandatory if Payment comes from the Create VA
# Request", which is exactly this flow. Inventing a fresh id there instead
# would leave the payment unlinked to the VA the merchant created.
CONFIRMED_TRX_ID="$(echo "$CREATE_VA_RESPONSE" | jq -r '.virtualAccountData.trxId // empty')"
echo "==> trxId: ${CONFIRMED_TRX_ID}"

# The defining property of a no-bill VA: create-va registers the VA number and
# creates NO transaction. Before feature 013-no-bill-payment-transaction this
# count would have been 1 (a pending transaction), which is exactly what made
# the VA payable only once.
if [[ "$IS_NO_BILL" == true ]]; then
	TXN_AFTER_CREATE="$(transaction_count_for "$VA_NO")"
	echo "==> transactions after create-va: ${TXN_AFTER_CREATE}"
	check "no-bill create-va wrote NO transaction (expect 0, got ${TXN_AFTER_CREATE})" \
		"$([[ "$TXN_AFTER_CREATE" == "0" ]] && echo true || echo false)"
fi
echo

echo "=================================================================="
echo "Step 2/4: POST /openapi/v1.0/transfer-va/inquiry (vendor identity)"
echo "=================================================================="
INQUIRY_RESPONSE="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")"
echo "$INQUIRY_RESPONSE" | jq .

# Assert the inquiry actually succeeded. Without this the flow reported "done"
# even when every inquiry was being rejected, because only create-va and
# payment were ever checked — and a rejected inquiry still leaves
# CONFIRMED_INQUIRY_REQUEST_ID empty, which silently turns step 3 into a
# no-inquiry payment instead of the inquiry-then-pay flow this script claims
# to exercise. A no-bill VA has no bill to present, so it is exempt.
INQUIRY_CODE="$(echo "$INQUIRY_RESPONSE" | jq -r '.responseCode // empty')"
if [[ "$IS_NO_BILL" != true && "$INQUIRY_CODE" != 2* ]]; then
	echo "!! inquiry did not succeed (got: ${INQUIRY_CODE:-<none>}) — aborting." >&2
	exit 1
fi

# ASPI PaymentRequest.paymentRequestId: "If Payment comes from the Inquiry
# process, this value must be the same with inquiryRequestId" — which is this
# flow, so step 3 reuses the id this inquiry was keyed on.
CONFIRMED_INQUIRY_REQUEST_ID="$(echo "$INQUIRY_RESPONSE" | jq -r '.virtualAccountData.inquiryRequestId // empty')"
echo

echo "=================================================================="
echo "Step 3/4: POST /openapi/v1.0/transfer-va/payment (vendor identity)"
echo "=================================================================="
for ((PAY_N = 1; PAY_N <= PAYMENT_COUNT; PAY_N++)); do
	[[ "$PAYMENT_COUNT" -gt 1 ]] && echo "--- payment ${PAY_N}/${PAYMENT_COUNT} ---"

	PAYMENT_ARGS=(-s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -f "$VENDOR_ENV_FILE" -u "$BASE_URL")
	[[ -n "$CONFIRMED_TRX_ID" ]] && PAYMENT_ARGS+=(-t "$CONFIRMED_TRX_ID")
	if [[ "$PAY_N" -eq 1 && -n "$CONFIRMED_INQUIRY_REQUEST_ID" ]]; then
		# ASPI: when the payment follows an inquiry, paymentRequestId must equal
		# that inquiryRequestId.
		PAYMENT_ARGS+=(-q "$CONFIRMED_INQUIRY_REQUEST_ID")
	else
		# Each repeat payment is a distinct payment and must carry its own
		# paymentRequestId — reusing one would (correctly) replay the first.
		PAYMENT_ARGS+=(-q "PAY-$(date +%s%N)-${PAY_N}")
	fi

	PAYMENT_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" "${PAYMENT_ARGS[@]}")"
	echo "$PAYMENT_RESPONSE" | jq .

	PAYMENT_CODE="$(echo "$PAYMENT_RESPONSE" | jq -r '.responseCode // empty')"
	if [[ "$PAYMENT_COUNT" -gt 1 ]]; then
		check "payment ${PAY_N} succeeded (got ${PAYMENT_CODE:-<none>})" \
			"$([[ "$PAYMENT_CODE" == 2* ]] && echo true || echo false)"
	fi
	if [[ "$PAYMENT_CODE" != 2* ]]; then
		echo "!! payment ${PAY_N} did not return a success responseCode (got: ${PAYMENT_CODE:-<none>}) — aborting before callback check." >&2
		exit 1
	fi
	echo
done

# One VA, N transactions — each payment created its own. Before feature
# 013-no-bill-payment-transaction, payment 2 here would have been rejected
# with 4092500 "already paid or inactive".
if [[ "$IS_NO_BILL" == true ]]; then
	TXN_AFTER_PAYMENTS="$(transaction_count_for "$VA_NO")"
	echo "==> transactions after ${PAYMENT_COUNT} payment(s): ${TXN_AFTER_PAYMENTS}"
	check "each payment created its own transaction (expect ${PAYMENT_COUNT}, got ${TXN_AFTER_PAYMENTS})" \
		"$([[ "$TXN_AFTER_PAYMENTS" == "$PAYMENT_COUNT" ]] && echo true || echo false)"

	VA_ROWS="$("$SCRIPT_DIR/merchant-list-va.sh" -s "$PARTNER_SERVICE_ID" -v "$VA_NO" -f "$MERCHANT_ENV_FILE" -u "$BASE_URL" 2>/dev/null |
		jq -r '.pagination.totalRows // "?"')"
	check "still exactly ONE registered VA after ${PAYMENT_COUNT} payments (expect 1, got ${VA_ROWS})" \
		"$([[ "$VA_ROWS" == "1" ]] && echo true || echo false)"
	echo
fi

echo "=================================================================="
echo "Step 4/4: Merchant payment callback"
echo "=================================================================="
if [[ "$USING_LOCAL_LISTENER" == true ]]; then
	echo "==> Waiting for the async callback (Asynq -> payment_notification_worker) to reach ${NOTIFICATION_URL} ..."
	CALLBACK_RECEIVED=false
	for _ in $(seq 1 20); do
		if [[ -s "$CALLBACK_LOG" ]]; then
			CALLBACK_RECEIVED=true
			break
		fi
		sleep 0.5
	done

	if [[ "$CALLBACK_RECEIVED" == true ]]; then
		echo "==> Callback received by merchant:"
		tail -n1 "$CALLBACK_LOG" | jq .
	else
		echo "!! No callback arrived within 10s. Possible causes:" >&2
		echo "   - the payment_notification_worker isn't running (check the Asynq queue/worker process)" >&2
		echo "   - the PSP API can't reach 127.0.0.1 on this machine (e.g. it's running in Docker — pass your own -w)" >&2
		echo "   - the VA had no notificationUrl registered before this run (re-run without an existing -v)" >&2
	fi
else
	echo "==> Using your own notificationUrl (${NOTIFICATION_URL}) — check that endpoint yourself for the callback;"
	echo "    this script only polls its own local listener, which wasn't used this run."
fi

echo
echo "=================================================================="
echo "Done: VA ${VA_NO} created -> inquiry confirmed -> paid -> callback checked."
if [[ $((CHECKS_PASSED + CHECKS_FAILED)) -gt 0 ]]; then
	echo "Assertions: ${CHECKS_PASSED} passed, ${CHECKS_FAILED} failed"
fi
echo "=================================================================="

[[ "$CHECKS_FAILED" -gt 0 ]] && exit 1
exit 0
