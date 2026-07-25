package middleware

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"backbone-new/internal/infrastructure/telemetry"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var Logger *slog.Logger

// parseLogLevel maps the LOG_LEVEL env var (debug/info/warn/error, any case)
// to a slog.Level, defaulting to Info for unset or unrecognized values.
func parseLogLevel(raw string) slog.Level {
	switch raw {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN", "warning", "WARNING":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func init() {
	opts := &slog.HandlerOptions{Level: parseLogLevel(os.Getenv("LOG_LEVEL"))}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// json is the default format — Loki/Promtail/Alloy parse it as
		// structured log lines and turn the fields into labels/metadata.
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	Logger = slog.New(handler)
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

			// Route (not raw URL) keeps the label cardinality bounded — echo
			// exposes the registered path template, e.g. "/admin/clients/:clientId/keys".
			route := c.Path()
			if route == "" {
				route = "unmatched"
			}
			status := c.Response().Status
			telemetry.HTTPRequestsTotal.WithLabelValues(req.Method, route, strconv.Itoa(status)).Inc()
			telemetry.HTTPRequestDuration.WithLabelValues(req.Method, route).Observe(latency.Seconds())

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
