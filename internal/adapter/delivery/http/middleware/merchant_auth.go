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

// merchantSignatureAlgorithm is fixed, unlike the vendor side's per-vendor
// setting: merchants are onboarded by us, so there is no counterparty whose
// existing integration could pin anything but SNAP's HMAC-SHA512.
const merchantSignatureAlgorithm = "HMAC-SHA512"

// MerchantAuthMiddleware requires a valid, unexpired accessToken (bearer
// JWT issued by POST /openapi/v1.0/access-token/b2b) AND a valid HMAC-SHA512
// X-SIGNATURE (feature 010-merchant-hmac-signature) on every request. It
// applies only to merchant-facing routes (create-va/list/delete-va) — the
// vendor-facing routes use SNAPAuthMiddleware instead (feature
// 009-transfer-va-auth). Both the bearer token and signature checks are
// always enforced; only the ±5 minute X-TIMESTAMP freshness check can be
// disabled, via skipSkewCheck (intended for APP_ENV=dev/uat only).
//
// acceptLegacyBodyHash keeps the superseded base64 body-hash conventions
// working during the cutover to the spec's hex — see verifyMerchantSignature.
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

// verifyMerchantSignature checks X-SIGNATURE against the merchant
// stringToSign, derived by the same symmetricSignature the vendor side uses.
//
// The canonical RequestBody component is Lowercase(HexEncode(SHA-256(
// MinifyJson(body)))) — what SNAP specifies and what the vendor side has
// always sent. This side reached it in two steps, each found the expensive
// way: it hashed the raw body while the vendor side minified, and then it
// encoded the digest as base64 while the vendor side used hex. The same
// request body therefore had two different valid signatures depending on
// which endpoint received it, with nothing in the 401 to say so.
//
// acceptLegacyBodyHash keeps both superseded conventions working during the
// cutover: base64 of the minified body, and base64 of the raw body. Neither
// is a form a merchant can be moved off at deploy time without warning, so
// operations turns them off (MERCHANT_LEGACY_BODY_HASH=false) once every
// merchant signs the hex form. Note the raw-body digest only differs for
// whitespace-bearing JSON — for a compact body MinifyJson is a no-op — so
// bodyHashCandidates drops it as a duplicate rather than verifying twice.
func verifyMerchantSignature(c echo.Context, secret, token, timestamp string, bodyBytes []byte, acceptLegacyBodyHash bool) bool {
	signature := signatureFromRequest(c, token, timestamp, bodyBytes)
	signature.Secret = secret
	signature.Algorithm = merchantSignatureAlgorithm
	signature.BodyHashEncodings = []string{crypto.BodyHashHex}

	if acceptLegacyBodyHash {
		signature.BodyHashEncodings = append(signature.BodyHashEncodings, crypto.BodyHashBase64)
		signature.ExtraBodyHashes = []string{crypto.HashSHA256Base64(string(bodyBytes))}
	}

	return signature.verify(c.Request().Header.Get("X-SIGNATURE"))
}
