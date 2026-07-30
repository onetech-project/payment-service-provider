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
)

// newSignedRequest builds a POST request with a correctly-computed
// X-SIGNATURE (per the SNAP symmetric-signature convention already used by
// scripts/vendor-inquiry-va.sh) for the given secret/body/timestamp, so
// tests can exercise the happy path without duplicating the signing logic.
func newSignedRequest(t *testing.T, path, body, secret, timestamp string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	bodyHash := crypto.HashSHA256Hex(body)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSNAPAuthMiddleware_TimestampMissingTimezone_Rejected(t *testing.T) {
	// "2026-07-22T10:00:00" (no timezone offset) passes the loose
	// isValidISO8601 format check but fails strict time.RFC3339 parsing —
	// this must be rejected as unauthorized (feature 009's new freshness
	// check), not treated as an internal error.
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", strings.NewReader(`{}`))
	req.Header.Set("X-TIMESTAMP", "2026-07-22T10:00:00")
	req.Header.Set("X-SIGNATURE", "irrelevant")
	req.Header.Set("X-EXTERNAL-ID", "123456")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSNAPAuthMiddleware_StaleTimestamp_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/inquiry", body, "correct-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{ClientSecret: "correct-secret", SignatureAlgorithm: "HMAC-SHA512"}

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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
	bodyHash := crypto.HashSHA256Hex(body)
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

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer)
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

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
	assert.Contains(t, rec.Body.String(), "Authorization")
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

	middleware := SNAPAuthMiddleware(vendorConfig, nil)
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

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer)
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

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer)
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

	middleware := SNAPAuthMiddleware(vendorConfig, mockIssuer)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
	assert.Contains(t, rec.Body.String(), "expired")
}

func TestIsValidISO8601(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2026-07-22T10:00:00+07:00", true},
		{"2026-07-22T10:00:00Z", true},
		{"2026-07-22T10:00:00", true},
		{"invalid", false},
		{"2026-07-22", false},
		{"", false},
		{"2026/07/22T10:00:00", false},
	}

	for _, tt := range tests {
		result := isValidISO8601(tt.input)
		assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
	}
}
