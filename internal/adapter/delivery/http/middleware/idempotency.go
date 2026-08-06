package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// IdempotencyStore is the subset of redis.Client's behaviour the idempotency
// middleware depends on. Defined here (consumer side) so tests can fake it
// without a live Redis connection.
type IdempotencyStore interface {
	GetResponseCache(ctx context.Context, key string) ([]byte, error)
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
	SetResponseCache(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type CachedResponse struct {
	StatusCode  int                 `json:"statusCode"`
	Headers     map[string][]string `json:"headers"`
	Body        string              `json:"body"`
	PayloadHash string              `json:"payloadHash"`
}

type bodyInterceptor struct {
	echo.Response
	body *bytes.Buffer
}

func (w *bodyInterceptor) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.Response.Write(b)
}

// IdempotencyOption tunes IdempotencyMiddleware behaviour.
type IdempotencyOption func(*idempotencyConfig)

type idempotencyConfig struct {
	suppressReplay func(echo.Context) bool
}

// WithReplaySuppressedFor stops the middleware from replaying the cached
// response for requests pred matches, letting the duplicate reach the handler
// instead. The payload-mismatch and in-flight-lock guards (both 409 Conflict,
// per-service code) are untouched — only the "identical key, identical
// payload" replay is.
//
// The VA payment endpoint needs this: a resubmit of the same X-EXTERNAL-ID
// with the same paymentRequestId must answer 4042518 Inconsistent Request,
// which only the usecase can build (it needs the persisted payment's data).
// Replaying the original 2002500 from cache would hide the collision behind a
// second apparent success.
func WithReplaySuppressedFor(pred func(echo.Context) bool) IdempotencyOption {
	return func(cfg *idempotencyConfig) { cfg.suppressReplay = pred }
}

// IdempotencyMiddleware enforces idempotency for mutating requests, keyed on
// the ASPI-standard X-EXTERNAL-ID header (rather than a custom Idempotency-Key
// header, which isn't part of the SNAP/ASPI contract). lockTTL bounds how
// long a concurrent duplicate request is held off while the original is in
// flight; cacheTTL is how long the completed response is replayed for a
// repeated key. Both are caller-supplied (sourced from env, e.g.
// IDEMPOTENCY_LOCK_TTL_SECONDS / IDEMPOTENCY_CACHE_TTL_SECONDS) rather than
// hardcoded, since they're operational tuning knobs.
func IdempotencyMiddleware(redisClient IdempotencyStore, lockTTL, cacheTTL time.Duration, opts ...IdempotencyOption) echo.MiddlewareFunc {
	cfg := &idempotencyConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// Only enforce idempotency on state-mutating requests (POST, PUT, DELETE, PATCH)
			if req.Method == http.MethodGet || req.Method == http.MethodOptions || req.Method == http.MethodHead {
				return next(c)
			}

			// Rejections here carry the service code of the endpoint being
			// called, and the SNAP envelope for it: this middleware runs
			// before SNAPAuthMiddleware, so for a request missing or reusing
			// X-EXTERNAL-ID it is the only thing that answers BCA.
			service := domain.ServiceCodeForPath(req.URL.Path)

			idempotencyKey := req.Header.Get(headerExternalID)
			if idempotencyKey == "" {
				return c.JSON(http.StatusBadRequest, domain.NewSNAPErrorBody(
					service,
					domain.CodeMissingMandatory(service),
					"Invalid Mandatory Field ["+headerExternalID+"]",
					domain.VAIdentityEcho{},
				))
			}

			// Read and hash payload
			var bodyBytes []byte
			if req.Body != nil {
				bodyBytes, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
			hash := sha256.Sum256(bodyBytes)
			payloadHash := base64.StdEncoding.EncodeToString(hash[:])

			ctx := req.Context()

			// Check if response is cached
			cachedBytes, err := redisClient.GetResponseCache(ctx, idempotencyKey)
			if err == nil && cachedBytes != nil {
				var cached CachedResponse
				if err := json.Unmarshal(cachedBytes, &cached); err == nil {
					// Same key, different payload. SNAP has no 422 — BCA
					// documents this exact case ("same X-EXTERNAL-ID but a
					// different paymentRequestId") as 409 Conflict, so that is
					// what we answer.
					if cached.PayloadHash != payloadHash {
						return c.JSON(http.StatusConflict, domain.NewSNAPErrorBody(
							service,
							domain.CodeConflict(service),
							"Conflict",
							domain.VAIdentityEcho{},
						))
					}

					if cfg.suppressReplay == nil || !cfg.suppressReplay(c) {
						for k, vals := range cached.Headers {
							for _, v := range vals {
								c.Response().Header().Add(k, v)
							}
						}
						c.Response().Header().Set("X-Cache-Replay", "true")
						return c.Blob(cached.StatusCode, echo.MIMEApplicationJSON, []byte(cached.Body))
					}
				}
			}

			// Acquire lock
			locked, err := redisClient.AcquireLock(ctx, idempotencyKey, lockTTL)
			if err != nil || !locked {
				return c.JSON(http.StatusConflict, domain.NewSNAPErrorBody(
					service,
					domain.CodeConflict(service),
					"Conflict",
					domain.VAIdentityEcho{},
				))
			}
			defer func() { _ = redisClient.ReleaseLock(ctx, idempotencyKey) }()

			// Intercept response
			buf := new(bytes.Buffer)
			interceptor := &bodyInterceptor{
				Response: *c.Response(),
				body:     buf,
			}
			c.Response().Writer = interceptor

			err = next(c)

			// Cache response on successful completion (status code < 500)
			if c.Response().Status < 500 {
				cached := CachedResponse{
					StatusCode:  c.Response().Status,
					Headers:     c.Response().Header(),
					Body:        buf.String(),
					PayloadHash: payloadHash,
				}
				if jsonBytes, err := json.Marshal(cached); err == nil {
					_ = redisClient.SetResponseCache(ctx, idempotencyKey, jsonBytes, cacheTTL)
				}
			}

			return err
		}
	}
}
