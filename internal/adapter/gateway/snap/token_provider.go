package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
)

// tokenRefreshMargin is how long before expiry a cached token is discarded.
// SNAP tokens live 900s; refreshing a minute early costs one extra token
// request per 15 minutes and removes the race where a token that was valid
// when we checked has expired by the time the vendor validates it.
const tokenRefreshMargin = 60 * time.Second

// TokenProvider obtains an accessToken FROM a vendor, for calls this service
// originates (feature 014-vendor-status-reconciliation).
//
// This is the mirror image of the inbound token flow: there, a vendor signs
// "clientId|timestamp" with ITS private key and we verify with the public key
// we hold. Here we are the caller, so we sign with OUR private key and the
// vendor verifies with the public key it holds for us. The credential is
// therefore VendorConfig.OutboundPrivateKeyPath, never ClientSecret — mixing
// the two up is the kind of mistake a 401 will not explain.
type TokenProvider struct {
	config     *config.VendorConfig
	httpClient *http.Client
	rsaSigner  *crypto.RSASigner

	mu        sync.Mutex
	token     string
	expiresAt time.Time
	// now is injectable so the expiry logic can be tested without sleeping.
	now func() time.Time
}

// NewTokenProvider creates a token provider for one vendor.
func NewTokenProvider(cfg *config.VendorConfig) *TokenProvider {
	return &TokenProvider{
		config:     cfg,
		httpClient: &http.Client{Timeout: time.Duration(cfg.RequestTimeout) * time.Second},
		rsaSigner:  crypto.NewRSASigner(),
		now:        time.Now,
	}
}

// AccessToken returns a valid token, fetching one only when the cached token
// is absent or within tokenRefreshMargin of expiry.
func (p *TokenProvider) AccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && p.now().Add(tokenRefreshMargin).Before(p.expiresAt) {
		return p.token, nil
	}

	token, expiresIn, err := p.fetch(ctx)
	if err != nil {
		// The stale token is deliberately NOT returned as a fallback: a vendor
		// that rejects our token request will reject the token too, and
		// pretending otherwise turns one clear error into a confusing 401
		// further down.
		p.token, p.expiresAt = "", time.Time{}
		return "", err
	}

	p.token = token
	p.expiresAt = p.now().Add(time.Duration(expiresIn) * time.Second)
	return token, nil
}

type tokenResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	AccessToken     string `json:"accessToken"`
	// SNAP documents expiresIn as a string ("900"), but vendors have been seen
	// sending a bare number. Decoded loosely and normalised below rather than
	// failing the whole token fetch over a type.
	ExpiresIn json.RawMessage `json:"expiresIn"`
}

func (p *TokenProvider) fetch(ctx context.Context) (string, int, error) {
	if p.config.ClientID == "" || p.config.OutboundPrivateKeyPath == "" {
		return "", 0, fmt.Errorf("vendor %s/%s has no outbound credentials (VENDOR_CLIENT_ID / VENDOR_PRIVATE_KEY_PATH)", p.config.Vendor, p.config.Channel)
	}

	privateKey, err := os.ReadFile(p.config.OutboundPrivateKeyPath)
	if err != nil {
		return "", 0, fmt.Errorf("read outbound private key: %w", err)
	}

	// snapTimestamp, not time.RFC3339: the "Z" form Go emits for UTC is
	// signed into clientId|timestamp and BCA rebuilds it from the header.
	timestamp := snapTimestamp(p.now())
	// SNAP asymmetric signature for the B2B token endpoint: clientId|timestamp.
	stringToSign := p.config.ClientID + "|" + timestamp
	signature, err := p.rsaSigner.Sign(string(privateKey), stringToSign)
	if err != nil {
		return "", 0, fmt.Errorf("sign token request: %w", err)
	}

	endpoint := p.config.TokenEndpoint
	if endpoint == "" {
		endpoint = "/openapi/v1.0/access-token/b2b"
	}
	body := []byte(`{"grantType":"client_credentials","additionalInfo":{}}`)

	// Everything the vendor used to decide, minus the secret itself.
	//
	// A 401 on this endpoint has exactly three causes — unknown clientId,
	// a signature the vendor cannot verify, or a timestamp it considers
	// stale — and the vendor's own message distinguishes them only
	// partially. keyFingerprint is here because the hardest of the three to
	// diagnose is a correct clientId signed with the wrong key: the config
	// names a key PATH, which says nothing about whether the vendor holds
	// its public half. The fingerprint is comparable against the registered
	// public key, and carries no key material.
	// Debug, like every other outbound trace. A token failure stays visible
	// without it: the error is wrapped up through Client.doRequest into
	// va_reconcile_result, which logs at Error with this message inside it.
	slog.Debug("vendor_token_request",
		slog.String("vendor", p.config.Vendor),
		slog.String("channel", p.config.Channel),
		slog.String("url", p.config.BaseURL+endpoint),
		slog.String("clientId", p.config.ClientID),
		slog.String("timestamp", timestamp),
		slog.String("stringToSign", stringToSign),
		slog.String("keyPath", p.config.OutboundPrivateKeyPath),
		slog.String("keyFingerprint", crypto.PublicKeyFingerprint(string(privateKey))),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CLIENT-KEY", p.config.ClientID)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)

	// The full token request, headers and a runnable curl included.
	//
	// Nothing here is secret: X-SIGNATURE is an RSA signature over
	// clientId|timestamp, which proves possession of the private key without
	// revealing it, and there is no bearer token yet — this is the call that
	// obtains one. So unlike the API requests, this command is complete and
	// can be run as-is, which is what makes it possible to check a 401
	// against the vendor by hand.
	// Guarded for the same reason as vendor_api_request_full: slog evaluates
	// its arguments before deciding to discard them, so without this a curl
	// command and a header slice are rendered on every token fetch only to be
	// thrown away.
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.DebugContext(ctx, "vendor_token_request_full",
			slog.String("vendor", p.config.Vendor),
			slog.String("channel", p.config.Channel),
			slog.String("method", http.MethodPost),
			slog.String("url", p.config.BaseURL+endpoint),
			slog.Any("headers", loggableHeaders(req.Header)),
			slog.String("body", string(body)),
			slog.String("curl", curlCommand(http.MethodPost, p.config.BaseURL+endpoint, req.Header, body)),
		)
	}

	started := p.now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		// Debug like the other vendor_token_response sites — one event name at
		// one level, or a reader filtering for it gets a partial view.
		slog.Debug("vendor_token_response",
			slog.String("vendor", p.config.Vendor),
			slog.String("channel", p.config.Channel),
			slog.String("url", p.config.BaseURL+endpoint),
			slog.String("clientId", p.config.ClientID),
			slog.Int64("durationMs", p.now().Sub(started).Milliseconds()),
			slog.String("error", err.Error()),
		)
		return "", 0, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read token response: %w", err)
	}

	// The body is logged verbatim. A token response carries no accessToken on
	// the failure paths — which are the ones worth logging — and on success
	// the token is redacted below rather than the whole body being dropped,
	// because the vendor's responseCode/responseMessage is the only place its
	// reasoning is stated.
	durationMs := p.now().Sub(started).Milliseconds()
	logBody := string(respBody)
	if resp.StatusCode < 400 {
		logBody = redactAccessToken(respBody)
	}
	attrs := []any{
		slog.String("vendor", p.config.Vendor),
		slog.String("channel", p.config.Channel),
		slog.String("url", p.config.BaseURL+endpoint),
		slog.String("clientId", p.config.ClientID),
		slog.Int("status", resp.StatusCode),
		slog.Int64("durationMs", durationMs),
		slog.String("body", logBody),
	}
	// Debug at every status, rejections included. This is the vendor's raw
	// answer; the caller decides what it means. A 401 here fails the whole
	// outbound call, and that failure is reported at Error by the usecase with
	// the responseMessage ("Unauthorized. [Unknown client]") carried along —
	// so nothing actionable is lost when this is off.
	slog.Debug("vendor_token_response", attrs...)
	if resp.StatusCode >= 400 {
		return "", 0, fmt.Errorf("token request rejected (status %d): %s", resp.StatusCode, string(respBody))
	}

	var parsed tokenResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", 0, fmt.Errorf("parse token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", 0, fmt.Errorf("token response carried no accessToken (responseCode %s %s)", parsed.ResponseCode, parsed.ResponseMessage)
	}

	return parsed.AccessToken, normalizeExpiresIn(parsed.ExpiresIn), nil
}

// normalizeExpiresIn accepts "900" and 900 alike, falling back to SNAP's
// documented 900s when the field is absent or unreadable. A missing lifetime
// must not mean "never expires".
func normalizeExpiresIn(raw json.RawMessage) int {
	const defaultExpiresIn = 900
	if len(raw) == 0 {
		return defaultExpiresIn
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if v, convErr := strconv.Atoi(asString); convErr == nil && v > 0 {
			return v
		}
		return defaultExpiresIn
	}
	var asNumber int
	if err := json.Unmarshal(raw, &asNumber); err == nil && asNumber > 0 {
		return asNumber
	}
	return defaultExpiresIn
}

// redactAccessToken replaces the accessToken value in a successful token
// response, so the surrounding responseCode/expiresIn stay readable in the
// log without the bearer token itself being written to it. Anything that does
// not parse is reported as such rather than logged raw — an unparseable body
// on a 2xx is itself the interesting fact.
func redactAccessToken(body []byte) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return "<unparseable token response>"
	}
	if _, ok := fields["accessToken"]; ok {
		fields["accessToken"] = json.RawMessage(`"<redacted>"`)
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return "<unparseable token response>"
	}
	return string(out)
}
