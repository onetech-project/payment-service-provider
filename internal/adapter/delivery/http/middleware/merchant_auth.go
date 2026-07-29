package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/labstack/echo/v4"
)

// MerchantAuthMiddleware requires a valid, unexpired accessToken (bearer
// JWT issued by POST /openapi/v1.0/access-token/b2b) AND a valid HMAC-SHA512
// X-SIGNATURE (feature 010-merchant-hmac-signature) on every request. It
// applies only to merchant-facing routes (create-va/list/delete-va) — the
// vendor-facing routes use SNAPAuthMiddleware instead (feature
// 009-transfer-va-auth). Both checks are unconditional — no configuration
// exists to enable/disable either one.
func MerchantAuthMiddleware(jwtIssuer domain.JWTIssuer, clientRepo domain.ClientRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Missing or invalid Authorization header]",
				})
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwtIssuer.ValidateToken(token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid or expired access token]",
				})
			}

			// Fail closed: a client with no provisioned/active shared secret
			// is rejected outright, regardless of what signature it sends.
			ctx := c.Request().Context()
			secret, err := clientRepo.GetActiveClientSecret(ctx, claims.ClientID)
			if err != nil || secret == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [No signing secret provisioned for this client]",
				})
			}

			// Timestamp freshness — same ±5 minute tolerance as
			// SNAPAuthMiddleware (feature 009).
			timestamp := c.Request().Header.Get("X-TIMESTAMP")
			parsedTimestamp, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid or missing X-TIMESTAMP]",
				})
			}
			if time.Since(parsedTimestamp) > 5*time.Minute || time.Until(parsedTimestamp) > 5*time.Minute {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Timestamp skew exceeds 5 minutes]",
				})
			}

			// HMAC signature verification. Unlike SNAPAuthMiddleware's
			// vendor-side convention, the AccessToken component of
			// stringToSign is the REAL bearer token here — it is genuinely
			// transmitted via the Authorization header on merchant
			// endpoints, unlike vendor-facing endpoints where no header
			// ever carries it.
			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Unable to read request body]",
				})
			}
			c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			bodyHash := crypto.HashSHA256Hex(string(bodyBytes))
			stringToSign := crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, token, bodyHash, timestamp)
			signer := crypto.NewHMACSigner(secret, "HMAC-SHA512")
			if !signer.Verify(stringToSign, c.Request().Header.Get("X-SIGNATURE")) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid signature]",
				})
			}

			return next(c)
		}
	}
}
