package middleware

import (
	"bytes"
	"context"
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

// CachedResponse records that an X-EXTERNAL-ID has already been answered.
//
// It is a seen-marker, not a response cache: nothing is ever replayed from it.
// BCA types X-EXTERNAL-ID as "unique in the same day" on all three services,
// so a second request carrying one already used is a conflict rather than
// something to answer twice. StatusCode/Headers/Body are kept because the
// record is also the only trace of what the first request was answered, which
// is what an operator reaches for when BCA disputes an outcome.
//
// PaymentRequestID is populated on the payment endpoint alone and is what the
// double-flag exemption compares; see WithDoubleFlagPassthroughFor.
type CachedResponse struct {
	StatusCode       int                 `json:"statusCode"`
	Headers          map[string][]string `json:"headers"`
	Body             string              `json:"body"`
	PaymentRequestID string              `json:"paymentRequestId"`
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
	doubleFlagPassthrough func(echo.Context) bool
	dayStamp              func() string
}

// defaultDayTimezone is where "the same day" is measured when nothing else is
// configured. BCA operates in WIB, so a boundary drawn anywhere else would put
// the reset at the wrong hour — under UTC, for instance, an X-EXTERNAL-ID
// first used at 06:00 WIB would still be counted against the previous day.
const defaultDayTimezone = "Asia/Jakarta"

// DayTimezone resolves the location the day boundary is measured in, falling
// back to WIB whenever it cannot honour the name it was given — an empty
// setting, a typo, or a runtime image shipped without a tzdata database. The
// fallback is a fixed +07:00 offset rather than another LoadLocation attempt,
// because the case it exists for is precisely the one where LoadLocation
// cannot work. WIB has no daylight saving, so the fixed offset is exact rather
// than an approximation.
func DayTimezone(name string) *time.Location {
	if name == "" {
		name = defaultDayTimezone
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	if name != defaultDayTimezone {
		if loc, err := time.LoadLocation(defaultDayTimezone); err == nil {
			return loc
		}
	}
	return time.FixedZone("WIB", 7*60*60)
}

// WithDayBoundaryIn measures "the same day" in loc.
//
// BCA types X-EXTERNAL-ID "unique in the same day", and the day is a CALENDAR
// day, not a rolling window: an id spent at 23:50 is free again at 00:00, not
// twenty-four hours later. Scoping the stored key by the date means the
// boundary is drawn by the key itself — at midnight every id is simply
// looking at a different record — instead of relying on a TTL landing on the
// right second.
//
// Getting this wrong is not symmetric. A window that runs long rejects a
// re-use that BCA is entitled to make, and on the payment endpoint a 409 is
// what BCA reads as a failed transaction.
func WithDayBoundaryIn(loc *time.Location) IdempotencyOption {
	return func(cfg *idempotencyConfig) {
		cfg.dayStamp = func() string { return time.Now().In(loc).Format("20060102") }
	}
}

// WithDayStamp drives the day boundary directly. It is the seam tests use to
// cross midnight without waiting for it.
func WithDayStamp(stamp func() string) IdempotencyOption {
	return func(cfg *idempotencyConfig) { cfg.dayStamp = stamp }
}

// WithDoubleFlagPassthroughFor exempts the endpoints pred matches from the
// one-hit-per-X-EXTERNAL-ID rule when the repeat carries the SAME
// paymentRequestId, letting it reach the handler instead of being rejected.
//
// Only the VA payment endpoint needs it, and it is not a relaxation — it is
// the other half of a rule BCA splits in two (Tech. Doc. OpenAPI
// VA-Payment-Flag v2.3, "Note"):
//
//	same X-EXTERNAL-ID, DIFFERENT paymentRequestId → 4092500 "Conflict"
//	same X-EXTERNAL-ID, SAME paymentRequestId      → 4042518 "Inconsistent
//	                                                 Request"
//
// The second is the double flag BCA sends after a system error, and only the
// usecase can answer it: 4042518 must carry the paymentFlagStatus/Reason of
// the FIRST request, which lives in the payment record. Answering it 409
// instead would not merely be untidy — BCA states that any code other than
// 2002500, 2022500 or 4042518 makes it "consider the response as failed
// transaction", so a payment already settled on our side would be booked as
// failed on theirs.
//
// Inquiry and status have no such carve-out in their own tech docs, and so get
// the strict rule: any repeat is a conflict, whatever the body says.
func WithDoubleFlagPassthroughFor(pred func(echo.Context) bool) IdempotencyOption {
	return func(cfg *idempotencyConfig) { cfg.doubleFlagPassthrough = pred }
}

// paymentRequestIDOf extracts the field the double-flag exemption compares.
// It returns "" for every endpoint without the exemption, so no other service
// pays for parsing a body it does not need read.
func paymentRequestIDOf(cfg *idempotencyConfig, c echo.Context, body []byte) string {
	if cfg.doubleFlagPassthrough == nil || !cfg.doubleFlagPassthrough(c) {
		return ""
	}
	var probe struct {
		PaymentRequestID string `json:"paymentRequestId"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	return probe.PaymentRequestID
}

// isDoubleFlag reports whether a repeat of an already-answered X-EXTERNAL-ID
// is BCA's documented double flag rather than a conflict.
//
// It is never true off the payment endpoint, and never true for an empty
// paymentRequestId: an unparseable or id-less body is not a double flag, and
// treating it as one would let a genuine key re-use through on the strength of
// two blanks matching.
func isDoubleFlag(cfg *idempotencyConfig, c echo.Context, cached CachedResponse, paymentRequestID string) bool {
	if cfg.doubleFlagPassthrough == nil || !cfg.doubleFlagPassthrough(c) {
		return false
	}
	return paymentRequestID != "" && cached.PaymentRequestID == paymentRequestID
}

// IdempotencyMiddleware enforces idempotency for mutating requests, keyed on
// X-EXTERNAL-ID.
//
// That is the only key there is. BCA's header tables — the common one in
// OAuth & Signature v1.1 and the per-service ones in VA-BillPresentment v2.4,
// VA-Payment-Flag v2.3 and VA-Payment-Status V2 v1.0 — publish exactly
// Content-Type, Authorization, X-TIMESTAMP, X-SIGNATURE, ORIGIN (optional),
// CHANNEL-ID, X-PARTNER-ID and X-EXTERNAL-ID. There is no Idempotency-Key in
// any of them, so none is read or sent anywhere in this service.
//
// lockTTL bounds how long a concurrent duplicate request is held off while the
// original is in flight. cacheTTL is how long the seen-record survives; with
// the key scoped by date it is a garbage-collection bound rather than the rule
// itself, and must simply outlive a day so a record written at 00:01 is still
// there at 23:59. Both are caller-supplied (sourced from env, e.g.
// IDEMPOTENCY_LOCK_TTL_SECONDS / IDEMPOTENCY_CACHE_TTL_SECONDS) rather than
// hardcoded, since they're operational tuning knobs.
func IdempotencyMiddleware(redisClient IdempotencyStore, lockTTL, cacheTTL time.Duration, opts ...IdempotencyOption) echo.MiddlewareFunc {
	cfg := &idempotencyConfig{}
	WithDayBoundaryIn(DayTimezone(""))(cfg)
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

			externalID := req.Header.Get(headerExternalID)
			// The stored key is scoped by calendar day, which is what makes
			// the rule "unique in the same day" rather than "unique for the
			// next twenty-four hours": once the date rolls over, the same
			// X-EXTERNAL-ID addresses a record that does not exist yet.
			idempotencyKey := cfg.dayStamp() + ":" + externalID
			if externalID == "" {
				return c.JSON(http.StatusBadRequest, domain.NewSNAPErrorBody(
					service,
					domain.CodeMissingMandatory(service),
					"Invalid Mandatory Field ["+headerExternalID+"]",
					domain.VAIdentityEcho{},
				))
			}

			var bodyBytes []byte
			if req.Body != nil {
				bodyBytes, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
			// Only the payment endpoint reads anything out of the body, and
			// only the one field the double-flag exemption turns on. A body
			// that does not parse leaves it empty, which is not fatal here:
			// the request goes on to the handler, which rejects it 400.
			paymentRequestID := paymentRequestIDOf(cfg, c, bodyBytes)

			ctx := req.Context()

			// A key that has already been answered. X-EXTERNAL-ID is typed
			// "unique in the same day" on all three services, so re-using one
			// is a conflict on its own — the body is not consulted, and an
			// identical repeat is no more acceptable than a changed one.
			//
			// The single exception is BCA's double flag on the payment
			// endpoint: same key AND same paymentRequestId, which must reach
			// the handler to be answered 4042518. See
			// WithDoubleFlagPassthroughFor.
			cachedBytes, err := redisClient.GetResponseCache(ctx, idempotencyKey)
			if err == nil && cachedBytes != nil {
				var cached CachedResponse
				if err := json.Unmarshal(cachedBytes, &cached); err == nil {
					if !isDoubleFlag(cfg, c, cached, paymentRequestID) {
						return c.JSON(http.StatusConflict, domain.NewSNAPErrorBody(
							service,
							domain.CodeConflict(service),
							"Conflict",
							domain.VAIdentityEcho{},
						))
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

			// Record the key as answered on completion (status code < 500). A
			// 5xx is deliberately not recorded: it decided nothing, and BCA
			// retrying after one must not meet a 409 for a request that was
			// never actually answered.
			if c.Response().Status < 500 {
				cached := CachedResponse{
					StatusCode:       c.Response().Status,
					Headers:          c.Response().Header(),
					Body:             buf.String(),
					PaymentRequestID: paymentRequestID,
				}
				if jsonBytes, err := json.Marshal(cached); err == nil {
					_ = redisClient.SetResponseCache(ctx, idempotencyKey, jsonBytes, cacheTTL)
				}
			}

			return err
		}
	}
}
