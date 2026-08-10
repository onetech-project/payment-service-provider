package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
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
)

// sha256Sum is a small helper for constructing a raw body hash in
// TestSNAPAuthMiddleware_HexSignatureEncoding_Rejected.
func sha256Sum(data string) []byte {
	h := sha256.New()
	h.Write([]byte(data))
	return h.Sum(nil)
}

// newSignedRequest builds a POST request with a correctly-computed
// X-SIGNATURE (per the SNAP symmetric-signature convention already used by
// scripts/vendor-inquiry-va.sh) for the given secret/body/timestamp, so
// tests can exercise the happy path without duplicating the signing logic.
func newSignedRequest(t *testing.T, path, body, secret, timestamp string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	// Hex is the BCA/SNAP spec encoding for the body-hash component and the
	// default for a VendorConfig that does not pin one.
	bodyHash := crypto.HashRequestBody([]byte(body), crypto.BodyHashHex)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, "", bodyHash, timestamp)
	signature := crypto.NewHMACSigner(secret, "HMAC-SHA512").Sign(stringToSign)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	req.Header.Set("X-EXTERNAL-ID", "123456")
	return req
}

func TestSNAPAuthMiddleware_MissingHeaders(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	// A missing mandatory header is "Invalid Mandatory Field" (400), not
	// Unauthorized — BCA's Appendix A separates the two.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid Mandatory Field")
}

func TestSNAPAuthMiddleware_InvalidTimestamp(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-TIMESTAMP", "invalid")
	req.Header.Set("X-CLIENT-KEY", "test")
	req.Header.Set("X-SIGNATURE", "test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSNAPAuthMiddleware_Success(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/test", body, "test-secret", timestamp)
	req.Header.Set("X-CLIENT-KEY", "test")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders:    []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
		ClientSecret:       "test-secret",
		SignatureAlgorithm: "HMAC-SHA512",
	}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSNAPAuthMiddleware_MissingExternalID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-TIMESTAMP", time.Now().Format(time.RFC3339))
	req.Header.Set("X-CLIENT-KEY", "test")
	req.Header.Set("X-SIGNATURE", "test")
	// Missing X-EXTERNAL-ID for POST request
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSNAPAuthMiddleware_DefaultHeaders_NoClientKeyRequired(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/test", body, "test-secret", timestamp)
	// Deliberately no X-CLIENT-KEY: per ASPI spec it's only required on the
	// access-token endpoint, not on transfer-va transaction endpoints.
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "test-secret", SignatureAlgorithm: "HMAC-SHA512"} // no RequiredHeaders set -> default applies

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Signature & Timestamp Freshness Enforcement Tests (feature 009-transfer-va-auth) ---

func TestSNAPAuthMiddleware_ValidSignature_PassesThrough(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973","customerNo":"04000000000000000001"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "handler should be invoked for a correctly-signed request")
}

func TestSNAPAuthMiddleware_InvalidSignature_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	req.Header.Set("X-SIGNATURE", "clearly-wrong-signature")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called, "handler must not be invoked for a mismatched signature")
}

func TestSNAPAuthMiddleware_HexEncodedSignature_Rejected(t *testing.T) {
	// X-SIGNATURE itself is always base64 ("X-SIGNATURE should be encoded by
	// Base64" — Developer API BCA). A hex-encoded HMAC must be rejected
	// regardless of how the body-hash component is encoded.
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	bodyHashHex := hex.EncodeToString(sha256Sum(body))
	stringToSign := crypto.BuildStringToSign(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", "", bodyHashHex, timestamp)
	mac := hmac.New(sha512.New, []byte("correct-secret"))
	mac.Write([]byte(stringToSign))
	signatureHex := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", strings.NewReader(body))
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signatureHex)
	req.Header.Set("X-EXTERNAL-ID", "123456")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called, "handler must not be invoked for a hex-signed (pre-migration) request")
}

func TestSNAPAuthMiddleware_EmptySignatureValue_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", strings.NewReader(`{}`))
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", "")
	req.Header.Set("X-EXTERNAL-ID", "123456")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	// An empty X-SIGNATURE is an absent mandatory header: 4002402 on the
	// inquiry service, not a 401.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "4002402")
}

func TestSNAPAuthMiddleware_MissingSecret_FailsClosed(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	// Even a "correctly" signed request (against an empty secret) must be rejected.
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSNAPAuthMiddleware_TimestampMissingTimezone_Rejected(t *testing.T) {
	// "2026-07-22T10:00:00" (no timezone offset) is not ISO 8601 as BCA
	// defines it — the timezone designator is mandatory — so it is rejected
	// as Invalid Field Format [X-TIMESTAMP] rather than being treated as an
	// internal error or silently accepted.
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", strings.NewReader(`{}`))
	req.Header.Set("X-TIMESTAMP", "2026-07-22T10:00:00")
	req.Header.Set("X-SIGNATURE", "irrelevant")
	req.Header.Set("X-EXTERNAL-ID", "123456")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid Field Format [X-TIMESTAMP]")
}

func TestSNAPAuthMiddleware_StaleTimestamp_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSNAPAuthMiddleware_FutureTimestamp_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSNAPAuthMiddleware_TimestampWithinWindow_PassesThrough(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(4 * time.Minute).Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

func TestSNAPAuthMiddleware_SkewCheckSkippedWhenFlagSet(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, true)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

// --- Vendor Access Token in Symmetric Signature Tests (feature 011-vendor-access-token-signature) ---

// newTokenBoundSignedRequest builds a POST request carrying an Authorization
// bearer token and a correctly-computed X-SIGNATURE where the AccessToken
// component of stringToSign is the real token — used to exercise migrated
// vendor configs (ClientID set), unlike newSignedRequest's legacy empty
// AccessToken component.
func newTokenBoundSignedRequest(t *testing.T, path, body, token, secret, timestamp string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	bodyHash := crypto.HashRequestBody([]byte(body), crypto.BodyHashHex)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, token, bodyHash, timestamp)
	signature := crypto.NewHMACSigner(secret, "HMAC-SHA512").Sign(stringToSign)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	req.Header.Set("X-EXTERNAL-ID", "123456")
	return req
}

func TestSNAPAuthMiddleware_MigratedVendor_ValidBoundToken_Accepted(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	token := "valid-vendor-token"
	req := newTokenBoundSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, token, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientID: "vendor-client-1", ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}
	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", token).Return(&domain.TokenClaims{ClientID: "vendor-client-1"}, nil)

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "handler should be invoked for a valid bound token + signature")
}

func TestSNAPAuthMiddleware_MigratedVendor_MissingAuthorization_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	// Legacy signing convention: no Authorization header, empty AccessToken component.
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientID: "vendor-client-1", ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}
	mockIssuer := new(MockJWTIssuer)

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
	// BCA publishes 4012401 "Invalid Token (B2B)" for a missing/invalid
	// bearer token on the inquiry service.
	assert.Contains(t, rec.Body.String(), "4012401")
	assert.Contains(t, rec.Body.String(), "Invalid Token (B2B)")
	assert.NotContains(t, rec.Body.String(), "Invalid signature")
	mockIssuer.AssertNotCalled(t, "ValidateToken", mock.Anything)
}

func TestSNAPAuthMiddleware_LegacyVendor_NoClientID_UnchangedBehavior(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// ClientID deliberately empty: legacy vendor, not yet migrated.
	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "legacy (non-migrated) vendor requests must remain unaffected")
}

func TestSNAPAuthMiddleware_MigratedVendor_TokenSwappedAfterSigning_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	tokenA := "token-a"
	tokenB := "token-b"
	// Signature computed with tokenA, but Authorization carries tokenB.
	req := newTokenBoundSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, tokenA, "correct-secret", timestamp)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientID: "vendor-client-1", ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}
	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", tokenB).Return(&domain.TokenClaims{ClientID: "vendor-client-1"}, nil)

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestSNAPAuthMiddleware_MigratedVendor_TokenClientIDMismatch_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	token := "other-vendor-token"
	req := newTokenBoundSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, token, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientID: "vendor-client-1", ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}
	mockIssuer := new(MockJWTIssuer)
	// Token is well-formed and valid, but was issued for a different client_id.
	mockIssuer.On("ValidateToken", token).Return(&domain.TokenClaims{ClientID: "some-other-client"}, nil)

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestSNAPAuthMiddleware_MigratedVendor_ExpiredToken_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	token := "expired-token"
	req := newTokenBoundSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, token, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientID: "vendor-client-1", ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}
	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", token).Return(nil, assert.AnError)

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
	assert.Contains(t, rec.Body.String(), "4012401")
	assert.Contains(t, rec.Body.String(), "Invalid Token (B2B)")
}

// The loose isValidISO8601 helper is gone: X-TIMESTAMP is now parsed with
// time.Parse(time.RFC3339) directly, which is what BCA requires (ISO 8601
// *with* a timezone designator). The behaviour it used to guard is covered by
// TestSNAPAuthMiddleware_TimestampMissingTimezone_Rejected and
// TestSNAPAuthMiddleware_InvalidTimestampFormat_Rejected.
