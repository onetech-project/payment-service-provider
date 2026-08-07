package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
)

// Client implements domain.VAGateway using generic SNAP configuration
type Client struct {
	config     *config.VendorConfig
	hmacSigner *crypto.HMACSigner
	httpClient *http.Client
	// tokens supplies the accessToken bound into stringToSign. Nil for a
	// vendor with no outbound credentials, in which case the legacy
	// empty-AccessToken convention applies.
	tokens domain.VendorTokenProvider
	// externalID generates the per-request X-EXTERNAL-ID. Injectable so tests
	// can assert on a fixed value.
	externalID func() string
}

// NewClient creates a new SNAP API client using vendor config.
//
// tokens may be nil: a vendor that has not been given outbound credentials
// signs with the empty-AccessToken convention, the same way legacy vendors
// call in. Pass a provider for any vendor whose API requires a bearer token —
// BCA's does.
func NewClient(cfg *config.VendorConfig, tokens domain.VendorTokenProvider) *Client {
	return &Client{
		config:     cfg,
		hmacSigner: crypto.NewHMACSigner(cfg.ClientSecret, cfg.SignatureAlgorithm),
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.RequestTimeout) * time.Second,
		},
		tokens:     tokens,
		externalID: newExternalID,
	}
}

// newExternalID builds the "Numeric String ... unique in the same day" that
// SNAP requires of X-EXTERNAL-ID. Nanosecond time plus a random tail: the
// timestamp alone collides when two calls land in the same nanosecond bucket,
// which a sweep reconciling a batch back-to-back can genuinely do.
func newExternalID() string {
	return fmt.Sprintf("%d%04d", time.Now().UnixNano(), rand.IntN(10000))
}

// Inquiry sends an inquiry request to SNAP VA API
func (c *Client) Inquiry(ctx context.Context, req *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	endpoint := c.config.APIEndpoints["INQUIRY"]
	if endpoint == "" {
		endpoint = "/transfer-va/inquiry"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inquiry request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}

	var vaResp domain.VAInquiryResponse
	if err := json.Unmarshal(resp, &vaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inquiry response: %w", err)
	}

	return &vaResp, nil
}

// PaymentStatus sends a status inquiry request to SNAP VA API
func (c *Client) PaymentStatus(ctx context.Context, req *domain.VAStatusRequest) (*domain.VAStatusResponse, error) {
	endpoint := c.config.APIEndpoints["STATUS"]
	if endpoint == "" {
		endpoint = "/transfer-va/status"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal status request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}

	var vaResp domain.VAStatusResponse
	if err := json.Unmarshal(resp, &vaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status response: %w", err)
	}

	return &vaResp, nil
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body []byte) ([]byte, error) {
	url := c.config.BaseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// The accessToken is bound into stringToSign, so it must be resolved
	// BEFORE signing. A vendor with no outbound credentials keeps the legacy
	// empty-AccessToken convention.
	accessToken := ""
	if c.tokens != nil {
		accessToken, err = c.tokens.AccessToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("obtain vendor access token: %w", err)
		}
	}

	// Generate timestamp
	timestamp := time.Now().Format(time.RFC3339)

	// Hash the request body per BCA's Signature Symmetric spec:
	// Lowercase(HexEncode(SHA-256(MinifyJson(RequestBody)))). The encoding
	// comes from this vendor's config, so an outbound call is signed the same
	// way the vendor verifies it.
	bodyHash := crypto.HashRequestBody(body, c.config.BodyHashEncoding)

	stringToSign := crypto.BuildStringToSign(method, endpoint, accessToken, bodyHash, timestamp)
	signature := c.hmacSigner.Sign(stringToSign)

	// Set headers from vendor config
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CHANNEL-ID", c.config.ChannelID)
	req.Header.Set("X-PARTNER-ID", c.config.PartnerID)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	// X-EXTERNAL-ID is Mandatory on every transfer-va service and was simply
	// absent here: BCA rejects the call outright without it, so this client
	// could never have completed a request against BCA.
	req.Header.Set("X-EXTERNAL-ID", c.externalID())
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if c.config.Origin != "" {
		req.Header.Set("ORIGIN", c.config.Origin)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// A 4xx from a SNAP service still carries a meaningful body — 4042601
	// "Transaction Not Found" is a real answer to a status inquiry, not a
	// transport failure. Discarding it as an error string threw away the
	// responseCode the caller needs to act on, so the body is returned and the
	// caller decides. Only 5xx is treated as "no answer".
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("vendor API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
