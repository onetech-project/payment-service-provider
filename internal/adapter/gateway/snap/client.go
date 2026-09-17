package snap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
)

// Fallback endpoint paths for a vendor whose config names none, taken from
// Developer API BCA's own curl samples.
//
// They carry the /openapi/<version> prefix on purpose. The bare
// "/transfer-va/..." forms these replace could not work against any SNAP
// vendor: the prefix is part of the relative URL, so dropping it produced both
// a 404 and — since the same string is the RelativeUrl component of
// stringToSign — a signature computed over a path that was never requested,
// which surfaces as an unexplained 401 rather than a routing error.
//
// Status is v2.0 while inquiry is v1.0. That asymmetry is BCA's, not a typo:
// service 26 runs partner→BCA and is versioned separately from the two
// services BCA calls on us.
const (
	defaultInquiryEndpoint = "/openapi/v1.0/transfer-va/inquiry"
	defaultStatusEndpoint  = "/openapi/v2.0/transfer-va/status"
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
// SNAP requires of X-EXTERNAL-ID.
//
// Shaped as yyyyMMddHHmmss + a 6-digit random tail — 20 digits, matching the
// form BCA's own samples use and scripts/bca-payment-status.sh sends
// successfully. The previous UnixNano()+4 digits produced 23 digits, and BCA
// answered those with a 504 Timeout rather than a field-format error, which
// is why this was not obvious from the response: the gateway never says the
// header is too long, it just stops answering.
//
// The random tail is what keeps it unique: second resolution alone collides
// when a sweep reconciles a batch back-to-back, and a collision inside the
// same day earns 4092600 Conflict.
func newExternalID() string {
	return time.Now().Format("20060102150405") + fmt.Sprintf("%06d", rand.IntN(1000000))
}

// outboundPartnerID is the X-PARTNER-ID sent to the vendor: its Company Code
// VA, falling back to the inbound PartnerID for a vendor where the two
// coincide (and so that a config written before the two were separated keeps
// behaving as it did).
func (c *Client) outboundPartnerID() string {
	if c.config.OutboundPartnerID != "" {
		return c.config.OutboundPartnerID
	}
	return c.config.PartnerID
}

// Inquiry sends an inquiry request to SNAP VA API
func (c *Client) Inquiry(ctx context.Context, req *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	endpoint := c.config.APIEndpoints["INQUIRY"]
	if endpoint == "" {
		endpoint = defaultInquiryEndpoint
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
		endpoint = defaultStatusEndpoint
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

	// X-EXTERNAL-ID is Mandatory on every transfer-va service and was simply
	// absent here: BCA rejects the call outright without it, so this client
	// could never have completed a request against BCA.
	//
	// Generated up here, before anything can fail, so the request log below
	// and the header carry the same value — it is what correlates the two
	// halves of one call in Loki.
	externalID := c.externalID()
	req.Header.Set("X-EXTERNAL-ID", externalID)

	// Logged before ANY step that can abort the call, so the attempt and its
	// body are on record even when nothing reaches the vendor.
	//
	// This used to sit further down, after the token fetch, which meant the
	// failure it was most needed for — a 401 on the token, so no request is
	// ever sent — produced no request log at all. The signature inputs are
	// added to the response log instead, since they only exist once signing
	// has happened.
	//
	// Debug: this is the full outbound payload on every call, which is more
	// than a healthy deployment needs in Loki. Failures stay visible without
	// it — the usecase logs va_reconcile_result at Error with the vendor's own
	// message — so turning it on is for diagnosing, not monitoring.
	slog.Debug("vendor_api_request",
		slog.String("vendor", c.config.Vendor),
		slog.String("channel", c.config.Channel),
		slog.String("method", method),
		slog.String("url", url),
		slog.String("endpoint", endpoint),
		slog.String("partnerId", c.outboundPartnerID()),
		slog.String("channelId", c.config.ChannelID),
		slog.String("externalId", externalID),
		slog.String("body", string(body)),
	)

	// The accessToken is bound into stringToSign, so it must be resolved
	// BEFORE signing. A vendor with no outbound credentials keeps the legacy
	// empty-AccessToken convention.
	accessToken := ""
	if c.tokens != nil {
		accessToken, err = c.tokens.AccessToken(ctx)
		if err != nil {
			// Reported as a response so every vendor_api_request has a
			// matching vendor_api_response, even though no request was sent:
			// a request logged with nothing after it is indistinguishable
			// from a call still in flight.
			//
			// Debug like the other vendor_api_response sites — one event name
			// at one level, or a reader filtering for it gets a partial view.
			// The token failure itself is logged at Error by the token
			// provider, so this being off loses nothing.
			slog.Debug("vendor_api_response",
				slog.String("vendor", c.config.Vendor),
				slog.String("channel", c.config.Channel),
				slog.String("method", method),
				slog.String("url", url),
				slog.String("externalId", externalID),
				slog.String("stage", "access_token"),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("obtain vendor access token: %w", err)
		}
	}

	// Generate timestamp. snapTimestamp, not time.RFC3339: BCA rejects the
	// "Z" form Go emits for UTC, and the value is part of stringToSign.
	timestamp := snapTimestamp(time.Now())

	// Hash the request body per BCA's Signature Symmetric spec:
	// Lowercase(HexEncode(SHA-256(MinifyJson(RequestBody)))). The encoding
	// comes from this vendor's config, so an outbound call is signed the same
	// way the vendor verifies it.
	bodyHash := crypto.HashRequestBody(body, c.config.BodyHashEncoding)

	stringToSign := crypto.BuildStringToSign(method, endpoint, accessToken, bodyHash, timestamp)
	signature := c.hmacSigner.Sign(stringToSign)

	// The same string with the AccessToken component blanked, for logging.
	// stringToSign embeds the live bearer token, so logging it verbatim would
	// publish a working credential to Loki — where it is queryable, retained,
	// and visible to anyone with Grafana access. The other four components
	// are what a signature mismatch is actually diagnosed from, and none of
	// them is secret.
	loggableStringToSign := crypto.BuildStringToSign(method, endpoint, "<redacted>", bodyHash, timestamp)

	// Set headers from vendor config
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CHANNEL-ID", c.config.ChannelID)
	req.Header.Set("X-PARTNER-ID", c.outboundPartnerID())
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	// X-ORIGIN, not ORIGIN.
	//
	// Two things were wrong with the bare name. It is not what the spec calls
	// this header — ASPI names it X-ORIGIN (aspi-open-api-va.yaml:46), and BCA
	// documents ORIGIN only for the inbound direction, where it is what BCA
	// sends US. And "Origin" is a reserved CORS request header: to the gateway
	// in front of BCA's API it does not read as a vendor field at all, it reads
	// as a cross-origin browser request from a host that is not allowed to make
	// one, which is rejected before any SNAP handler sees it.
	//
	// That rejection came back as 5042600 "Timeout" in 251ms — not a timeout,
	// and not a field error naming the header, which is why this survived three
	// rounds of looking at the signature instead.
	if c.config.Origin != "" {
		req.Header.Set("X-ORIGIN", c.config.Origin)
	}

	// The fully-formed request, logged once the headers exist.
	//
	// Separate from vendor_api_request above rather than replacing it: that
	// one is emitted before the token fetch, so it survives a call that never
	// gets sent. This one can only exist after signing, and carries what the
	// vendor will actually receive.
	//
	// curl is the field that earns its place. Reproducing a SNAP call by hand
	// goes wrong in ways the vendor answers with an unexplained 504 —
	// re-typed JSON changes the body hash, a forgotten X-EXTERNAL-ID is
	// rejected on unrelated grounds — so a command copied straight out of the
	// log is what makes "the service fails, my curl works" a comparison of
	// two identical things. The bearer token is a placeholder; everything
	// else is verbatim.
	// Guarded rather than left to the handler's own level filter: slog
	// evaluates its arguments before deciding to discard them, so without this
	// every call would render a curl command and a header slice only to throw
	// both away. Enabled() is the documented way to skip that.
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		slog.DebugContext(ctx, "vendor_api_request_full",
			slog.String("vendor", c.config.Vendor),
			slog.String("channel", c.config.Channel),
			slog.String("method", method),
			slog.String("url", url),
			slog.String("externalId", externalID),
			slog.Any("headers", loggableHeaders(req.Header)),
			slog.String("stringToSign", loggableStringToSign),
			slog.String("bodyHash", bodyHash),
			slog.String("bodyHashEncoding", c.config.BodyHashEncoding),
			slog.String("body", string(body)),
			slog.String("curl", curlCommand(method, url, req.Header, body)),
		)
	}

	// Execute request
	started := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Debug for the same consistency reason as above. A transport failure
		// still surfaces: the error is returned to the caller, and
		// va_reconcile_result logs it at Error with this message inside it.
		slog.Debug("vendor_api_response",
			slog.String("vendor", c.config.Vendor),
			slog.String("channel", c.config.Channel),
			slog.String("method", method),
			slog.String("url", url),
			slog.String("externalId", externalID),
			slog.String("stage", "transport"),
			slog.Int64("durationMs", time.Since(started).Milliseconds()),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Logged whole: a SNAP response body is small, and its responseCode plus
	// the status-specific fields (paymentFlagStatus, paidAmount) are exactly
	// what an operator reconstructs an outcome from. A 4xx is Warn, not
	// Error — 4042601 "Transaction Not Found" is an ordinary answer here,
	// not a fault.
	respAttrs := []any{
		slog.String("vendor", c.config.Vendor),
		slog.String("channel", c.config.Channel),
		slog.String("method", method),
		slog.String("url", url),
		slog.String("externalId", externalID),
		// The signature inputs ride on the response rather than the request:
		// they do not exist until signing has run, and they are only ever
		// interesting once the vendor has said what it made of them.
		slog.String("timestamp", timestamp),
		slog.String("stringToSign", loggableStringToSign),
		slog.Bool("hasAccessToken", accessToken != ""),
		slog.Int("status", resp.StatusCode),
		slog.Int64("durationMs", time.Since(started).Milliseconds()),
		slog.String("body", string(respBody)),
	}
	// Debug at every status, including 5xx.
	//
	// It reads oddly to log a failure at Debug, but this is the vendor's raw
	// answer, not the outcome: whoever called the gateway decides what the
	// answer means and logs that. ReconciliationUsecase already emits
	// va_reconcile_result at Error carrying the vendor's own message, so a
	// failure is not lost when this is off — and keeping 5xx at Error here
	// would have split one call across two levels, which is worse to read than
	// either.
	slog.Debug("vendor_api_response", respAttrs...)

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
