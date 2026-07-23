package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backbone-new/internal/infrastructure/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestSNAPAuthMiddleware_MissingHeaders(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

middleware := SNAPAuthMiddleware(vendorConfig)
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

	middleware := SNAPAuthMiddleware(vendorConfig)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSNAPAuthMiddleware_Success(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-TIMESTAMP", "2026-07-22T10:00:00+07:00")
	req.Header.Set("X-CLIENT-KEY", "test")
	req.Header.Set("X-SIGNATURE", "test")
	req.Header.Set("X-EXTERNAL-ID", "123456")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

	middleware := SNAPAuthMiddleware(vendorConfig)
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
	req.Header.Set("X-TIMESTAMP", "2026-07-22T10:00:00+07:00")
	req.Header.Set("X-CLIENT-KEY", "test")
	req.Header.Set("X-SIGNATURE", "test")
	// Missing X-EXTERNAL-ID for POST request
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		RequiredHeaders: []string{"X-TIMESTAMP", "X-CLIENT-KEY", "X-SIGNATURE"},
	}

	middleware := SNAPAuthMiddleware(vendorConfig)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSNAPAuthMiddleware_DefaultHeaders_NoClientKeyRequired(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-TIMESTAMP", "2026-07-22T10:00:00+07:00")
	req.Header.Set("X-SIGNATURE", "test")
	req.Header.Set("X-EXTERNAL-ID", "123456")
	// Deliberately no X-CLIENT-KEY: per ASPI spec it's only required on the
	// access-token endpoint, not on transfer-va transaction endpoints.
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{} // no RequiredHeaders set -> default applies

	middleware := SNAPAuthMiddleware(vendorConfig)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
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
