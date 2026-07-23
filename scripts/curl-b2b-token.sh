#!/usr/bin/env bash
#
# Generates X-TIMESTAMP + X-SIGNATURE and calls POST /v1.0/access-token/b2b.
#
# Usage:
#   ./scripts/curl-b2b-token.sh -i <client_id> -p <private_key.pem> [-u <base-url>]
#
# Requires: openssl, curl
set -euo pipefail

BASE_URL="http://localhost:8080"
PRIVATE_KEY_PATH=""
CLIENT_ID=""

usage() {
	echo "Usage: $0 -i <client_id> -p <private_key.pem> [-u <base-url>]" >&2
	exit 1
}

while getopts "i:p:u:h" opt; do
	case "$opt" in
	i) CLIENT_ID="$OPTARG" ;;
	p) PRIVATE_KEY_PATH="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	h | *) usage ;;
	esac
done

[[ -z "$CLIENT_ID" || -z "$PRIVATE_KEY_PATH" ]] && usage
[[ -f "$PRIVATE_KEY_PATH" ]] || {
	echo "Private key not found: $PRIVATE_KEY_PATH" >&2
	exit 1
}

# SNAP timestamp format: yyyy-MM-ddTHH:mm:ss±HH:00
TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"

# stringToSign = X-CLIENT-KEY|X-TIMESTAMP, signed SHA256withRSA, base64-encoded.
# Must match the X-CLIENT-KEY header byte-for-byte, or the server-side RSA
# verification (against clientID's registered public key) will fail.
STRING_TO_SIGN="${CLIENT_ID}|${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha256 -sign "$PRIVATE_KEY_PATH" | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script
# be chained/captured by other scripts (see e2e-va-flow.sh).
echo "==> X-TIMESTAMP: $TIMESTAMP" >&2
echo "==> stringToSign: $STRING_TO_SIGN" >&2
echo "==> X-SIGNATURE: $SIGNATURE" >&2
echo "==> X-CLIENT-KEY: $CLIENT_ID" >&2
echo >&2

curl -sS -X POST "${BASE_URL}/v1.0/access-token/b2b" \
	-H "Content-Type: application/json" \
	-H "X-CLIENT-KEY: ${CLIENT_ID}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "Idempotency-Key: $(uuidgen)" \
	-d '{"grantType":"client_credentials","additionalInfo":{}}' \
	| (command -v jq >/dev/null && jq . || cat)
