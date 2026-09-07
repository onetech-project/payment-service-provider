#!/usr/bin/env bash
#
# Outbound to BCA: POST /openapi/v2.0/transfer-va/status  (VA Payment Status V2)
#
# Two calls, in order:
#   1. POST /openapi/v1.0/access-token/b2b  — asymmetric SHA256withRSA signature
#   2. POST /openapi/v2.0/transfer-va/status — symmetric HMAC-SHA512 signature
#
# Specs:
#   - "BCA API - OAuth & Signature OpenAPI - v1.1.pdf"
#       asymmetric: X-SIGNATURE = Base64(SHA256withRSA(privateKey, clientId|timestamp))
#       symmetric : X-SIGNATURE = Base64(HMAC-SHA512(clientSecret, stringToSign))
#       stringToSign = METHOD:RelativeUrl:AccessToken:Lowercase(HexEncode(SHA256(MinifyJson(body)))):Timestamp
#   - "Technical Documentation OpenAPI VA-Payment-Status API V2 v1.0.pdf"
#       endpoint /openapi/v2.0/transfer-va/status (v2.0 — inquiry/payment stay v1.0)
#       headers CHANNEL-ID (95231), X-PARTNER-ID, X-EXTERNAL-ID (numeric, unique same day)
#       body partnerServiceId(8, space-padded left) / customerNo(<=18) /
#            virtualAccountNo(<=26) / inquiryRequestId(30) / additionalInfo
#
# Usage:
#   ./scripts/bca-payment-status.sh -c <customerNo> -r <inquiryRequestId> \
#       (-f <env-file> | -I <clientId> -S <clientSecret> -k <private_key.pem>) \
#       [-s <partnerServiceId>] [-v <virtualAccountNo>] [-p <X-PARTNER-ID>] \
#       [-i <CHANNEL-ID>] [-u <base-url>] [-t <accessToken>] [-x <X-EXTERNAL-ID>] [-n]
#
# Examples:
#   ./scripts/bca-payment-status.sh -f config/.env.bca.va -s 12345 \
#       -c 123456789012345678 -r 202202111031031234500001136962
#
#   # dry run — print stringToSign/signatures, send nothing
#   ./scripts/bca-payment-status.sh -f config/.env.bca.va -s 12345 \
#       -c 000000123 -r 202202111031031234500001136962 -n
#
# -f reads VENDOR_CLIENT_ID / VENDOR_CLIENT_SECRET / VENDOR_PRIVATE_KEY_PATH /
# VENDOR_BASE_URL / VENDOR_CHANNEL_ID / VENDOR_PARTNER_ID / VENDOR_ENDPOINT_STATUS
# from a .env.<vendor>.<channel> file, so the secret never appears in shell
# history or `ps aux`. Explicit flags win over the env file.
#
# Requires: curl, openssl, jq
set -euo pipefail

BASE_URL=""
ENDPOINT=""
TOKEN_ENDPOINT="/openapi/v1.0/access-token/b2b"
CLIENT_ID=""
CLIENT_SECRET=""
PRIVATE_KEY_PATH=""
PARTNER_SERVICE_ID=""
CUSTOMER_NO=""
VA_NO=""
INQUIRY_REQUEST_ID=""
CHANNEL_ID=""
PARTNER_ID=""
EXTERNAL_ID=""
ACCESS_TOKEN=""
ENV_FILE=""
DRY_RUN=0

# BCA's spec is Lowercase(HexEncode(SHA-256(...))). It is overridable only
# because this repo also talks to a vendor onboarded under feature
# 012-base64-hash-encoding; against real BCA, leave it on hex or every call
# comes back 401 "Unauthorized. [Signature]".
BODY_HASH_ENCODING="${BODY_HASH_ENCODING:-hex}"

# BCA UAT/prod both expect Jakarta local time in X-TIMESTAMP. Pinning TZ here
# keeps the signature valid when the script runs on a UTC host — a timestamp
# outside BCA's tolerance answers 4007301 "Invalid field format [X-TIMESTAMP]".
export TZ="${TZ:-Asia/Jakarta}"

usage() {
	sed -n '3,40p' "$0" >&2
	exit 1
}

# read_env_var extracts KEY=value from a .env.<vendor>.<channel> file,
# stripping surrounding quotes the same way vendor_config.go's parseEnvFile does.
read_env_var() {
	local file="$1" key="$2" line value
	line="$(grep -E "^${key}=" "$file" | tail -n1 || true)"
	[[ -n "$line" ]] || return 1
	value="${line#*=}"
	if [[ "$value" == \"*\" && "$value" == *\" ]]; then
		value="${value:1:${#value}-2}"
	elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
		value="${value:1:${#value}-2}"
	fi
	printf '%s' "$value"
}

while getopts "f:I:S:k:s:c:v:r:p:i:u:t:x:nh" opt; do
	case "$opt" in
	f) ENV_FILE="$OPTARG" ;;
	I) CLIENT_ID="$OPTARG" ;;
	S) CLIENT_SECRET="$OPTARG" ;;
	k) PRIVATE_KEY_PATH="$OPTARG" ;;
	s) PARTNER_SERVICE_ID="$OPTARG" ;;
	c) CUSTOMER_NO="$OPTARG" ;;
	v) VA_NO="$OPTARG" ;;
	r) INQUIRY_REQUEST_ID="$OPTARG" ;;
	p) PARTNER_ID="$OPTARG" ;;
	i) CHANNEL_ID="$OPTARG" ;;
	u) BASE_URL="$OPTARG" ;;
	t) ACCESS_TOKEN="$OPTARG" ;;
	x) EXTERNAL_ID="$OPTARG" ;;
	n) DRY_RUN=1 ;;
	h | *) usage ;;
	esac
done

if [[ -n "$ENV_FILE" ]]; then
	[[ -f "$ENV_FILE" ]] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
	[[ -z "$CLIENT_ID" ]] && CLIENT_ID="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_ID || true)"
	[[ -z "$CLIENT_SECRET" ]] && CLIENT_SECRET="$(read_env_var "$ENV_FILE" VENDOR_CLIENT_SECRET || true)"
	[[ -z "$PRIVATE_KEY_PATH" ]] && PRIVATE_KEY_PATH="$(read_env_var "$ENV_FILE" VENDOR_PRIVATE_KEY_PATH || true)"
	[[ -z "$BASE_URL" ]] && BASE_URL="$(read_env_var "$ENV_FILE" VENDOR_BASE_URL || true)"
	[[ -z "$CHANNEL_ID" ]] && CHANNEL_ID="$(read_env_var "$ENV_FILE" VENDOR_CHANNEL_ID || true)"
	[[ -z "$ENDPOINT" ]] && ENDPOINT="$(read_env_var "$ENV_FILE" VENDOR_ENDPOINT_STATUS || true)"
	TOKEN_ENDPOINT_FROM_ENV="$(read_env_var "$ENV_FILE" VENDOR_TOKEN_ENDPOINT || true)"
	[[ -n "$TOKEN_ENDPOINT_FROM_ENV" ]] && TOKEN_ENDPOINT="$TOKEN_ENDPOINT_FROM_ENV"
	# VENDOR_PARTNER_ID in this repo's env files is the INBOUND partner id BCA
	# sends us (e.g. "1-MANJO-SNAP"). Outbound, BCA wants its own String(5)
	# Company Code VA, which is partnerServiceId — so the env value is only a
	# fallback, applied after the partnerServiceId default below.
	ENV_PARTNER_ID="$(read_env_var "$ENV_FILE" VENDOR_PARTNER_ID || true)"
fi

BASE_URL="${BASE_URL:-https://devapi.klikbca.com}"
ENDPOINT="${ENDPOINT:-/openapi/v2.0/transfer-va/status}"
CHANNEL_ID="${CHANNEL_ID:-95231}"

[[ -z "$CUSTOMER_NO" || -z "$INQUIRY_REQUEST_ID" ]] && usage
[[ -z "$CLIENT_ID" || -z "$CLIENT_SECRET" ]] && {
	echo "!! clientId/clientSecret missing — pass -I/-S or -f <env-file>." >&2
	exit 1
}
if [[ -z "$ACCESS_TOKEN" ]]; then
	[[ -n "$PRIVATE_KEY_PATH" ]] || { echo "!! private key missing — pass -k, or -t <accessToken> to skip the token call." >&2; exit 1; }
	[[ -f "$PRIVATE_KEY_PATH" ]] || { echo "!! private key not found: $PRIVATE_KEY_PATH" >&2; exit 1; }
fi
command -v jq >/dev/null || { echo "!! jq is required (MinifyJson step)." >&2; exit 1; }

# partnerServiceId is String(8) Fixed, "space padding on the left if it doesn't
# reach 8 characters" — the padding is part of the signed body, so it must be
# applied before hashing, not by BCA afterwards.
if [[ -z "$PARTNER_SERVICE_ID" ]]; then
	echo "!! -s <partnerServiceId> is required (BCA Company Code VA)." >&2
	exit 1
fi
PARTNER_SERVICE_ID_TRIMMED="${PARTNER_SERVICE_ID#"${PARTNER_SERVICE_ID%%[![:space:]]*}"}"
PARTNER_SERVICE_ID_PADDED="$(printf '%8s' "$PARTNER_SERVICE_ID_TRIMMED")"

# virtualAccountNo = partnerServiceId (8, padded) + customerNo, per the spec's
# own derivation. Overridable with -v for a VA that does not follow it.
VA_NO="${VA_NO:-${PARTNER_SERVICE_ID_PADDED}${CUSTOMER_NO}}"

# X-PARTNER-ID outbound is BCA's String(5) Company Code VA == partnerServiceId.
PARTNER_ID="${PARTNER_ID:-${PARTNER_SERVICE_ID_TRIMMED:-${ENV_PARTNER_ID:-}}}"

# X-EXTERNAL-ID: "Numeric String reference number that should be unique in the
# same day", max 36. date alone is only second-resolution and two calls in the
# same second would collide onto 4092600 Conflict, so a random tail is appended.
EXTERNAL_ID="${EXTERNAL_ID:-$(date +%Y%m%d%H%M%S)$(printf '%06d' $((RANDOM % 1000000)))}"

# ---------------------------------------------------------------------------
# Step 1 — access token (asymmetric SHA256withRSA)
# ---------------------------------------------------------------------------
if [[ -z "$ACCESS_TOKEN" ]]; then
	TOKEN_TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
	TOKEN_STRING_TO_SIGN="${CLIENT_ID}|${TOKEN_TIMESTAMP}"
	TOKEN_SIGNATURE="$(printf '%s' "$TOKEN_STRING_TO_SIGN" | openssl dgst -sha256 -sign "$PRIVATE_KEY_PATH" | openssl base64 -A)"
	TOKEN_BODY='{"grantType":"client_credentials"}'

	echo "==> [1/2] POST ${BASE_URL}${TOKEN_ENDPOINT}" >&2
	echo "==> X-CLIENT-KEY:  ${CLIENT_ID}" >&2
	echo "==> X-TIMESTAMP:   ${TOKEN_TIMESTAMP}" >&2
	echo "==> stringToSign:  ${TOKEN_STRING_TO_SIGN}" >&2
	echo "==> X-SIGNATURE:   ${TOKEN_SIGNATURE}" >&2
	echo "==> body:          ${TOKEN_BODY}" >&2
	echo "==> curl:" >&2
	cat <<CURLEOF >&2
curl -sS -X POST '${BASE_URL}${TOKEN_ENDPOINT}' \\
  -H 'Content-Type: application/json' \\
  -H 'X-CLIENT-KEY: ${CLIENT_ID}' \\
  -H 'X-TIMESTAMP: ${TOKEN_TIMESTAMP}' \\
  -H 'X-SIGNATURE: ${TOKEN_SIGNATURE}' \\
  -d '${TOKEN_BODY}'
CURLEOF

	if [[ "$DRY_RUN" -eq 1 ]]; then
		echo "==> (dry run) token call skipped; using placeholder accessToken" >&2
		ACCESS_TOKEN="DRYRUN-ACCESS-TOKEN"
	else
		TOKEN_RESPONSE="$(curl -sS -X POST "${BASE_URL}${TOKEN_ENDPOINT}" \
			-H "Content-Type: application/json" \
			-H "X-CLIENT-KEY: ${CLIENT_ID}" \
			-H "X-TIMESTAMP: ${TOKEN_TIMESTAMP}" \
			-H "X-SIGNATURE: ${TOKEN_SIGNATURE}" \
			-d "${TOKEN_BODY}")"
		echo "==> token response:" >&2
		echo "$TOKEN_RESPONSE" | jq . >&2 2>/dev/null || echo "$TOKEN_RESPONSE" >&2
		ACCESS_TOKEN="$(echo "$TOKEN_RESPONSE" | jq -r '.accessToken // empty' 2>/dev/null || true)"
		[[ -n "$ACCESS_TOKEN" ]] || { echo "!! no accessToken in response — aborting." >&2; exit 1; }
	fi
	echo >&2
fi

# ---------------------------------------------------------------------------
# Step 2 — VA payment status (symmetric HMAC-SHA512)
# ---------------------------------------------------------------------------
TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"

# Built compact with jq so the bytes that are hashed are byte-for-byte the
# bytes that are sent — the MinifyJson step is load-bearing: hashing a
# pretty-printed body yields a different digest and every call returns 401.
BODY="$(jq -cn \
	--arg partnerServiceId "$PARTNER_SERVICE_ID_PADDED" \
	--arg customerNo "$CUSTOMER_NO" \
	--arg virtualAccountNo "$VA_NO" \
	--arg inquiryRequestId "$INQUIRY_REQUEST_ID" \
	'{partnerServiceId: $partnerServiceId,
	  customerNo: $customerNo,
	  virtualAccountNo: $virtualAccountNo,
	  inquiryRequestId: $inquiryRequestId,
	  additionalInfo: {}}')"

case "$BODY_HASH_ENCODING" in
hex) BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -binary | xxd -p -c 256)" ;;
base64) BODY_HASH="$(printf '%s' "$BODY" | openssl dgst -sha256 -binary | openssl base64 -A)" ;;
*) echo "!! unknown BODY_HASH_ENCODING: $BODY_HASH_ENCODING (want hex|base64)" >&2; exit 1 ;;
esac

STRING_TO_SIGN="POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}"
SIGNATURE="$(printf '%s' "$STRING_TO_SIGN" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A)"

# Diagnostics go to stderr so stdout stays clean JSON — this lets the script be
# chained/captured by other scripts (see e2e-va-flow.sh).
echo "==> [2/2] POST ${BASE_URL}${ENDPOINT}" >&2
echo "==> Authorization: Bearer ${ACCESS_TOKEN}" >&2
echo "==> X-TIMESTAMP:   ${TIMESTAMP}" >&2
echo "==> CHANNEL-ID:    ${CHANNEL_ID}" >&2
echo "==> X-PARTNER-ID:  ${PARTNER_ID}" >&2
echo "==> X-EXTERNAL-ID: ${EXTERNAL_ID}" >&2
echo "==> bodyHash(${BODY_HASH_ENCODING}): ${BODY_HASH}" >&2
echo "==> stringToSign:  ${STRING_TO_SIGN}" >&2
echo "==> X-SIGNATURE:   ${SIGNATURE}" >&2
echo "==> Request body:" >&2
echo "$BODY" | jq . >&2
echo "==> curl:" >&2
cat <<CURLEOF >&2
curl -sS -X POST '${BASE_URL}${ENDPOINT}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${ACCESS_TOKEN}' \\
  -H 'X-TIMESTAMP: ${TIMESTAMP}' \\
  -H 'X-SIGNATURE: ${SIGNATURE}' \\
  -H 'CHANNEL-ID: ${CHANNEL_ID}' \\
  -H 'X-PARTNER-ID: ${PARTNER_ID}' \\
  -H 'X-EXTERNAL-ID: ${EXTERNAL_ID}' \\
  -d '${BODY}'
CURLEOF
echo >&2

if [[ "$DRY_RUN" -eq 1 ]]; then
	echo "==> (dry run) status call skipped" >&2
	exit 0
fi

curl -sS -X POST "${BASE_URL}${ENDPOINT}" \
	-H "Content-Type: application/json" \
	-H "Authorization: Bearer ${ACCESS_TOKEN}" \
	-H "X-TIMESTAMP: ${TIMESTAMP}" \
	-H "X-SIGNATURE: ${SIGNATURE}" \
	-H "CHANNEL-ID: ${CHANNEL_ID}" \
	-H "X-PARTNER-ID: ${PARTNER_ID}" \
	-H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
	-d "${BODY}" \
	| (command -v jq >/dev/null && jq . || cat)
