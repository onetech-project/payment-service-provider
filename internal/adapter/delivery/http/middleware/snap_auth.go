package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/labstack/echo/v4"
)

// SNAPAuthMiddleware validates SNAP authentication headers based on vendor config
func SNAPAuthMiddleware(vendorConfig *config.VendorConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get required headers from config
			requiredHeaders := vendorConfig.RequiredHeaders
			if len(requiredHeaders) == 0 {
				// Default SNAP required headers per ASPI spec for transfer-va
				// endpoints: X-TIMESTAMP, X-SIGNATURE, X-PARTNER-ID, X-EXTERNAL-ID.
				// X-CLIENT-KEY is NOT part of this list — it is only used on the
				// access-token endpoint, never on transaction endpoints.
				requiredHeaders = []string{"X-TIMESTAMP", "X-SIGNATURE"}
			}

			// Validate required headers
			for _, header := range requiredHeaders {
				value := c.Request().Header.Get(header)
				if value == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"responseCode":    "4010000",
						"responseMessage": "Unauthorized. [Missing required header: " + header + "]",
					})
				}
			}

			// Validate timestamp format (ISO 8601)
			timestamp := c.Request().Header.Get("X-TIMESTAMP")
			if !isValidISO8601(timestamp) {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"responseCode":    "4000001",
					"responseMessage": "Invalid Field Format [X-TIMESTAMP]",
				})
			}

			// Validate CHANNEL-ID if required
			if vendorConfig.ChannelID != "" {
				channelID := c.Request().Header.Get("CHANNEL-ID")
				if channelID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{
						"responseCode":    "4000002",
						"responseMessage": "Invalid Mandatory Field [CHANNEL-ID]",
					})
				}
			}

			// Validate X-PARTNER-ID if required
			if vendorConfig.PartnerID != "" {
				partnerID := c.Request().Header.Get("X-PARTNER-ID")
				if partnerID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{
						"responseCode":    "4000002",
						"responseMessage": "Invalid Mandatory Field [X-PARTNER-ID]",
					})
				}
			}

			// Validate X-EXTERNAL-ID for non-GET requests
			if c.Request().Method != http.MethodGet {
				externalID := c.Request().Header.Get("X-EXTERNAL-ID")
				if externalID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{
						"responseCode":    "4000002",
						"responseMessage": "Invalid Mandatory Field [X-EXTERNAL-ID]",
					})
				}
			}

			// Timestamp freshness (feature 009-transfer-va-auth, FR-003):
			// reject requests whose X-TIMESTAMP is stale or too far in the
			// future, independent of signature validity. Same ±5 minute
			// tolerance already used for the B2B token endpoint
			// (internal/usecase/token_usecase.go).
			parsedTimestamp, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid X-TIMESTAMP]",
				})
			}
			if time.Since(parsedTimestamp) > 5*time.Minute || time.Until(parsedTimestamp) > 5*time.Minute {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Timestamp skew exceeds 5 minutes]",
				})
			}

			// HMAC signature verification (feature 009-transfer-va-auth,
			// FR-001/FR-002/FR-004): recompute the expected signature from
			// the raw request body using the vendor/channel's shared secret,
			// and compare against X-SIGNATURE. Fails closed (rejects) if the
			// shared secret is unconfigured. The request body is read here
			// and immediately re-buffered so the downstream handler can
			// still bind it (same pattern as IdempotencyMiddleware).
			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Unable to read request body]",
				})
			}
			c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			bodyHash := crypto.HashSHA256Hex(string(bodyBytes))
			// The accessToken component of stringToSign is not carried by
			// any header on these symmetric-signed transactional endpoints
			// (per the existing client tooling in scripts/vendor-*.sh, which
			// never sends an Authorization header here) — it is always the
			// empty string on the server side to match what was actually
			// signed.
			stringToSign := crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, "", bodyHash, timestamp)
			signer := crypto.NewHMACSigner(vendorConfig.ClientSecret, vendorConfig.SignatureAlgorithm)
			if vendorConfig.ClientSecret == "" || !signer.Verify(stringToSign, c.Request().Header.Get("X-SIGNATURE")) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid signature]",
				})
			}

			return next(c)
		}
	}
}

// isValidISO8601 validates ISO 8601 timestamp format
func isValidISO8601(s string) bool {
	// Basic validation: must contain 'T' and be at least 19 chars
	if len(s) < 19 {
		return false
	}
	if !strings.Contains(s, "T") {
		return false
	}
	// Check date part (YYYY-MM-DD)
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	// Check time part (HH:MM:SS)
	if s[13] != ':' || s[16] != ':' {
		return false
	}
	return true
}
