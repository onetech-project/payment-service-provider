package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestAdminAuthMiddleware(t *testing.T) {
	e := echo.New()

	next := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	t.Run("Disabled when apiKey empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/clients", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := AdminAuthMiddleware("")(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("Missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/clients", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := AdminAuthMiddleware("secret-key")(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Invalid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/clients", nil)
		req.Header.Set("X-Admin-API-Key", "wrong-key")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := AdminAuthMiddleware("secret-key")(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Valid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/clients", nil)
		req.Header.Set("X-Admin-API-Key", "secret-key")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := AdminAuthMiddleware("secret-key")(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
