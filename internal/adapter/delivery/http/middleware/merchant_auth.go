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
// 009-transfer-va-auth). Both the bearer token and signature checks are
// always enforced; only the ±5 minute X-TIMESTAMP freshness check can be
// disabled, via skipSkewCheck (intended for APP_ENV=dev/uat only).
//
// acceptLegacyBodyHash keeps pre-minification merchant signatures working
// during the cutover — see verifyMerchantSignature.
func MerchantAuthMiddleware(jwtIssuer domain.JWTIssuer, clientRepo domain.ClientRepository, skipSkewCheck, acceptLegacyBodyHash bool) echo.MiddlewareFunc {
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
			// SNAPAuthMiddleware (feature 009), skippable via skipSkewCheck.
			timestamp := c.Request().Header.Get("X-TIMESTAMP")
			parsedTimestamp, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid or missing X-TIMESTAMP]",
				})
			}
			if !skipSkewCheck && (time.Since(parsedTimestamp) > 5*time.Minute || time.Until(parsedTimestamp) > 5*time.Minute) {
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

			if !verifyMerchantSignature(c, secret, token, timestamp, bodyBytes, acceptLegacyBodyHash) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid signature]",
				})
			}

			return next(c)
		}
	}
}

// verifyMerchantSignature checks X-SIGNATURE against the merchant stringToSign.
//
// The RequestBody component is base64(SHA-256(MinifyJson(body))) — the same
// minify-then-hash rule SNAPAuthMiddleware applies on the vendor side, and the
// one SNAP specifies. The two middlewares previously disagreed: the vendor
// side minified while this one hashed the raw bytes, so the identical request
// body produced two different valid signatures depending on which endpoint it
// was sent to. That is not a difference anyone can discover from a 401, and it
// cost real debugging time.
//
// acceptLegacyBodyHash additionally accepts the old raw-body digest. It exists
// because the change is only observable for merchants that send
// whitespace-bearing JSON — for compact bodies MinifyJson is a no-op and the
// two digests are identical — so switching outright would have broken exactly
// those merchants, silently, at deploy time. Operations turns it off
// (MERCHANT_LEGACY_BODY_HASH=false) once every merchant has migrated.
func verifyMerchantSignature(c echo.Context, secret, token, timestamp string, bodyBytes []byte, acceptLegacyBodyHash bool) bool {
	signer := crypto.NewHMACSigner(secret, "HMAC-SHA512")
	provided := c.Request().Header.Get("X-SIGNATURE")

	minified := crypto.HashRequestBody(bodyBytes, crypto.BodyHashBase64)
	if signer.Verify(crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, token, minified, timestamp), provided) {
		return true
	}
	if !acceptLegacyBodyHash {
		return false
	}

	legacy := crypto.HashSHA256Base64(string(bodyBytes))
	// Identical digests mean the body was already compact, so the legacy path
	// adds nothing and the signature is simply wrong. Skipping the second
	// Verify keeps that the single, unambiguous outcome.
	if legacy == minified {
		return false
	}
	return signer.Verify(crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, token, legacy, timestamp), provided)
}
