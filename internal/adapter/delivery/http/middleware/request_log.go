package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
)

const redacted = "<redacted>"

// maxLoggedBodyBytes caps the body written to the log line. The handler still
// receives the full body; only the copy in Loki is cut short.
const maxLoggedBodyBytes = 64 << 10

// sensitiveHeaders are inbound headers whose values are credentials rather
// than request facts, keyed by canonical name. Each one is replaced outright,
// never truncated — a prefix of a token is still part of the token.
//
// X-SIGNATURE is deliberately absent, for the same reason as on the outbound
// side (gateway/snap/curl_log.go): it is a MAC over this one request, useless
// without the redacted bearer token, and comparing it against an
// independently computed signature is most of the point of logging a request.
var sensitiveHeaders = map[string]bool{
	"Authorization":          true,
	"Authorization-Customer": true,
	"Proxy-Authorization":    true,
	"Cookie":                 true,
	"X-Admin-Api-Key":        true,
	// Inputs of the /api/v1/utilities/signature-* helpers.
	"X-Client-Secret": true,
	"X-Private-Key":   true,
	"Accesstoken":     true,
}

// sensitiveFields are body and query keys whose values are redacted,
// matched after normalizeFieldName so clientSecret, client_secret and
// CLIENT-SECRET are all caught.
var sensitiveFields = map[string]bool{
	"secret":        true,
	"secretvalue":   true,
	"clientsecret":  true,
	"password":      true,
	"passwd":        true,
	"pin":           true,
	"otp":           true,
	"cvv":           true,
	"token":         true,
	"accesstoken":   true,
	"refreshtoken":  true,
	"authorization": true,
	"apikey":        true,
	"privatekey":    true,
}

func normalizeFieldName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// logInboundRequest writes the full inbound request — headers, query and body
// — at Debug, with credentials redacted. It is the inbound counterpart of
// vendor_api_request_full: when a vendor reports a rejected call, this is the
// record of exactly what they sent.
//
// Guarded by Enabled() because the body has to be read and the headers
// rendered before slog can decide to discard them; at Info and above this
// costs nothing and leaves req.Body untouched.
func logInboundRequest(c echo.Context, correlationID string) {
	req := c.Request()
	if !Logger.Enabled(req.Context(), slog.LevelDebug) {
		return
	}

	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		body, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		// Hand the handler the bytes it would have read itself, so logging
		// never changes what Bind or the body-hash check sees.
		req.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			Logger.DebugContext(req.Context(), "http_request_full",
				slog.String("correlation_id", correlationID),
				slog.String("error", "read body: "+err.Error()),
			)
			return
		}
	}

	attrs := []any{
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.String("remote_ip", c.RealIP()),
		slog.String("correlation_id", correlationID),
		slog.Any("headers", redactHeaders(req.Header)),
	}
	if req.URL.RawQuery != "" {
		attrs = append(attrs, slog.String("query", redactQuery(req.URL.RawQuery)))
	}
	if len(body) > 0 {
		attrs = append(attrs,
			slog.Int("body_bytes", len(body)),
			slog.String("body", redactBody(req.Header.Get(echo.HeaderContentType), body)),
		)
	}
	Logger.DebugContext(req.Context(), "http_request_full", attrs...)
}

// redactHeaders renders headers as "Name: value" lines sorted by name, so two
// log entries can be diffed against each other.
func redactHeaders(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, name := range names {
		value := strings.Join(h.Values(name), ", ")
		if sensitiveHeaders[http.CanonicalHeaderKey(name)] {
			value = redacted
		}
		out = append(out, name+": "+value)
	}
	return out
}

func redactQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return redacted
	}
	if !redactValues(values) {
		return raw
	}
	return values.Encode()
}

func redactValues(values url.Values) bool {
	changed := false
	for key, vs := range values {
		if sensitiveFields[normalizeFieldName(key)] {
			for i := range vs {
				vs[i] = redacted
			}
			changed = true
		}
	}
	return changed
}

// redactBody returns the body as it should appear in the log. A body with
// nothing to redact is returned byte-for-byte, so the logged copy can still be
// hashed to check a client's X-SIGNATURE; only a body that actually carried a
// secret is re-encoded.
func redactBody(contentType string, body []byte) string {
	var out string
	switch {
	case strings.HasPrefix(contentType, echo.MIMEApplicationForm):
		out = redactQuery(string(body))
	case json.Valid(body):
		out = redactJSON(body)
	default:
		out = string(body)
	}
	if len(out) > maxLoggedBodyBytes {
		out = out[:maxLoggedBodyBytes] + "...<truncated>"
	}
	return out
}

func redactJSON(body []byte) string {
	dec := json.NewDecoder(bytes.NewReader(body))
	// UseNumber keeps amounts like "10000.00" from round-tripping through
	// float64 and coming back altered.
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(body)
	}
	if !redactJSONValue(v) {
		return string(body)
	}
	out, err := json.Marshal(v)
	if err != nil {
		// Never fall back to the original here: it is known to hold a secret.
		return redacted
	}
	return string(out)
}

func redactJSONValue(v any) bool {
	changed := false
	switch t := v.(type) {
	case map[string]any:
		for key, child := range t {
			if sensitiveFields[normalizeFieldName(key)] {
				t[key] = redacted
				changed = true
				continue
			}
			if redactJSONValue(child) {
				changed = true
			}
		}
	case []any:
		for _, child := range t {
			if redactJSONValue(child) {
				changed = true
			}
		}
	}
	return changed
}
