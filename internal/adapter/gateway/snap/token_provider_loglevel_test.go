package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backbone-new/internal/infrastructure/config"
)

// tokenFetchWithLogLevel runs one token fetch against a stub vendor answering
// status, and returns the msg field of every line logged at level.
func tokenFetchWithLogLevel(t *testing.T, level slog.Level, status int) []string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = w.Write([]byte(`{"responseCode":"4017300","responseMessage":"Unauthorized. [Unknown client]"}`))
			return
		}
		_, _ = w.Write([]byte(`{"responseCode":"2007300","accessToken":"TOK","expiresIn":"900"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})))
	defer slog.SetDefault(restore)

	cfg := &config.VendorConfig{
		Vendor: "bca", Channel: "va",
		ClientID:               "cid",
		OutboundPrivateKeyPath: writeTestPrivateKey(t),
		BaseURL:                srv.URL,
		TokenEndpoint:          "/openapi/v1.0/access-token/b2b",
		RequestTimeout:         5,
	}
	_, _ = NewTokenProvider(cfg).AccessToken(context.Background())

	var msgs []string
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %s", line)
		}
		msgs = append(msgs, entry.Msg)
	}
	return msgs
}

// Silent at info, including on the 401 that fails every outbound call. The
// failure itself is not lost: the error is returned and the usecase logs it at
// Error with the vendor's responseMessage inside.
func TestTokenLogs_SilentAtInfo(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		if msgs := tokenFetchWithLogLevel(t, slog.LevelInfo, status); len(msgs) != 0 {
			t.Errorf("status %d: expected no log lines at info, got %v", status, msgs)
		}
	}
}

// At debug, all three appear — including on a rejection, which is the case the
// trail exists for.
func TestTokenLogs_PresentAtDebug(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		msgs := tokenFetchWithLogLevel(t, slog.LevelDebug, status)
		for _, want := range []string{"vendor_token_request", "vendor_token_request_full", "vendor_token_response"} {
			found := false
			for _, got := range msgs {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("status %d: %q missing at debug; got %v", status, want, msgs)
			}
		}
	}
}
