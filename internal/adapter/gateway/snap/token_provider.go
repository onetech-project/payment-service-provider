package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	timestamp := p.now().Format(time.RFC3339)
	// SNAP asymmetric signature for the B2B token endpoint: clientId|timestamp.
	signature, err := p.rsaSigner.Sign(string(privateKey), p.config.ClientID+"|"+timestamp)
	if err != nil {
		return "", 0, fmt.Errorf("sign token request: %w", err)
	}

	endpoint := p.config.TokenEndpoint
	if endpoint == "" {
		endpoint = "/openapi/v1.0/access-token/b2b"
	}
	body := []byte(`{"grantType":"client_credentials","additionalInfo":{}}`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CLIENT-KEY", p.config.ClientID)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read token response: %w", err)
	}
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
