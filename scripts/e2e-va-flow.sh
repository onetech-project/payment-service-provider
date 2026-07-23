#!/usr/bin/env bash
#
# End-to-end VA flow: get a B2B access token, create a VA, inquire it, pay it,
# and show the merchant payment callback arriving.
#
# Chains together, in order:
#   1. curl-b2b-token.sh      POST /v1.0/access-token/b2b        (get accessToken)
#   2. merchant-create-va.sh  POST /v1.0/transfer-va/create-va   (create the VA, using the token)
#   3. vendor-inquiry-va.sh   POST /v1.0/transfer-va/inquiry     (inquire the VA, using the token)
#   4. vendor-payment-va.sh   POST /v1.0/transfer-va/payment     (pay the VA, using the token)
#   5. (local callback listener) shows the async merchant notification
#      (internal/usecase/va_usecase.go notifyMerchantWithVA -> Asynq queue ->
#      payment_notification_worker) actually being delivered.
#
# Each step's JSON response is captured (diagnostics from the sub-scripts go
# to stderr, so stdout stays clean JSON) and the accessToken / virtualAccountNo
# are threaded automatically into the next step — no manual copy-pasting.
#
# Usage:
#   ./scripts/e2e-va-flow.sh -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> \
#       -i <client_id> -k <private_key.pem> (-e <client-secret> | -f <env-file>) \
#       [-a <amount>] [-v <virtualAccountNo>] [-t <trxId>] [-w <notificationUrl>] \
#       [-b <billNo>] [-d <billName>] [-L <listener-port>] [-u <base-url>]
#
# -i/-k are for the B2B access-token call (asymmetric RSA signing, see
# curl-b2b-token.sh). -e/-f are for the create-va/inquiry/payment HMAC signing
# (see merchant-create-va.sh / vendor-inquiry-va.sh / vendor-payment-va.sh) —
# -f loads VENDOR_CLIENT_SECRET straight out of a .env.<vendor>.<channel> file
# instead of typing it raw.
#
# -b/-d attach one billDetails entry to the create-va call (see
# merchant-create-va.sh), so this flow can also exercise bill-detail
# persistence (SaveBillDetails) end-to-end, not just the VA row itself.
#
# Callback verification: if -w is NOT given, this script starts a throwaway
# local HTTP listener (python3) and registers ITS URL as the VA's
# notificationUrl, so the merchant callback that vendor-payment-va.sh
# triggers has somewhere of ours to land — its raw payload is then printed to
# the terminal in step 5. If the PSP API runs in a container/host that can't
# reach 127.0.0.1 on this machine (e.g. a separate Docker network), pass your
# own reachable -w instead; step 5 will then just remind you to check it
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
CLIENT_ID=""
PRIVATE_KEY_PATH=""
CLIENT_SECRET=""
ENV_FILE=""
LISTENER_PORT="8099"

usage() {
	echo "Usage: $0 -s <partnerServiceId> -c <customerNo> -n <virtualAccountName> -i <client_id> -k <private_key.pem> (-e <client-secret> | -f <env-file>) [-a <amount>] [-v <virtualAccountNo>] [-t <trxId>] [-w <notificationUrl>] [-b <billNo>] [-d <billName>] [-L <listener-port>] [-u <base-url>]" >&2
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

while getopts "s:c:n:a:v:t:w:i:k:e:f:b:d:L:u:h" opt; do
	case "$opt" in
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	n) VA_NAME="$OPTARG" ;;
	a) AMOUNT="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	t) TRX_ID="$OPTARG" ;;
	w) NOTIFICATION_URL="$OPTARG" ;;
	i) CLIENT_ID="$OPTARG" ;;
	k) PRIVATE_KEY_PATH="$OPTARG" ;;
	e) CLIENT_SECRET="$OPTARG" ;;
	f) ENV_FILE="$OPTARG" ;;
	b) BILL_NO="$OPTARG" ;;
	d) BILL_NAME="$OPTARG" ;;
	L) LISTENER_PORT="$OPTARG" ;;
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

USING_LOCAL_LISTENER=false
CALLBACK_LOG=""
LISTENER_PID=""

cleanup() {
	if [[ -n "$LISTENER_PID" ]]; then
		kill "$LISTENER_PID" 2>/dev/null || true
		wait "$LISTENER_PID" 2>/dev/null || true
	fi
	[[ -n "$CALLBACK_LOG" && -f "$CALLBACK_LOG" ]] && rm -f "$CALLBACK_LOG"
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
echo "Step 1/5: POST /v1.0/access-token/b2b"
echo "=================================================================="
TOKEN_RESPONSE="$("$SCRIPT_DIR/curl-b2b-token.sh" -i "$CLIENT_ID" -p "$PRIVATE_KEY_PATH" -u "$BASE_URL")"
echo "$TOKEN_RESPONSE" | jq .

ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty')"
if [[ -z "$ACCESS_TOKEN" ]]; then
	echo "!! Failed to obtain accessToken from step 1 response above — aborting." >&2
	exit 1
fi
echo "==> accessToken acquired: ${ACCESS_TOKEN:0:12}..."
echo

echo "=================================================================="
echo "Step 2/5: POST /v1.0/transfer-va/create-va"
echo "=================================================================="
CREATE_VA_ARGS=(-s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -n "$VA_NAME" -a "$AMOUNT" -e "$CLIENT_SECRET" -o "$ACCESS_TOKEN" -u "$BASE_URL")
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
echo

echo "=================================================================="
echo "Step 3/5: POST /v1.0/transfer-va/inquiry"
echo "=================================================================="
INQUIRY_RESPONSE="$("$SCRIPT_DIR/vendor-inquiry-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -e "$CLIENT_SECRET" -t "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$INQUIRY_RESPONSE" | jq .
echo

echo "=================================================================="
echo "Step 4/5: POST /v1.0/transfer-va/payment"
echo "=================================================================="
PAYMENT_RESPONSE="$("$SCRIPT_DIR/vendor-payment-va.sh" -s "$PARTNER_SERVICE_ID" -c "$CUSTOMER_NO" -v "$VA_NO" -a "$AMOUNT" -e "$CLIENT_SECRET" -t "$ACCESS_TOKEN" -u "$BASE_URL")"
echo "$PAYMENT_RESPONSE" | jq .

PAYMENT_CODE="$(echo "$PAYMENT_RESPONSE" | jq -r '.responseCode // empty')"
if [[ "$PAYMENT_CODE" != 2* ]]; then
	echo "!! payment did not return a success responseCode (got: ${PAYMENT_CODE:-<none>}) — aborting before callback check." >&2
	exit 1
fi
echo

echo "=================================================================="
echo "Step 5/5: Merchant payment callback"
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
echo "Done: token issued -> VA ${VA_NO} created -> inquiry confirmed -> paid -> callback checked."
echo "=================================================================="
