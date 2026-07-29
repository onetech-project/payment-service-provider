package middleware

import (
	"net/http"
	"strings"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// MerchantAuthMiddleware requires a valid, unexpired accessToken (bearer
// JWT issued by POST /openapi/v1.0/access-token/b2b) on every request. It
// applies only to merchant-facing routes (create-va/list/delete-va) — the
// vendor-facing routes use SNAPAuthMiddleware instead (feature
// 009-transfer-va-auth).
func MerchantAuthMiddleware(jwtIssuer domain.JWTIssuer) echo.MiddlewareFunc {
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
			if _, err := jwtIssuer.ValidateToken(token); err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"responseCode":    "4010000",
					"responseMessage": "Unauthorized. [Invalid or expired access token]",
				})
			}

			return next(c)
		}
	}
}
