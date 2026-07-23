package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/labstack/echo/v4"
)

// AdminAuthMiddleware protects trust-anchor management endpoints (client
// onboarding) with a static API key. If apiKey is empty, the admin API is
// disabled entirely (fail closed) rather than left unauthenticated.
func AdminAuthMiddleware(apiKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if apiKey == "" {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"status":  "error",
					"message": "Admin API is disabled. Set ADMIN_API_KEY to enable it.",
				})
			}

			provided := c.Request().Header.Get("X-Admin-API-Key")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"status":  "error",
					"message": "Unauthorized. Invalid or missing X-Admin-API-Key header.",
				})
			}

			return next(c)
		}
	}
}
