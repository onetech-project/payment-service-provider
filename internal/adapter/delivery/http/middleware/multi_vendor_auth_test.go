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
	"github.com/stretchr/testify/require"
)

func vendorConfigFor(vendorName, secret, partnerID, channelID, encoding string, strict bool) *config.VendorConfig {
	return &config.VendorConfig{
		Vendor:                vendorName,
		Channel:               "va",
		ClientSecret:          secret,
		PartnerID:             partnerID,
		ChannelID:             channelID,
		SignatureAlgorithm:    "HMAC-SHA512",
		BodyHashEncoding:      encoding,
		StrictMandatoryFields: strict,
		RequiredHeaders:       []string{headerTimestamp, headerSignature},
	}
}

func signedVendorRequest(path, body, secret, encoding, partnerID, channelID, timestamp string) *http.Request {
	bodyHash := crypto.HashRequestBody([]byte(body), encoding)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, "", bodyHash, timestamp)
	signature := crypto.NewHMACSigner(secret, "HMAC-SHA512").Sign(stringToSign)

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(headerTimestamp, timestamp)
	req.Header.Set(headerSignature, signature)
	req.Header.Set(headerExternalID, "123456")
	req.Header.Set(headerPartnerID, partnerID)
	req.Header.Set(headerChannelID, channelID)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

// TestMultiVendorSNAPAuth_EveryVendorReachable is the regression test for the
// routing defect this middleware replaced: registering the same method+path
// once per vendor does not stack in echo — the last registration wins — so
// only the final vendor in the config list was ever reachable, and every other
// vendor's traffic was rejected against the wrong credentials.
func TestMultiVendorSNAPAuth_EveryVendorReachable(t *testing.T) {
	vendors := []*config.VendorConfig{
		vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true),
		vendorConfigFor("legacy", "legacy-secret", "67890", "77001", crypto.BodyHashBase64, false),
		vendorConfigFor("third", "third-secret", "11111", "88002", crypto.BodyHashHex, true),
	}

	for _, vendorConfig := range vendors {
		t.Run(vendorConfig.Vendor, func(t *testing.T) {
			e := echo.New()
			body := `{"partnerServiceId":"` + vendorConfig.PartnerID + `"}`
			timestamp := time.Now().Format(time.RFC3339)
			req := signedVendorRequest("/openapi/v1.0/transfer-va/payment", body,
				vendorConfig.ClientSecret, vendorConfig.BodyHashEncoding,
				vendorConfig.PartnerID, vendorConfig.ChannelID, timestamp)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			called := false
			handler := MultiVendorSNAPAuth(vendors, nil, false)(func(c echo.Context) error {
				called = true
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			require.NoError(t, handler(c))
			assert.True(t, called, "vendor %s must be reachable", vendorConfig.Vendor)
			assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		})
	}
}

func TestMultiVendorSNAPAuth_RecordsResolvedVendorOnContext(t *testing.T) {
	// The handler reads field strictness from here, so resolution has to
	// identify the vendor, not merely accept the request.
	vendors := []*config.VendorConfig{
		vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true),
		vendorConfigFor("legacy", "legacy-secret", "67890", "77001", crypto.BodyHashBase64, false),
	}

	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	req := signedVendorRequest("/openapi/v1.0/transfer-va/payment", `{}`,
		"legacy-secret", crypto.BodyHashBase64, "67890", "77001", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var resolved domain.VendorContext
	handler := MultiVendorSNAPAuth(vendors, nil, false)(func(c echo.Context) error {
		resolved, _ = c.Get(domain.ContextKeyVendor).(domain.VendorContext)
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, handler(c))
	assert.Equal(t, "legacy", resolved.Vendor)
	assert.False(t, resolved.StrictMandatoryFields, "the resolved vendor's own field rules must apply")
}

func TestMultiVendorSNAPAuth_CredentialsDoNotCrossVendors(t *testing.T) {
	vendors := []*config.VendorConfig{
		vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true),
		vendorConfigFor("legacy", "legacy-secret", "67890", "77001", crypto.BodyHashBase64, false),
	}

	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	// BCA's headers, legacy's secret.
	req := signedVendorRequest("/openapi/v1.0/transfer-va/payment", `{}`,
		"legacy-secret", crypto.BodyHashHex, "12345", "95231", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	handler := MultiVendorSNAPAuth(vendors, nil, false)(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, handler(c))
	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "4012500")
}

func TestMultiVendorSNAPAuth_UnknownPartnerRejected(t *testing.T) {
	vendors := []*config.VendorConfig{
		vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true),
	}

	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	req := signedVendorRequest("/openapi/v1.0/transfer-va/inquiry", `{}`,
		"bca-secret", crypto.BodyHashHex, "99999", "95231", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := MultiVendorSNAPAuth(vendors, nil, false)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, handler(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "4012400")
}

func TestMultiVendorSNAPAuth_BodyIsReadableByTheHandler(t *testing.T) {
	// The middleware reads the body to hash it, and does so once for all
	// candidates. The handler must still be able to bind it afterwards.
	vendors := []*config.VendorConfig{
		vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true),
		vendorConfigFor("legacy", "legacy-secret", "67890", "77001", crypto.BodyHashHex, true),
	}

	e := echo.New()
	body := `{"partnerServiceId":"67890","customerNo":"123"}`
	timestamp := time.Now().Format(time.RFC3339)
	req := signedVendorRequest("/openapi/v1.0/transfer-va/payment", body,
		"legacy-secret", crypto.BodyHashHex, "67890", "77001", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var received map[string]string
	handler := MultiVendorSNAPAuth(vendors, nil, false)(func(c echo.Context) error {
		require.NoError(t, c.Bind(&received))
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, handler(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "67890", received["partnerServiceId"])
	assert.Equal(t, "123", received["customerNo"])
}

// BCA's header tables are closed sets. These pin that the service neither
// requires nor depends on anything outside them — in particular not
// Idempotency-Key, which appears in no BCA document, and not X-CLIENT-KEY,
// which belongs to the access-token endpoint alone.
func TestIsDocumentedTransferVAHeader(t *testing.T) {
	for _, header := range []string{
		"Content-Type", "Authorization", "X-TIMESTAMP", "X-SIGNATURE",
		"ORIGIN", "CHANNEL-ID", "X-PARTNER-ID", "X-EXTERNAL-ID",
	} {
		assert.True(t, isDocumentedTransferVAHeader(header), "%s is published for transfer-va", header)
	}

	for _, header := range []string{
		"Idempotency-Key", "X-CLIENT-KEY", "X-CLIENT-SECRET", "X-IP-ADDRESS",
		"X-DEVICE-ID", "X-LATITUDE", "X-LONGITUDE",
	} {
		assert.False(t, isDocumentedTransferVAHeader(header), "%s is not a transfer-va header", header)
	}

	// HTTP header names are case-insensitive.
	assert.True(t, isDocumentedTransferVAHeader("x-external-id"))
	assert.True(t, isDocumentedTransferVAHeader("  Channel-Id  "))
}

func TestMultiVendorSNAPAuth_ConfigCannotRequireAnUndocumentedHeader(t *testing.T) {
	// A vendor config listing X-CLIENT-KEY (a real deployment did) would
	// otherwise reject every conformant BCA request with a mandatory-field
	// error about a header BCA has no way to send.
	cfg := vendorConfigFor("bca", "bca-secret", "12345", "95231", crypto.BodyHashHex, true)
	cfg.RequiredHeaders = []string{
		headerTimestamp, headerSignature, "X-CLIENT-KEY", "Idempotency-Key",
	}

	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	req := signedVendorRequest("/openapi/v1.0/transfer-va/inquiry", `{}`,
		"bca-secret", crypto.BodyHashHex, "12345", "95231", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	handler := MultiVendorSNAPAuth([]*config.VendorConfig{cfg}, nil, false)(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, handler(c))
	assert.True(t, called, "the undocumented entries must be ignored, not enforced")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
