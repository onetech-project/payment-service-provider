package middleware

import (
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

// Both middlewares must derive the RequestBody component the same way:
// SHA-256 over the MINIFIED JSON, per SNAP. They used to disagree — the vendor
// side minified, the merchant side hashed raw bytes — so the same body signed
// correctly for one endpoint was a 401 on the other, with nothing in the
// response to say why.
func TestBodyHashRule_IsIdenticalForMerchantAndVendor(t *testing.T) {
	minified := crypto.MinifyJSON([]byte(prettyBody))
	require.NotEqual(t, prettyBody, string(minified),
		"the fixture must actually contain whitespace, or this test proves nothing")

	// Same rule, differing only in the encoding each side is configured for.
	assert.Equal(t,
		crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashBase64),
		crypto.HashSHA256Base64(string(minified)),
		"merchant bodyHash must be base64(SHA-256(minify(body)))")
	assert.Equal(t,
		crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashHex),
		crypto.HashSHA256Hex(string(minified)),
		"vendor bodyHash must be hex(SHA-256(minify(body)))")
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

func TestMerchantAuth_MinifiedBodyHash_Accepted(t *testing.T) {
	const path = "/openapi/v1.0/transfer-va/create-va"
	ts := time.Now().Format(time.RFC3339)

	req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts,
		crypto.HashRequestBody([]byte(prettyBody), crypto.BodyHashBase64))

	assert.Equal(t, http.StatusOK, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", false), req))
}

// The legacy raw-body digest is accepted only while the transition flag is on.
// Without it the change would have broken, at deploy time and with no warning,
// exactly those merchants that send pretty-printed JSON.
func TestMerchantAuth_LegacyRawBodyHash_AcceptedOnlyDuringTransition(t *testing.T) {
	const path = "/openapi/v1.0/transfer-va/create-va"
	ts := time.Now().Format(time.RFC3339)
	legacyHash := crypto.HashSHA256Base64(prettyBody)

	t.Run("accepted while MERCHANT_LEGACY_BODY_HASH is on", func(t *testing.T) {
		req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts, legacyHash)
		assert.Equal(t, http.StatusOK, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", true), req))
	})

	t.Run("rejected once it is turned off", func(t *testing.T) {
		req := signedMerchantRequest(t, path, "good-token", "merchant-secret", ts, legacyHash)
		assert.Equal(t, http.StatusUnauthorized, runMerchantAuth(t, merchantAuthFor(t, "merchant-secret", false), req))
	})
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
