package middleware

import (
	"log/slog"
	"os"
	"time"

	"backbone-new/internal/infrastructure/telemetry"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var Logger *slog.Logger

func init() {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func TelemetryMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			// Correlation ID
			correlationID := req.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
			}
			c.Response().Header().Set("X-Correlation-ID", correlationID)

			// OTel Trace Span
			tracer := telemetry.Tracer
			if tracer != nil {
				var span trace.Span
				ctx, span = tracer.Start(ctx, req.Method+" "+c.Path(),
					trace.WithAttributes(
						attribute.String("http.method", req.Method),
						attribute.String("http.url", req.URL.String()),
						attribute.String("correlation_id", correlationID),
					),
				)
				defer span.End()
				c.SetRequest(req.WithContext(ctx))
			}

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			traceID := ""
			spanID := ""
			if span := trace.SpanFromContext(c.Request().Context()); span.SpanContext().IsValid() {
				traceID = span.SpanContext().TraceID().String()
				spanID = span.SpanContext().SpanID().String()
			}

			Logger.Info("http_request",
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", c.Response().Status),
				slog.Duration("latency", latency),
				slog.String("correlation_id", correlationID),
				slog.String("trace_id", traceID),
				slog.String("span_id", spanID),
			)

			return err
		}
	}
}
