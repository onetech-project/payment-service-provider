package snap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticToken is a VendorTokenProvider that hands back a fixed token, so a
// test can assert on exactly what got bound into the signature.
type staticToken struct {
	token string
	err   error
	calls int32
}

func (s *staticToken) AccessToken(context.Context) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	return s.token, s.err
}

func outboundConfig(baseURL string) *config.VendorConfig {
	return &config.VendorConfig{
		Vendor:             "bca",
		Channel:            "va",
		ClientSecret:       "vendor-secret",
		SignatureAlgorithm: "HMAC-SHA512",
		BodyHashEncoding:   crypto.BodyHashHex,
		RequestTimeout:     5,
		BaseURL:            baseURL,
		ChannelID:          "95231",
		PartnerID:          "12345",
		APIEndpoints:       map[string]string{"STATUS": "/openapi/v2.0/transfer-va/status"},
	}
}

// Everything BCA requires on a transfer-va call must actually be on the wire.
// X-EXTERNAL-ID in particular was simply absent, which alone would have made
// every outbound call fail.
func TestClient_PaymentStatus_SendsMandatorySNAPHeaders(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"responseCode":"2002600","virtualAccountData":{"paymentFlagStatus":"00"}}`))
	}))
	defer server.Close()

	client := NewClient(outboundConfig(server.URL), &staticToken{token: "tok-123"})
	_, err := client.PaymentStatus(context.Background(), &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
	})
	require.NoError(t, err)

	assert.Equal(t, "95231", got.Get("CHANNEL-ID"))
	assert.Equal(t, "12345", got.Get("X-PARTNER-ID"))
	assert.NotEmpty(t, got.Get("X-TIMESTAMP"))
	assert.NotEmpty(t, got.Get("X-SIGNATURE"))
	assert.Equal(t, "Bearer tok-123", got.Get("Authorization"))

	externalID := got.Get("X-EXTERNAL-ID")
	require.NotEmpty(t, externalID, "X-EXTERNAL-ID is Mandatory on every transfer-va service")
	assert.LessOrEqual(t, len(externalID), 36)
	for _, r := range externalID {
		require.True(t, r >= '0' && r <= '9', "X-EXTERNAL-ID must be a Numeric String, got %q", externalID)
	}
}

// The accessToken is part of stringToSign, so a signature computed with an
// empty slot while a real token travels in the header would be rejected by the
// vendor. Recomputing the expected signature is the only way to pin that.
func TestClient_PaymentStatus_BindsTheAccessTokenIntoTheSignature(t *testing.T) {
	var gotSignature, gotTimestamp string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-SIGNATURE")
		gotTimestamp = r.Header.Get("X-TIMESTAMP")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"responseCode":"2002600","virtualAccountData":{"paymentFlagStatus":"00"}}`))
	}))
	defer server.Close()

	cfg := outboundConfig(server.URL)
	client := NewClient(cfg, &staticToken{token: "tok-abc"})
	_, err := client.PaymentStatus(context.Background(), &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
	})
	require.NoError(t, err)

	want := crypto.NewHMACSigner(cfg.ClientSecret, cfg.SignatureAlgorithm).Sign(
		crypto.BuildStringToSign(http.MethodPost, "/openapi/v2.0/transfer-va/status", "tok-abc",
			crypto.HashRequestBody(gotBody, crypto.BodyHashHex), gotTimestamp))

	assert.Equal(t, want, gotSignature)
}

// A vendor with no outbound credentials keeps the legacy empty-AccessToken
// convention and sends no Authorization header.
func TestClient_WithoutTokenProvider_UsesTheLegacyEmptySlot(t *testing.T) {
	var gotAuth, gotSignature, gotTimestamp string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotSignature = r.Header.Get("X-SIGNATURE")
		gotTimestamp = r.Header.Get("X-TIMESTAMP")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"responseCode":"2002600","virtualAccountData":{"paymentFlagStatus":"00"}}`))
	}))
	defer server.Close()

	cfg := outboundConfig(server.URL)
	client := NewClient(cfg, nil)
	_, err := client.PaymentStatus(context.Background(), &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
	})
	require.NoError(t, err)

	assert.Empty(t, gotAuth)
	want := crypto.NewHMACSigner(cfg.ClientSecret, cfg.SignatureAlgorithm).Sign(
		crypto.BuildStringToSign(http.MethodPost, "/openapi/v2.0/transfer-va/status", "",
			crypto.HashRequestBody(gotBody, crypto.BodyHashHex), gotTimestamp))
	assert.Equal(t, want, gotSignature)
}

// A token that cannot be obtained must fail the call outright rather than
// sending an unauthenticated request the vendor will reject less clearly.
func TestClient_TokenFailure_AbortsBeforeSending(t *testing.T) {
	var reached int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reached, 1)
	}))
	defer server.Close()

	client := NewClient(outboundConfig(server.URL), &staticToken{err: assertErr("no credentials")})
	_, err := client.PaymentStatus(context.Background(), &domain.VAStatusRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "obtain vendor access token")
	assert.Zero(t, atomic.LoadInt32(&reached), "no request may be sent without a token")
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// --- token provider ------------------------------------------------------

func TestTokenProvider_CachesUntilCloseToExpiry(t *testing.T) {
	var fetches int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetches, 1)
		_, _ = w.Write([]byte(`{"responseCode":"2007300","accessToken":"tok-1","expiresIn":"900"}`))
	}))
	defer server.Close()

	keyPath := writeTestPrivateKey(t)
	cfg := outboundConfig(server.URL)
	cfg.ClientID = "client-1"
	cfg.OutboundPrivateKeyPath = keyPath
	cfg.TokenEndpoint = "/openapi/v1.0/access-token/b2b"

	provider := NewTokenProvider(cfg)
	base := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return base }

	first, err := provider.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", first)

	// Well inside the lifetime: served from cache.
	provider.now = func() time.Time { return base.Add(10 * time.Minute) }
	second, err := provider.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", second)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetches), "a token still comfortably valid must not be refetched")

	// Inside the refresh margin: refetched rather than risking a token that
	// expires in flight.
	provider.now = func() time.Time { return base.Add(890 * time.Second) }
	_, err = provider.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&fetches))
}

func TestTokenProvider_SignsWithTheOutboundKeyAndSNAPStringToSign(t *testing.T) {
	var got http.Header
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"accessToken":"tok-x","expiresIn":"900"}`))
	}))
	defer server.Close()

	cfg := outboundConfig(server.URL)
	cfg.ClientID = "client-9"
	cfg.OutboundPrivateKeyPath = writeTestPrivateKey(t)
	cfg.TokenEndpoint = "/openapi/v1.0/access-token/b2b"

	_, err := NewTokenProvider(cfg).AccessToken(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "client-9", got.Get("X-CLIENT-KEY"))
	assert.NotEmpty(t, got.Get("X-TIMESTAMP"))
	assert.NotEmpty(t, got.Get("X-SIGNATURE"))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "client_credentials", parsed["grantType"])
}

// A vendor with no outbound credentials must fail with a message naming what
// is missing, not a generic transport error.
func TestTokenProvider_MissingCredentials_SaysWhich(t *testing.T) {
	cfg := outboundConfig("http://127.0.0.1:1")
	_, err := NewTokenProvider(cfg).AccessToken(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VENDOR_CLIENT_ID")
	assert.Contains(t, err.Error(), "VENDOR_PRIVATE_KEY_PATH")
}

// A missing or unreadable lifetime must not be read as "never expires".
func TestNormalizeExpiresIn(t *testing.T) {
	assert.Equal(t, 900, normalizeExpiresIn(nil))
	assert.Equal(t, 900, normalizeExpiresIn(json.RawMessage(`"not-a-number"`)))
	assert.Equal(t, 600, normalizeExpiresIn(json.RawMessage(`"600"`)))
	assert.Equal(t, 600, normalizeExpiresIn(json.RawMessage(`600`)))
	assert.Equal(t, 900, normalizeExpiresIn(json.RawMessage(`0`)))
}

// writeTestPrivateKey writes the repo's test client key to a temp file and
// returns its path.
func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	pem, err := findRepoFile("client_private.pem")
	require.NoError(t, err, "expected the repo's test RSA key to be readable")
	path := t.TempDir() + "/private.pem"
	require.NoError(t, os.WriteFile(path, pem, 0o600))
	return path
}

func findRepoFile(name string) ([]byte, error) {
	// The test binary runs in its package directory, so walk up to the module
	// root rather than hardcoding a relative depth.
	dir := "."
	for i := 0; i < 8; i++ {
		if data, err := os.ReadFile(dir + "/" + name); err == nil {
			return data, nil
		}
		dir += "/.."
	}
	return nil, os.ErrNotExist
}
