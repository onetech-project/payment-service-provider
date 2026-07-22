package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

// NewClient creates a new SNAP API client using vendor config
func NewClient(cfg *config.VendorConfig) *Client {
	return &Client{
		config:     cfg,
		hmacSigner: crypto.NewHMACSigner(cfg.ClientSecret, cfg.SignatureAlgorithm),
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.RequestTimeout) * time.Second,
		},
	}
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

	// Generate timestamp
	timestamp := time.Now().Format(time.RFC3339)

	// Hash request body
	bodyHash := crypto.HashSHA256Hex(string(body))

	// Build string to sign
	stringToSign := crypto.BuildStringToSign(method, endpoint, "", bodyHash, timestamp)

	// Sign with HMAC
	signature := c.hmacSigner.Sign(stringToSign)

	// Set headers from vendor config
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CHANNEL-ID", c.config.ChannelID)
	req.Header.Set("X-PARTNER-ID", c.config.PartnerID)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
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

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
