package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// prettyBody is deliberately whitespace-bearing: it is the only shape that can
// tell the minified digest apart from the raw one. A compact body hashes
// identically either way, which is exactly why the merchant/vendor divergence
// went unnoticed for so long.
const prettyBody = `{
  "partnerServiceId": "088899",
  "customerNo": "123456789012345678"
}`

// Both middlewares must derive the RequestBody component the same way, in
// full: Lowercase(HexEncode(SHA-256(minify(body)))), per SNAP. They disagreed
// twice — first on minification (the vendor side minified, the merchant side
// hashed raw bytes), then on the encoding (hex vs base64) — and each time the
// same body signed correctly for one endpoint was a 401 on the other, with
// nothing in the response to say why.
func TestBodyHashRule_IsIdenticalForMerchantAndVendor(t *testing.T) {
	minified := crypto.MinifyJSON([]byte(prettyBody))
	require.NotEqual(t, prettyBody, string(minified),
		"the fixture must actually contain whitespace, or this test proves nothing")

	expected := crypto.HashSHA256Hex(string(minified))
	assert.Equal(t, expected, crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashHex),
		"bodyHash must be hex(SHA-256(minify(body)))")
	assert.Equal(t, expected, hex.EncodeToString(sha256Sum(string(minified))),
		"and it must be lowercase hex of the raw digest, not a re-encoding of anything else")
}

// The canonical form must verify with the transition allowances turned OFF —
// that is what makes the flag a temporary allowance rather than the thing
// holding merchant auth up.
func TestMerchantAuth_HexBodyHash_AcceptedWithoutLegacyFlag(t *testing.T) {
	const path = "/openapi/v1.0/transfer-va/create-va"
	ts := time.Now().Format(time.RFC3339)

	req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts,
		crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashHex))

	assert.Equal(t, http.StatusOK, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", false), req))
}

// A merchant that verified against the vendor endpoints and reused that
// signing code must now get the same answer from the merchant endpoints.
func TestMerchantAndVendorAcceptTheSameBodyHash(t *testing.T) {
	ts := time.Now().Format(time.RFC3339)
	bodyHash := crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashHex)

	merchantReq := signedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va",
		"good-token", "shared-secret", ts, bodyHash)
	assert.Equal(t, http.StatusOK, runMerchantAuth(t, merchantAuthFor(t, "shared-secret", false), merchantReq))

	// Same rule on the vendor side, where AccessToken is the empty-string
	// convention rather than a bearer token.
	vendorPath := "/openapi/v1.0/transfer-va/payment"
	vendorReq := httptest.NewRequest(http.MethodPost, vendorPath, strings.NewReader(prettyBody))
	vendorReq.Header.Set("X-TIMESTAMP", ts)
	vendorReq.Header.Set("X-SIGNATURE", crypto.NewHMACSigner("shared-secret", "HMAC-SHA512").
		Sign(crypto.BuildStringToSign(http.MethodPost, vendorPath, "", bodyHash, ts)))
	vendorReq.Header.Set(headerExternalID, "123456")

	rec := httptest.NewRecorder()
	c := echo.New().NewContext(vendorReq, rec)
	mw := SNAPAuthMiddleware(&config.VendorConfig{
		ClientSecret:       "shared-secret",
		SignatureAlgorithm: "HMAC-SHA512",
		BodyHashEncoding:   crypto.BodyHashHex,
	}, nil, false)
	require.NoError(t, mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func merchantAuthFor(t *testing.T, secret string, acceptLegacy bool) echo.MiddlewareFunc {
	t.Helper()
	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return(secret, nil)
	return MerchantAuthMiddleware(mockIssuer, mockRepo, false, acceptLegacy)
}

// signedMerchantRequest signs prettyBody with the caller's choice of body
// digest, so a test can present either the new or the legacy convention.
func signedMerchantRequest(t *testing.T, path, token, secret, timestamp, bodyHash string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(prettyBody))
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, token, bodyHash, timestamp)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", crypto.NewHMACSigner(secret, "HMAC-SHA512").Sign(stringToSign))
	return req
}

func runMerchantAuth(t *testing.T, mw echo.MiddlewareFunc, req *http.Request) int {
	t.Helper()
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	require.NoError(t, mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})(c))
	return rec.Code
}

// Both superseded conventions are accepted only while the transition flag is
// on. Without it the switch to hex would have broken every merchant at deploy
// time, with no warning — the encoding change moves the digest for every body,
// not just the whitespace-bearing ones the earlier minification change touched.
func TestMerchantAuth_LegacyBodyHashes_AcceptedOnlyDuringTransition(t *testing.T) {
	const path = "/openapi/v1.0/transfer-va/create-va"

	legacyForms := map[string]string{
		"base64 of the minified body": crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashBase64),
		"base64 of the raw body":      crypto.HashSHA256Base64(prettyBody),
	}

	for name, bodyHash := range legacyForms {
		t.Run(name, func(t *testing.T) {
			ts := time.Now().Format(time.RFC3339)

			t.Run("accepted while MERCHANT_LEGACY_BODY_HASH is on", func(t *testing.T) {
				req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts, bodyHash)
				assert.Equal(t, http.StatusOK, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", true), req))
			})

			t.Run("rejected once it is turned off", func(t *testing.T) {
				req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts, bodyHash)
				assert.Equal(t, http.StatusUnauthorized, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", false), req))
			})
		})
	}
}

// The transition flag must not become a way in for a wrong secret.
func TestMerchantAuth_LegacyFallbackStillRequiresTheRightSecret(t *testing.T) {
	const path = "/openapi/v1.0/transfer-va/create-va"
	ts := time.Now().Format(time.RFC3339)

	req := signedMerchantRequest(t, path, "good-token", "wrong-secret", ts,
		crypto.HashSHA256Base64(prettyBody))

	assert.Equal(t, http.StatusUnauthorized, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", true), req))
}

// A vendor request signed over the raw (un-minified) body is rejected on both
// settings: the transition applies to merchants only, and the vendor side has
// always minified.
func TestVendorAuth_RawBodyHash_Rejected(t *testing.T) {
	ts := time.Now().Format(time.RFC3339)
	path := "/openapi/v1.0/transfer-va/payment"

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(prettyBody))
	rawHash := crypto.HashSHA256Hex(prettyBody)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, "", rawHash, ts)
	req.Header.Set("X-TIMESTAMP", ts)
	req.Header.Set("X-SIGNATURE", crypto.NewHMACSigner("vendor-secret", "HMAC-SHA512").Sign(stringToSign))
	req.Header.Set(headerExternalID, "123456")

	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	mw := SNAPAuthMiddleware(&config.VendorConfig{
		ClientSecret:       "vendor-secret",
		SignatureAlgorithm: "HMAC-SHA512",
	}, nil, false)
	require.NoError(t, mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})(c))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
