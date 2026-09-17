package snap

import (
	"net/http"
	"strings"
	"testing"
)

const probeToken = "SUPERSECRETTOKENVALUE"

func requestHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Authorization", "Bearer "+probeToken)
	h.Set("X-SIGNATURE", "abc+/=")
	h.Set("X-EXTERNAL-ID", "20260917150446842681")
	return h
}

// The bearer token must never reach the log, in either rendering. It is valid
// for ~900s across every service this vendor exposes, and Loki retains what it
// is given.
func TestLoggableHeaders_RedactsBearerToken(t *testing.T) {
	joined := strings.Join(loggableHeaders(requestHeaders()), "\n")
	if strings.Contains(joined, probeToken) {
		t.Errorf("bearer token leaked into headers:\n%s", joined)
	}
	if !strings.Contains(joined, "Authorization: <redacted>") {
		t.Errorf("Authorization not redacted:\n%s", joined)
	}
}

func TestCurlCommand_RedactsBearerToken(t *testing.T) {
	got := curlCommand(http.MethodPost, "https://devapi.klikbca.com/x", requestHeaders(), []byte(`{"a":1}`))
	if strings.Contains(got, probeToken) {
		t.Errorf("bearer token leaked into curl: %s", got)
	}
	if !strings.Contains(got, "Bearer $BCA_ACCESS_TOKEN") {
		t.Errorf("no token placeholder in curl: %s", got)
	}
}

// X-SIGNATURE stays readable on purpose: it is a per-request MAC, useless
// without the redacted token, and comparing it against an independently
// computed signature is the point of logging the request.
func TestCurlCommand_KeepsSignature(t *testing.T) {
	got := curlCommand(http.MethodPost, "https://devapi.klikbca.com/x", requestHeaders(), nil)
	if !strings.Contains(got, "X-Signature: abc+/=") {
		t.Errorf("signature should not be redacted: %s", got)
	}
}

// The body must appear byte-for-byte: it is what the signature is computed
// over, so a command carrying an altered body reproduces nothing.
func TestCurlCommand_BodyVerbatim(t *testing.T) {
	body := `{"partnerServiceId":"   15975","additionalInfo":{}}`
	got := curlCommand(http.MethodPost, "https://x/y", http.Header{}, []byte(body))
	if !strings.Contains(got, "-d '"+body+"'") {
		t.Errorf("body not verbatim in curl: %s", got)
	}
}

// A quote inside a value must not end the shell string early, or the logged
// command silently becomes a different one.
func TestCurlCommand_QuotesAreEscaped(t *testing.T) {
	h := http.Header{}
	h.Set("X-Odd", "has'quote")
	got := curlCommand(http.MethodPost, "https://x/y", h, []byte(`{"k":"it's"}`))
	if strings.Contains(got, "has'quote") {
		t.Errorf("single quote not escaped: %s", got)
	}
	if !strings.Contains(got, `'"'"'`) {
		t.Errorf("expected shell quote-escape sequence: %s", got)
	}
}

// Sorted output, so two logged requests can be diffed line by line.
func TestLoggableHeaders_Sorted(t *testing.T) {
	got := loggableHeaders(requestHeaders())
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("headers not sorted: %q before %q", got[i-1], got[i])
		}
	}
}
