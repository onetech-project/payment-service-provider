package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestTelemetryMiddleware(t *testing.T) {
	e := echo.New()

	next := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	t.Run("Generates correlation ID when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := TelemetryMiddleware()(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("X-Correlation-ID"))
	})

	t.Run("Preserves supplied correlation ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Correlation-ID", "my-correlation-id")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := TelemetryMiddleware()(next)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, "my-correlation-id", rec.Header().Get("X-Correlation-ID"))
	})

	t.Run("Propagates handler error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		failing := func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTeapot, "teapot")
		}

		handler := TelemetryMiddleware()(failing)
		err := handler(c)
		assert.Error(t, err)
	})
}
