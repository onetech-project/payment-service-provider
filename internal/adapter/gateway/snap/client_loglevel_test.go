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

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
)

// callWithLogLevel runs one status inquiry against a stub vendor with the
// default logger set to level, and returns the msg field of every line logged.
func callWithLogLevel(t *testing.T, level slog.Level, status int) []string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"responseCode":"4042601","virtualAccountData":{"paymentFlagStatus":""}}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})))
	defer slog.SetDefault(restore)

	cfg := &config.VendorConfig{
		Vendor: "bca", Channel: "va",
		ClientSecret: "secret", BaseURL: srv.URL,
		ChannelID: "95231", OutboundPartnerID: "15975",
		RequestTimeout: 5, SignatureAlgorithm: "HMAC-SHA512", BodyHashEncoding: "hex",
	}
	_, _ = NewClient(cfg, nil).PaymentStatus(context.Background(), &domain.VAStatusRequest{
		PartnerServiceID: "   15975", CustomerNo: "060000000000000046",
		VirtualAccountNo: "   15975060000000000000046",
		InquiryRequestID: "202609031611148444900051593962",
	})

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

// At the default info level the outbound trail must be silent: it is the full
// request and response body on every call, which is a diagnostic tool, not
// something a healthy deployment ships to Loki continuously.
func TestVendorLogs_SilentAtInfo(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusGatewayTimeout} {
		if msgs := callWithLogLevel(t, slog.LevelInfo, status); len(msgs) != 0 {
			t.Errorf("status %d: expected no log lines at info, got %v", status, msgs)
		}
	}
}

// At debug all three appear, so LOG_LEVEL=debug is all it takes to get the
// trail back — no rebuild.
func TestVendorLogs_PresentAtDebug(t *testing.T) {
	msgs := callWithLogLevel(t, slog.LevelDebug, http.StatusOK)
	for _, want := range []string{"vendor_api_request", "vendor_api_request_full", "vendor_api_response"} {
		found := false
		for _, got := range msgs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q missing at debug; got %v", want, msgs)
		}
	}
}

// A 5xx is logged at the same level as a 2xx: this is the vendor's raw answer,
// and the caller is what decides and logs the outcome. Splitting one event name
// across levels would give anyone filtering for it a partial view.
func TestVendorLogs_FailureNotEscalated(t *testing.T) {
	if msgs := callWithLogLevel(t, slog.LevelInfo, http.StatusGatewayTimeout); len(msgs) != 0 {
		t.Errorf("a 504 must not escalate past debug, got %v", msgs)
	}
}
