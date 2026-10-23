package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogger swaps Logger for one writing JSON at the given level into a
// buffer, restoring the original when the test ends.
func captureLogger(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := Logger
	Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))
	t.Cleanup(func() { Logger = orig })
	return &buf
}

func findLogEntry(t *testing.T, buf *bytes.Buffer, msg string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry["msg"] == msg {
			return entry
		}
	}
	return nil
}

func TestTelemetryMiddleware_LogsFullRequestAtDebug(t *testing.T) {
	buf := captureLogger(t, slog.LevelDebug)

	body := `{"partnerServiceId":"  12345","paidAmount":{"value":"10000.00","currency":"IDR"},"clientSecret":"s3cr3t","nested":[{"pin":"123456"}]}`
	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment?access_token=abc&page=1", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer eyJhbGciOi.secret.token")
	req.Header.Set("X-Admin-API-Key", "admin-key")
	req.Header.Set("X-SIGNATURE", "sig-value")
	req.Header.Set("X-PARTNER-ID", "partner-1")
	rec := httptest.NewRecorder()

	var seenBody string
	next := func(c echo.Context) error {
		b, err := io.ReadAll(c.Request().Body)
		require.NoError(t, err)
		seenBody = string(b)
		return c.NoContent(http.StatusOK)
	}
	require.NoError(t, TelemetryMiddleware()(next)(echo.New().NewContext(req, rec)))

	// The handler must still see the original, unredacted body.
	assert.Equal(t, body, seenBody)

	entry := findLogEntry(t, buf, "http_request_full")
	require.NotNil(t, entry, "http_request_full not logged; got: %s", buf.String())

	logged := buf.String()
	for _, secret := range []string{"eyJhbGciOi", "admin-key", "s3cr3t", "123456", "abc"} {
		assert.NotContains(t, logged, secret)
	}

	headers := entry["headers"].([]any)
	assert.Contains(t, headers, "Authorization: <redacted>")
	assert.Contains(t, headers, "X-Admin-Api-Key: <redacted>")
	assert.Contains(t, headers, "X-Signature: sig-value")
	assert.Contains(t, headers, "X-Partner-Id: partner-1")

	loggedBody := entry["body"].(string)
	assert.Contains(t, loggedBody, `"value":"10000.00"`)
	assert.Contains(t, loggedBody, `"partnerServiceId":"  12345"`)
	assert.Contains(t, entry["query"], "page=1")
}

func TestTelemetryMiddleware_BodyWithoutSecretsLoggedVerbatim(t *testing.T) {
	buf := captureLogger(t, slog.LevelDebug)

	// Key order and whitespace must survive, so the logged body can be hashed
	// to check the client's signature.
	body := "{\"virtualAccountNo\": \"  1234\",\n \"amount\":{\"value\":\"1.00\"}}"
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	next := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	require.NoError(t, TelemetryMiddleware()(next)(echo.New().NewContext(req, rec)))

	entry := findLogEntry(t, buf, "http_request_full")
	require.NotNil(t, entry)
	assert.Equal(t, body, entry["body"])
}

func TestTelemetryMiddleware_NoFullRequestLogAboveDebug(t *testing.T) {
	buf := captureLogger(t, slog.LevelInfo)

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	next := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	require.NoError(t, TelemetryMiddleware()(next)(echo.New().NewContext(req, rec)))

	assert.Nil(t, findLogEntry(t, buf, "http_request_full"))
	assert.NotNil(t, findLogEntry(t, buf, "http_request"))
}

func TestRedactBody_Form(t *testing.T) {
	out := redactBody(echo.MIMEApplicationForm, []byte("grant_type=client_credentials&client_secret=xyz"))
	assert.NotContains(t, out, "xyz")
	assert.Contains(t, out, "grant_type=client_credentials")
}
