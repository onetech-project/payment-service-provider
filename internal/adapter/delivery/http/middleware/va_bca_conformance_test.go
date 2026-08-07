package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/infrastructure/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BCA documents X-EXTERNAL-ID as a "Numeric String" on all three transfer-va
// services. Only its length was checked, so a non-numeric id sailed through
// here and was rejected downstream at BCA instead — with a response this
// service could not explain.
func TestSNAPAuthMiddleware_ExternalIDMustBeNumeric(t *testing.T) {
	for _, tc := range []struct {
		name       string
		externalID string
		wantStatus int
	}{
		{"numeric is accepted", "123456789012345678901234567890123456", http.StatusOK},
		{"alphanumeric is rejected", "EXT-20260806-0001", http.StatusBadRequest},
		{"letters are rejected", "abcdef", http.StatusBadRequest},
		{"over-long is still rejected", "1234567890123456789012345678901234567", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			timestamp := time.Now().Format(time.RFC3339)
			body := `{"partnerServiceId":"15973"}`
			req := newSignedRequest(t, "/openapi/v1.0/transfer-va/payment", body, "test-secret", timestamp)
			req.Header.Set(headerExternalID, tc.externalID)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			vendorConfig := &config.VendorConfig{
				ClientSecret:       "test-secret",
				SignatureAlgorithm: "HMAC-SHA512",
				// BCA's field tables are authoritative for this vendor.
				StrictMandatoryFields: true,
			}
			middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
			handler := middleware(func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			require.NoError(t, handler(c))
			assert.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())

			if tc.wantStatus != http.StatusBadRequest {
				return
			}
			// The rejection must name the offending header and carry the
			// payment service code (25), not a generic one.
			var parsed struct {
				ResponseCode    string `json:"responseCode"`
				ResponseMessage string `json:"responseMessage"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
			assert.Equal(t, "4002501", parsed.ResponseCode)
			assert.Contains(t, parsed.ResponseMessage, headerExternalID)
		})
	}
}

// The numeric rule is BCA's, not ASPI's (which types X-EXTERNAL-ID as a plain
// string). A vendor not configured for BCA's field tables keeps its
// alphanumeric ids — this gateway fronts more than one vendor and must not
// impose one vendor's table on the others.
func TestSNAPAuthMiddleware_NonStrictVendorKeepsAlphanumericExternalID(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/payment", body, "test-secret", timestamp)
	req.Header.Set(headerExternalID, "EXT-20260806-0001")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		ClientSecret:          "test-secret",
		SignatureAlgorithm:    "HMAC-SHA512",
		StrictMandatoryFields: false,
	}
	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	require.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Length is BCA-independent and stays enforced for every vendor.
func TestSNAPAuthMiddleware_NonStrictVendorStillRejectsOverlongExternalID(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"15973"}`
	req := newSignedRequest(t, "/openapi/v1.0/transfer-va/payment", body, "test-secret", timestamp)
	req.Header.Set(headerExternalID, "1234567890123456789012345678901234567")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	vendorConfig := &config.VendorConfig{
		ClientSecret:          "test-secret",
		SignatureAlgorithm:    "HMAC-SHA512",
		StrictMandatoryFields: false,
	}
	middleware := SNAPAuthMiddleware(vendorConfig, nil, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	require.NoError(t, handler(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestIsNumericString(t *testing.T) {
	assert.True(t, isNumericString("0"))
	assert.True(t, isNumericString("123456789012345678901234567890123456"))
	assert.False(t, isNumericString(""))
	assert.False(t, isNumericString("12 34"))
	assert.False(t, isNumericString("12.34"))
	assert.False(t, isNumericString("-1234"))
	// Non-ASCII digits are not ASCII digits.
	assert.False(t, isNumericString("١٢٣"))
}
