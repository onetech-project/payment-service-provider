package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"backbone-new/internal/infrastructure/redis"

	"github.com/labstack/echo/v4"
)

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

// IdempotencyMiddleware enforces Idempotency-Key handling for mutating
// requests. lockTTL bounds how long a concurrent duplicate request is held
// off while the original is in flight; cacheTTL is how long the completed
// response is replayed for a repeated key. Both are caller-supplied (sourced
// from env, e.g. IDEMPOTENCY_LOCK_TTL_SECONDS / IDEMPOTENCY_CACHE_TTL_SECONDS)
// rather than hardcoded, since they're operational tuning knobs.
func IdempotencyMiddleware(redisClient *redis.Client, lockTTL, cacheTTL time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// Only enforce idempotency on state-mutating requests (POST, PUT, DELETE, PATCH)
			if req.Method == http.MethodGet || req.Method == http.MethodOptions || req.Method == http.MethodHead {
				return next(c)
			}

			idempotencyKey := req.Header.Get("Idempotency-Key")
			if idempotencyKey == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"responseCode":    "4007300",
					"responseMessage": "Bad Request. Idempotency-Key header is required.",
				})
			}

			// Read and hash payload
			var bodyBytes []byte
			if req.Body != nil {
				bodyBytes, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
			hash := sha256.Sum256(bodyBytes)
			payloadHash := hex.EncodeToString(hash[:])

			ctx := req.Context()

			// Check if response is cached
			cachedBytes, err := redisClient.GetResponseCache(ctx, idempotencyKey)
			if err == nil && cachedBytes != nil {
				var cached CachedResponse
				if err := json.Unmarshal(cachedBytes, &cached); err == nil {
					if cached.PayloadHash != payloadHash {
						return c.JSON(http.StatusUnprocessableEntity, map[string]string{
							"responseCode":    "4227300",
							"responseMessage": "Unprocessable Entity. Idempotency-Key payload mismatch.",
						})
					}

					for k, vals := range cached.Headers {
						for _, v := range vals {
							c.Response().Header().Add(k, v)
						}
					}
					c.Response().Header().Set("X-Cache-Replay", "true")
					return c.Blob(cached.StatusCode, echo.MIMEApplicationJSON, []byte(cached.Body))
				}
			}

			// Acquire lock
			locked, err := redisClient.AcquireLock(ctx, idempotencyKey, lockTTL)
			if err != nil || !locked {
				return c.JSON(http.StatusConflict, map[string]string{
					"responseCode":    "4097300",
					"responseMessage": "Conflict. Request currently in progress for this Idempotency-Key.",
				})
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
