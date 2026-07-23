package middleware

import (
	"net/http"
	"strings"

	"backbone-new/internal/infrastructure/config"

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
