package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Negative cases. Two things are asserted throughout, because BCA checks both:
// the responseCode must carry the service code of the endpoint called, and a
// 400/404/409 must carry virtualAccountData with a status and reason.

// --- authentication -----------------------------------------------------

func TestE2E_Negative_Auth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		opts     []func(*requestOptions)
		wantHTTP int
		wantCode string
	}{
		{
			name:     "signature signed with the wrong secret",
			path:     inquiryPath,
			opts:     []func(*requestOptions){withSecret("wrong-secret")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012400",
		},
		{
			name:     "garbage signature",
			path:     paymentPath,
			opts:     []func(*requestOptions){withSignature("not-a-signature")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012500",
		},
		{
			name:     "empty signature is a missing mandatory header",
			path:     paymentPath,
			opts:     []func(*requestOptions){withSignature(" ")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012500",
		},
		{
			name:     "unknown CHANNEL-ID",
			path:     inquiryPath,
			opts:     []func(*requestOptions){withChannelID("00000")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012400",
		},
		{
			name:     "unknown X-PARTNER-ID",
			path:     paymentPath,
			opts:     []func(*requestOptions){withPartnerID("99999")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012500",
		},
		{
			name:     "missing CHANNEL-ID",
			path:     statusPath,
			opts:     []func(*requestOptions){withChannelID("")},
			wantHTTP: http.StatusBadRequest,
			wantCode: "4002602",
		},
		{
			name:     "missing X-PARTNER-ID",
			path:     inquiryPath,
			opts:     []func(*requestOptions){withPartnerID("")},
			wantHTTP: http.StatusBadRequest,
			wantCode: "4002402",
		},
		// BCA OpenAPI OAuth & Signature v1.1 lists Authorization as Mandatory
		// on every API request, and Appendix A of both service docs answers a
		// missing or unusable bearer with 401xx01 "Invalid Token (B2B)" —
		// distinct from 401xx00, which is the signature's own failure.
		{
			name:     "missing Authorization on a ClientID-onboarded vendor",
			path:     inquiryPath,
			opts:     []func(*requestOptions){withoutAccessToken()},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012401",
		},
		{
			name:     "accessToken this issuer never minted",
			path:     paymentPath,
			opts:     []func(*requestOptions){withAccessToken("forged-token")},
			wantHTTP: http.StatusUnauthorized,
			wantCode: "4012501",
		},
		{
			name:     "X-TIMESTAMP not parseable",
			path:     inquiryPath,
			opts:     []func(*requestOptions){withTimestamp("2026-08-06 10:00:00")},
			wantHTTP: http.StatusBadRequest,
			wantCode: "4002401",
		},
		{
			name:     "X-TIMESTAMP without a timezone designator",
			path:     paymentPath,
			opts:     []func(*requestOptions){withTimestamp("2026-08-06T10:00:00")},
			wantHTTP: http.StatusBadRequest,
			wantCode: "4002501",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer(t)
			payload := inquiryPayload(testPartnerServiceID, "678901234567890200", "INQ-NEG-1")
			if tc.path == paymentPath {
				payload = paymentPayload(testPartnerServiceID, "678901234567890200", "PAY-NEG-1", "1000.00")
			} else if tc.path == statusPath {
				payload = statusPayload(testPartnerServiceID, "678901234567890200", "INQ-NEG-1")
			}

			resp := s.call(t, tc.path, payload, tc.opts...)

			assert.Equal(t, tc.wantHTTP, resp.status, resp.raw)
			assert.Equal(t, tc.wantCode, resp.code(), resp.raw)
		})
	}
}

func TestE2E_Negative_WrongBodyHashEncodingRejected(t *testing.T) {
	// The vendor is configured for hex (the BCA/SNAP form). A signature
	// computed over a base64 body hash must not verify — and vice versa, which
	// is why the encoding is pinned per vendor rather than guessed.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890201", "1000.00")

	resp := s.call(t, paymentPath,
		paymentPayload(testPartnerServiceID, "678901234567890201", "PAY-ENC-1", "1000.00"),
		withBodyEncoding(crypto.BodyHashBase64))

	assert.Equal(t, http.StatusUnauthorized, resp.status, resp.raw)
	assert.Equal(t, "4012500", resp.code())
}

func TestE2E_SignatureCoversTheBody(t *testing.T) {
	// Tamper with the body after signing: the signature must stop verifying.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890202", "1000.00")

	original := mustJSON(t, paymentPayload(testPartnerServiceID, "678901234567890202", "PAY-TAMPER-1", "1000.00"))
	tampered := strings.Replace(original, `"1000.00"`, `"9999.00"`, 1)
	require.NotEqual(t, original, tampered)

	// Sign the ORIGINAL body, send the tampered one. The real accessToken goes
	// into stringToSign and onto the request: this vendor is ClientID-onboarded,
	// so signing with an empty AccessToken component would fail the signature
	// check on the token rather than on the body, and the test would pass
	// without proving anything about the body.
	token := s.tokenFor(t, testClientID)
	timestamp := time.Now().Format(time.RFC3339)
	bodyHash := crypto.HashRequestBody([]byte(original), crypto.BodyHashHex)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, paymentPath, token, bodyHash, timestamp)
	signature := crypto.NewHMACSigner(testSecret, "HMAC-SHA512").Sign(stringToSign)

	resp := s.call(t, paymentPath, nil,
		withRawBody(tampered), withTimestamp(timestamp), withSignature(signature),
		withAccessToken(token))

	assert.Equal(t, http.StatusUnauthorized, resp.status, resp.raw)
	assert.Equal(t, "4012500", resp.code())
}

func TestE2E_PrettyPrintedBodyStillVerifies(t *testing.T) {
	// BCA minifies the body before hashing, so an indented body whose
	// signature was computed over the minified form must verify.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890203", "1000.00")

	pretty := "{\n  \"partnerServiceId\": \"" + testPartnerServiceID + "\",\n" +
		"  \"customerNo\": \"678901234567890203\",\n" +
		"  \"virtualAccountNo\": \"" + testPartnerServiceID + "678901234567890203\",\n" +
		"  \"virtualAccountName\": \"Budi Manjo\",\n" +
		"  \"paymentRequestId\": \"PAY-PRETTY-1\",\n" +
		"  \"channelCode\": 6011,\n" +
		"  \"paidAmount\": { \"value\": \"1000.00\", \"currency\": \"IDR\" },\n" +
		"  \"totalAmount\": { \"value\": \"1000.00\", \"currency\": \"IDR\" },\n" +
		"  \"trxDateTime\": \"" + time.Now().Format(time.RFC3339) + "\",\n" +
		"  \"flagAdvise\": \"N\"\n}"

	resp := s.call(t, paymentPath, nil, withRawBody(pretty))

	require.Equal(t, http.StatusOK, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentSuccess, resp.code())
}

// --- X-EXTERNAL-ID ------------------------------------------------------

func TestE2E_Negative_ExternalID(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		s := newServer(t)
		resp := s.call(t, paymentPath,
			paymentPayload(testPartnerServiceID, "678901234567890210", "PAY-EXT-1", "1000.00"),
			withoutExternalID())

		assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
		assert.Equal(t, "4002502", resp.code())
		assert.Contains(t, resp.raw, "X-EXTERNAL-ID")
	})

	t.Run("longer than 36 characters", func(t *testing.T) {
		s := newServer(t)
		resp := s.call(t, inquiryPath,
			inquiryPayload(testPartnerServiceID, "678901234567890211", "INQ-EXT-1"),
			withExternalID(strings.Repeat("9", 37)))

		assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
		assert.Equal(t, "4002401", resp.code())
	})

	t.Run("reused with a different payload is a Conflict, not a 422", func(t *testing.T) {
		s := newServer(t)
		seedNoBillAccount(s, testPartnerServiceID, "678901234567890212")

		first := s.call(t, paymentPath,
			paymentPayload(testPartnerServiceID, "678901234567890212", "PAY-EXT-A", "1000.00"),
			withExternalID("900000000000001"))
		require.Equal(t, http.StatusOK, first.status, first.raw)

		second := s.call(t, paymentPath,
			paymentPayload(testPartnerServiceID, "678901234567890212", "PAY-EXT-B", "2000.00"),
			withExternalID("900000000000001"))

		// SNAP has no 422; BCA documents this as 4092500 Conflict.
		assert.Equal(t, http.StatusConflict, second.status, second.raw)
		assert.Equal(t, domain.CodePaymentConflict, second.code())
		assert.Equal(t, "Conflict", second.body["responseMessage"])
		assert.Equal(t, domain.PaymentFlagReject, second.vaData(t)["paymentFlagStatus"])
	})
}

// --- request body -------------------------------------------------------

func TestE2E_Negative_MalformedBody(t *testing.T) {
	for path, wantCode := range map[string]string{
		inquiryPath: domain.CodeInquiryBadRequest,
		paymentPath: domain.CodePaymentBadRequest,
		statusPath:  domain.CodeStatusBadRequest,
	} {
		t.Run(path, func(t *testing.T) {
			s := newServer(t)

			resp := s.call(t, path, nil, withRawBody(`{"partnerServiceId":`))

			assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
			// A body that could not be parsed is BCA's "Request Parsing
			// Error" → Bad Request, distinct from a parsed body with a bad
			// field.
			assert.Equal(t, wantCode, resp.code(), resp.raw)
			assert.Equal(t, "Bad Request", resp.body["responseMessage"])
			assert.NotNil(t, resp.vaData(t))
		})
	}
}

func TestE2E_Negative_InquiryMandatoryFields(t *testing.T) {
	for _, field := range []string{"partnerServiceId", "customerNo", "virtualAccountNo", "inquiryRequestId"} {
		t.Run(field, func(t *testing.T) {
			s := newServer(t)
			payload := inquiryPayload(testPartnerServiceID, "678901234567890220", "INQ-MAND-1")
			payload[field] = ""

			resp := s.call(t, inquiryPath, payload)

			assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
			assert.Equal(t, "4002402", resp.code())
			assert.Contains(t, resp.body["responseMessage"], field)
			assert.Equal(t, domain.InquiryStatusFailed, resp.vaData(t)["inquiryStatus"])
		})
	}
}

func TestE2E_Negative_PaymentMandatoryFields(t *testing.T) {
	for _, tc := range []struct {
		field  string
		mutate func(map[string]any)
	}{
		{"paymentRequestId", func(p map[string]any) { p["paymentRequestId"] = "" }},
		{"paidAmount", func(p map[string]any) { delete(p, "paidAmount") }},
		{"virtualAccountName", func(p map[string]any) { p["virtualAccountName"] = "" }},
		{"channelCode", func(p map[string]any) { delete(p, "channelCode") }},
		{"totalAmount", func(p map[string]any) { delete(p, "totalAmount") }},
		{"trxDateTime", func(p map[string]any) { delete(p, "trxDateTime") }},
		{"flagAdvise", func(p map[string]any) { delete(p, "flagAdvise") }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			s := newServer(t)
			payload := paymentPayload(testPartnerServiceID, "678901234567890221", "PAY-MAND-1", "1000.00")
			tc.mutate(payload)

			resp := s.call(t, paymentPath, payload)

			assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
			assert.Equal(t, "4002502", resp.code())
			assert.Contains(t, resp.body["responseMessage"], tc.field)
			assert.Equal(t, domain.PaymentFlagReject, resp.vaData(t)["paymentFlagStatus"])
		})
	}
}

func TestE2E_Payment_TrxIDStaysOptional(t *testing.T) {
	// BCA marks trxId "N". Requiring it rejected valid channel-originated
	// payments with a 400.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890222", "1000.00")

	payload := paymentPayload(testPartnerServiceID, "678901234567890222", "PAY-NOTRX-1", "1000.00")
	delete(payload, "trxId")

	resp := s.call(t, paymentPath, payload)

	require.Equal(t, http.StatusOK, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentSuccess, resp.code())
}

func TestE2E_Negative_PaymentFieldFormats(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{"partnerServiceId over 8", func(p map[string]any) { p["partnerServiceId"] = "123456789" }, "partnerServiceId"},
		{"paymentRequestId over 30", func(p map[string]any) {
			p["paymentRequestId"] = strings.Repeat("9", 31)
		}, "paymentRequestId"},
		{"virtualAccountName over 30", func(p map[string]any) {
			p["virtualAccountName"] = strings.Repeat("N", 31)
		}, "virtualAccountName"},
		{"referenceNo over 11", func(p map[string]any) { p["referenceNo"] = strings.Repeat("9", 12) }, "referenceNo"},
		{"unsupported currency", func(p map[string]any) {
			p["paidAmount"] = map[string]any{"value": "1000.00", "currency": "EUR"}
			p["totalAmount"] = map[string]any{"value": "1000.00", "currency": "EUR"}
		}, "paidAmount.currency"},
		{"currency disagrees between amounts", func(p map[string]any) {
			p["totalAmount"] = map[string]any{"value": "1000.00", "currency": "USD"}
		}, "totalAmount.currency"},
		{"amount over 13 integer digits", func(p map[string]any) {
			p["paidAmount"] = map[string]any{"value": "12345678901234.00", "currency": "IDR"}
			p["totalAmount"] = map[string]any{"value": "12345678901234.00", "currency": "IDR"}
		}, "paidAmount.value"},
		{"amount is not numeric", func(p map[string]any) {
			p["paidAmount"] = map[string]any{"value": "abc", "currency": "IDR"}
			p["totalAmount"] = map[string]any{"value": "abc", "currency": "IDR"}
		}, "paidAmount.value"},
		{"flagAdvise outside N/Y", func(p map[string]any) { p["flagAdvise"] = "X" }, "flagAdvise"},
		{"customerNo is not the VA suffix", func(p map[string]any) {
			p["customerNo"] = "999999999999999999"
		}, "virtualAccountNo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer(t)
			payload := paymentPayload(testPartnerServiceID, "678901234567890223", "PAY-FMT-1", "1000.00")
			tc.mutate(payload)

			resp := s.call(t, paymentPath, payload)

			assert.Equal(t, http.StatusBadRequest, resp.status, resp.raw)
			assert.Equal(t, "4002501", resp.code(), resp.raw)
			assert.Contains(t, resp.body["responseMessage"], tc.field)
		})
	}
}

// --- business outcomes --------------------------------------------------

func TestE2E_Negative_UnregisteredVA(t *testing.T) {
	s := newServer(t) // nothing seeded

	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890230", "INQ-404-1"))
	assert.Equal(t, http.StatusNotFound, inq.status, inq.raw)
	assert.Equal(t, domain.CodeInquiryNotFound, inq.code())
	assert.Equal(t, domain.InquiryStatusFailed, inq.vaData(t)["inquiryStatus"])
	assert.Equal(t, "Virtual Account Not Found", inq.vaData(t)["inquiryReason"].(map[string]any)["english"])

	// The payment side used to record this as an orphan row instead of
	// rejecting it.
	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890230", "PAY-404-1", "1000.00"))
	assert.Equal(t, http.StatusNotFound, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentNotFound, pay.code())
	assert.Equal(t, domain.PaymentFlagReject, pay.vaData(t)["paymentFlagStatus"])
	assert.Equal(t, 0, s.notifier.count())
}

func TestE2E_Negative_ExpiredBill(t *testing.T) {
	s := newServer(t)
	rec := seedFixedBill(s, testPartnerServiceID, "678901234567890231", "1000.00")
	past := time.Now().Add(-time.Hour)
	rec.ExpiredDate = &past

	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890231", "INQ-EXPIRED-1"))
	assert.Equal(t, http.StatusNotFound, inq.status, inq.raw)
	assert.Equal(t, domain.CodeInquiryExpired, inq.code())

	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890231", "PAY-EXPIRED-1", "1000.00"))
	assert.Equal(t, http.StatusNotFound, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentExpired, pay.code())
	assert.Equal(t, domain.PaymentFlagReject, pay.vaData(t)["paymentFlagStatus"])
}

func TestE2E_Negative_StatusTransactionNotFound(t *testing.T) {
	s := newServer(t)

	resp := s.call(t, statusPath, statusPayload(testPartnerServiceID, "678901234567890232", "UNKNOWN-REQ"))

	assert.Equal(t, http.StatusNotFound, resp.status, resp.raw)
	// BCA publishes 4042601 for this. 4042619 is not a code it recognises.
	assert.Equal(t, domain.CodeStatusNotFound, resp.code())
	assert.Equal(t, "Transaction Not Found", resp.body["responseMessage"])
}

// --- response envelope --------------------------------------------------

func TestE2E_EveryRejectionCarriesStatusAndReason(t *testing.T) {
	// BCA: "If the inquiryStatus and inquiryReason field are empty, BCA will
	// consider it a failed transaction and will be rejected." The same holds
	// for paymentFlagStatus/paymentFlagReason. A bare {responseCode,
	// responseMessage} is not a valid rejection.
	s := newServer(t)

	cases := []struct {
		name      string
		path      string
		payload   any
		statusKey string
		reasonKey string
		opts      []func(*requestOptions)
	}{
		{"inquiry not found", inquiryPath,
			inquiryPayload(testPartnerServiceID, "678901234567890240", "INQ-ENV-1"), "inquiryStatus", "inquiryReason", nil},
		{"inquiry mandatory field", inquiryPath,
			map[string]any{"partnerServiceId": testPartnerServiceID}, "inquiryStatus", "inquiryReason", nil},
		{"payment not found", paymentPath,
			paymentPayload(testPartnerServiceID, "678901234567890240", "PAY-ENV-1", "1000.00"),
			"paymentFlagStatus", "paymentFlagReason", nil},
		{"payment mandatory field", paymentPath,
			map[string]any{"partnerServiceId": testPartnerServiceID}, "paymentFlagStatus", "paymentFlagReason", nil},
		{"status not found", statusPath,
			statusPayload(testPartnerServiceID, "678901234567890240", "ST-ENV-1"),
			"paymentFlagStatus", "paymentFlagReason", nil},
		{"conflict", paymentPath,
			paymentPayload(testPartnerServiceID, "678901234567890240", "PAY-ENV-2", "1000.00"),
			"paymentFlagStatus", "paymentFlagReason", []func(*requestOptions){withExternalID("900000000000003")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := s.call(t, tc.path, tc.payload, tc.opts...)

			require.GreaterOrEqual(t, resp.status, 400, "expected a rejection: %s", resp.raw)
			data := resp.vaData(t)
			assert.Equal(t, domain.InquiryStatusFailed, data[tc.statusKey], resp.raw)

			reason, ok := data[tc.reasonKey].(map[string]any)
			require.True(t, ok, "%s is mandatory: %s", tc.reasonKey, resp.raw)
			assert.NotEmpty(t, reason["english"], resp.raw)
			assert.NotEmpty(t, reason["indonesia"], resp.raw)
		})
	}
}

func TestE2E_AuthFailuresCarryBareBody(t *testing.T) {
	// The documented exception: BCA's Appendix A shows "-" in the status
	// column for Unauthorized and Invalid Token, so those carry no
	// virtualAccountData.
	s := newServer(t)

	resp := s.call(t, inquiryPath,
		inquiryPayload(testPartnerServiceID, "678901234567890241", "INQ-AUTH-1"),
		withSecret("wrong-secret"))

	require.Equal(t, http.StatusUnauthorized, resp.status)
	assert.Equal(t, "4012400", resp.code())
	assert.NotContains(t, resp.body, "virtualAccountData")
}

func TestE2E_ResponseCodesCarryTheCalledServiceCode(t *testing.T) {
	// The single most common conformance failure: a middleware rejection
	// answering with a service code the endpoint does not own.
	s := newServer(t)

	for path, service := range map[string]string{
		inquiryPath: "24",
		paymentPath: "25",
		statusPath:  "26",
	} {
		resp := s.call(t, path, nil, withRawBody(`{`))

		code := resp.code()
		require.Len(t, code, 7, "responseCode must be AAABBCC: %s", resp.raw)
		assert.Equal(t, service, code[3:5], "%s must answer with service code %s, got %s", path, service, code)
	}
}

// BCA's header tables are closed sets. A request carrying an extra header must
// still be processed normally — the service reads only what BCA publishes, so
// a stray Idempotency-Key (which this service used to send, and which appears
// in no BCA document) can neither be required nor change the outcome.
func TestE2E_UndocumentedHeadersAreIgnored(t *testing.T) {
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890260", "1000.00")

	resp := s.call(t, paymentPath,
		paymentPayload(testPartnerServiceID, "678901234567890260", "PAY-HDR-1", "1000.00"),
		withExtraHeader("Idempotency-Key", "d3b07384-d9a0-4c9b-9b0e-1f2a3b4c5d6e"),
		withExtraHeader("X-CLIENT-KEY", "some-client-key"),
		withExtraHeader("X-DEVICE-ID", "device-1"))

	require.Equal(t, http.StatusOK, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentSuccess, resp.code())
}

// The signature covers METHOD:PATH:TOKEN:BODYHASH:TIMESTAMP and nothing else,
// so an extra header cannot invalidate it either.
func TestE2E_UndocumentedHeadersDoNotBreakTheSignature(t *testing.T) {
	s := newServer(t)

	resp := s.call(t, inquiryPath,
		inquiryPayload(testPartnerServiceID, "678901234567890261", "INQ-HDR-1"),
		withExtraHeader("Idempotency-Key", "irrelevant"))

	// The VA does not exist, but the request got as far as the business
	// decision — it was not rejected as unauthorized.
	require.Equal(t, http.StatusNotFound, resp.status, resp.raw)
	assert.Equal(t, domain.CodeInquiryNotFound, resp.code())
}
