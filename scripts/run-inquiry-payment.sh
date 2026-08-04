#!/usr/bin/env bash
# Run inquiry + payment against uatbca.manjo.co.id and save request/response to txt files
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

BASE_URL="https://uatbca.manjo.co.id"
CLIENT_ID="a9612651-9181-4461-af20-4ef6598a93f5"
CLIENT_SECRET="1c19e3313badc5068995ca0b42a4938e96ae57ca1c57ad13cfeed728a9ceb15a"
PRIVATE_KEY="$PROJECT_DIR/client_private.pem"
VA_NO="1597366666666"
TRX_ID="cmscyczsl00061wryys7zli68"
PARTNER_SERVICE_ID="   12345"
CUSTOMER_NO="66666666"
PARTNER_ID="1-MANJO-SNAP"
CHANNEL_ID="95231"
AMOUNT="150000.00"

# ---- Step 1: Get access token ----
echo ">>> Getting access token..."
TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
TOKEN_SIG="$(printf '%s' "${CLIENT_ID}|${TIMESTAMP}" | openssl dgst -sha256 -sign "$PRIVATE_KEY" | openssl base64 -A)"
TOKEN_BODY='{"grantType":"client_credentials","additionalInfo":{}}'

TOKEN_RESP=$(curl -sS -X POST "${BASE_URL}/openapi/v1.0/access-token/b2b" \
  -H "Content-Type: application/json" \
  -H "X-CLIENT-KEY: ${CLIENT_ID}" \
  -H "X-TIMESTAMP: ${TIMESTAMP}" \
  -H "X-SIGNATURE: ${TOKEN_SIG}" \
  -d "${TOKEN_BODY}")

ACCESS_TOKEN=$(echo "$TOKEN_RESP" | jq -r '.accessToken // empty')
if [[ -z "$ACCESS_TOKEN" ]]; then
  echo "!! Failed to get access token. Response:"
  echo "$TOKEN_RESP" | jq .
  exit 1
fi
echo ">>> Token obtained (expires in 15min)"

# Helper: compute HMAC-SHA512 symmetric signature
sign_request() {
  local method="$1" endpoint="$2" body="$3" ts="$4"
  local body_hash
  body_hash="$(printf '%s' "$body" | openssl dgst -sha256 -binary | openssl base64 -A)"
  local sts="${method}:${endpoint}:${ACCESS_TOKEN}:${body_hash}:${ts}"
  printf '%s' "$sts" | openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A
}

# ================================================================
# INQUIRY
# ================================================================
echo ""
echo "========================================"
echo "  INQUIRY"
echo "========================================"

TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
EXTERNAL_ID="$(date +%s)$((RANDOM % 9000 + 1000))"
INQUIRY_REQ_ID="INQ-$(date +%s)$((RANDOM % 9000 + 1000))"
INQUIRY_EP="/openapi/v1.0/transfer-va/inquiry"

INQUIRY_BODY=$(jq -cn \
  --arg p "$PARTNER_SERVICE_ID" \
  --arg c "$CUSTOMER_NO" \
  --arg v "$VA_NO" \
  --arg d "$TIMESTAMP" \
  --arg a "$AMOUNT" \
  --arg r "$INQUIRY_REQ_ID" \
  '{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,txnDateInit:$d,amount:{value:$a,currency:"IDR"},inquiryRequestId:$r}')

INQUIRY_SIG=$(sign_request "POST" "$INQUIRY_EP" "$INQUIRY_BODY" "$TIMESTAMP")

INQUIRY_CURL="curl -sS -X POST '${BASE_URL}${INQUIRY_EP}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${ACCESS_TOKEN}' \\
  -H 'X-TIMESTAMP: ${TIMESTAMP}' \\
  -H 'X-SIGNATURE: ${INQUIRY_SIG}' \\
  -H 'X-PARTNER-ID: ${PARTNER_ID}' \\
  -H 'X-EXTERNAL-ID: ${EXTERNAL_ID}' \\
  -H 'CHANNEL-ID: ${CHANNEL_ID}' \\
  -d '${INQUIRY_BODY}'"

echo ">>> Sending inquiry..."
INQUIRY_RESP=$(curl -sS -X POST "${BASE_URL}${INQUIRY_EP}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "X-TIMESTAMP: ${TIMESTAMP}" \
  -H "X-SIGNATURE: ${INQUIRY_SIG}" \
  -H "X-PARTNER-ID: ${PARTNER_ID}" \
  -H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
  -H "CHANNEL-ID: ${CHANNEL_ID}" \
  -d "${INQUIRY_BODY}")

echo ">>> Response:"
echo "$INQUIRY_RESP" | jq .

# Save inquiry to file
cat > "$PROJECT_DIR/inquiry_result.txt" <<EOF
============================================================
INQUIRY REQUEST
============================================================
Endpoint: POST ${BASE_URL}${INQUIRY_EP}
Timestamp: ${TIMESTAMP}
InquiryRequestId: ${INQUIRY_REQ_ID}
VirtualAccountNo: ${VA_NO}

--- HEADERS ---
Content-Type: application/json
Authorization: Bearer ${ACCESS_TOKEN}
X-TIMESTAMP: ${TIMESTAMP}
X-SIGNATURE: ${INQUIRY_SIG}
X-PARTNER-ID: ${PARTNER_ID}
X-EXTERNAL-ID: ${EXTERNAL_ID}
CHANNEL-ID: ${CHANNEL_ID}

--- REQUEST BODY ---
$(echo "$INQUIRY_BODY" | jq .)

--- CURL COMMAND ---
${INQUIRY_CURL}

============================================================
INQUIRY RESPONSE
============================================================
$(echo "$INQUIRY_RESP" | jq .)
EOF

echo ">>> Saved to inquiry_result.txt"

# ================================================================
# PAYMENT
# ================================================================
echo ""
echo "========================================"
echo "  PAYMENT"
echo "========================================"

# Need fresh timestamp for payment
sleep 1
TIMESTAMP="$(date +%Y-%m-%dT%H:%M:%S%:z)"
EXTERNAL_ID="$(date +%s)$((RANDOM % 9000 + 1000))"
PAYMENT_REQ_ID="PAY-$(date +%s)$((RANDOM % 9000 + 1000))"
PAYMENT_EP="/openapi/v1.0/transfer-va/payment"
REF_NO="R$(date +%s | tail -c 10)"

PAYMENT_BODY=$(jq -cn \
  --arg p "$PARTNER_SERVICE_ID" \
  --arg c "$CUSTOMER_NO" \
  --arg v "$VA_NO" \
  --arg t "$TRX_ID" \
  --arg q "$PAYMENT_REQ_ID" \
  --arg a "$AMOUNT" \
  --arg d "$TIMESTAMP" \
  --arg n "$REF_NO" \
  '{partnerServiceId:$p,customerNo:$c,virtualAccountNo:$v,trxId:$t,paymentRequestId:$q,paidAmount:{value:$a,currency:"IDR"},totalAmount:{value:$a,currency:"IDR"},trxDateTime:$d,referenceNo:$n}')

PAYMENT_SIG=$(sign_request "POST" "$PAYMENT_EP" "$PAYMENT_BODY" "$TIMESTAMP")

PAYMENT_CURL="curl -sS -X POST '${BASE_URL}${PAYMENT_EP}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${ACCESS_TOKEN}' \\
  -H 'X-TIMESTAMP: ${TIMESTAMP}' \\
  -H 'X-SIGNATURE: ${PAYMENT_SIG}' \\
  -H 'X-PARTNER-ID: ${PARTNER_ID}' \\
  -H 'X-EXTERNAL-ID: ${EXTERNAL_ID}' \\
  -H 'CHANNEL-ID: ${CHANNEL_ID}' \\
  -d '${PAYMENT_BODY}'"

echo ">>> Sending payment..."
PAYMENT_RESP=$(curl -sS -X POST "${BASE_URL}${PAYMENT_EP}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "X-TIMESTAMP: ${TIMESTAMP}" \
  -H "X-SIGNATURE: ${PAYMENT_SIG}" \
  -H "X-PARTNER-ID: ${PARTNER_ID}" \
  -H "X-EXTERNAL-ID: ${EXTERNAL_ID}" \
  -H "CHANNEL-ID: ${CHANNEL_ID}" \
  -d "${PAYMENT_BODY}")

echo ">>> Response:"
echo "$PAYMENT_RESP" | jq .

# Save payment to file
cat > "$PROJECT_DIR/payment_result.txt" <<EOF
============================================================
PAYMENT REQUEST
============================================================
Endpoint: POST ${BASE_URL}${PAYMENT_EP}
Timestamp: ${TIMESTAMP}
PaymentRequestId: ${PAYMENT_REQ_ID}
TrxId: ${TRX_ID}
VirtualAccountNo: ${VA_NO}

--- HEADERS ---
Content-Type: application/json
Authorization: Bearer ${ACCESS_TOKEN}
X-TIMESTAMP: ${TIMESTAMP}
X-SIGNATURE: ${PAYMENT_SIG}
X-PARTNER-ID: ${PARTNER_ID}
X-EXTERNAL-ID: ${EXTERNAL_ID}
CHANNEL-ID: ${CHANNEL_ID}

--- REQUEST BODY ---
$(echo "$PAYMENT_BODY" | jq .)

--- CURL COMMAND ---
${PAYMENT_CURL}

============================================================
PAYMENT RESPONSE
============================================================
$(echo "$PAYMENT_RESP" | jq .)
EOF

echo ">>> Saved to payment_result.txt"
echo ""
echo "Done! Check inquiry_result.txt and payment_result.txt"
