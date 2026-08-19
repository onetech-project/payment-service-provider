package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIdempotencyStore is an in-memory stand-in for redis.Client, scoped to
// the IdempotencyStore interface so the middleware can be unit tested
// without a live Redis connection.
type fakeIdempotencyStore struct {
	mu     sync.Mutex
	cache  map[string][]byte
	locked map[string]bool

	getErr  error
	lockErr error
	lockOK  *bool // nil means "compute from locked map"
	setErr  error
	relErr  error
}

func newFakeIdempotencyStore() *fakeIdempotencyStore {
	return &fakeIdempotencyStore{
		cache:  map[string][]byte{},
		locked: map[string]bool{},
	}
}

func (f *fakeIdempotencyStore) GetResponseCache(ctx context.Context, key string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.cache[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (f *fakeIdempotencyStore) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if f.lockErr != nil {
		return false, f.lockErr
	}
	if f.lockOK != nil {
		return *f.lockOK, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.locked[key] {
		return false, nil
	}
	f.locked[key] = true
	return true, nil
}

func (f *fakeIdempotencyStore) ReleaseLock(ctx context.Context, key string) error {
	if f.relErr != nil {
		return f.relErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.locked, key)
	return nil
}

func (f *fakeIdempotencyStore) SetResponseCache(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache[key] = value
	return nil
}

func TestIdempotencyMiddleware_SkipsSafeMethods(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIdempotencyMiddleware_MissingKey(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	// Missing X-EXTERNAL-ID is a mandatory-field rejection carrying the
	// service code of the endpoint called.
	assert.Contains(t, rec.Body.String(), "Invalid Mandatory Field [X-EXTERNAL-ID]")
}

func TestIdempotencyMiddleware_FirstRequestRecordsTheKey(t *testing.T) {
	e := echo.New()
	body := `{"foo":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("X-EXTERNAL-ID", "key-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second,
		WithDayStamp(func() string { return "20260817" }))
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"status": "created"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "created")

	store.mu.Lock()
	_, cached := store.cache["20260817:key-1"]
	_, stillLocked := store.locked["20260817:key-1"]
	store.mu.Unlock()
	assert.True(t, cached, "the key must be recorded as spent after a completed request")
	assert.False(t, stillLocked, "lock should be released after the request completes")
}

// X-EXTERNAL-ID is typed "unique in the same day" on all three services, so
// one key buys one request. An identical repeat used to be replayed from
// cache; it is a conflict like any other re-use, and the handler must not see
// it twice.
func TestIdempotencyMiddleware_IdenticalRepeatIsAConflict(t *testing.T) {
	e := echo.New()
	body := `{"foo":"bar"}`

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	calls := 0
	handler := mw(func(c echo.Context) error {
		calls++
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	newReq := func() (echo.Context, *httptest.ResponseRecorder) {
		r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		r.Header.Set("X-EXTERNAL-ID", "key-2")
		rec := httptest.NewRecorder()
		return e.NewContext(r, rec), rec
	}

	c1, _ := newReq()
	assert.NoError(t, handler(c1))
	assert.Equal(t, 1, calls)

	c2, rec2 := newReq()
	assert.NoError(t, handler(c2))

	assert.Equal(t, 1, calls, "a repeated key must not reach the handler again")
	assert.Equal(t, http.StatusConflict, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Conflict")
	assert.Empty(t, rec2.Header().Get("X-Cache-Replay"), "nothing is replayed any more")
}

// BCA's post-system-error double flag repeats BOTH the X-EXTERNAL-ID and the
// paymentRequestId, and must be answered 4042518 by the handler — a 409 here
// would make BCA book an already-settled payment as failed.
func TestIdempotencyMiddleware_DoubleFlagReachesHandler(t *testing.T) {
	e := echo.New()
	body := `{"paymentRequestId":"PAY-1"}`

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second,
		WithDoubleFlagPassthroughFor(func(c echo.Context) bool {
			return strings.HasSuffix(c.Request().URL.Path, "/transfer-va/payment")
		}))

	calls := 0
	handler := mw(func(c echo.Context) error {
		calls++
		if calls == 1 {
			return c.JSON(http.StatusOK, map[string]string{"responseCode": "2002500"})
		}
		return c.JSON(http.StatusNotFound, map[string]string{"responseCode": "4042518"})
	})

	newReq := func() (echo.Context, *httptest.ResponseRecorder) {
		r := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", strings.NewReader(body))
		r.Header.Set("X-EXTERNAL-ID", "extid-dup")
		rec := httptest.NewRecorder()
		return e.NewContext(r, rec), rec
	}

	c1, _ := newReq()
	assert.NoError(t, handler(c1))
	assert.Equal(t, 1, calls)

	c2, rec2 := newReq()
	assert.NoError(t, handler(c2))

	assert.Equal(t, 2, calls, "the double flag must re-invoke the handler")
	assert.Equal(t, http.StatusNotFound, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "4042518")
}

// The exemption is scoped to the double flag alone: a reused key carrying a
// DIFFERENT paymentRequestId is the case BCA spells 4092500, and one carrying
// no paymentRequestId at all is not a double flag either — two blanks matching
// must not open the passthrough.
func TestIdempotencyMiddleware_DoubleFlagPassthroughRejectsEverythingElse(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"different paymentRequestId", `{"paymentRequestId":"PAY-1"}`, `{"paymentRequestId":"PAY-2"}`},
		{"paymentRequestId dropped", `{"paymentRequestId":"PAY-1"}`, `{"foo":"bar"}`},
		{"no paymentRequestId either side", `{"foo":"bar"}`, `{"foo":"bar"}`},
		{"unparseable body", `{"paymentRequestId":"PAY-1"}`, `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			store := newFakeIdempotencyStore()
			mw := IdempotencyMiddleware(store, time.Second, time.Second,
				WithDoubleFlagPassthroughFor(func(echo.Context) bool { return true }))

			calls := 0
			handler := mw(func(c echo.Context) error {
				calls++
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			req1 := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", strings.NewReader(tc.first))
			req1.Header.Set("X-EXTERNAL-ID", "extid-mismatch")
			assert.NoError(t, handler(e.NewContext(req1, httptest.NewRecorder())))

			req2 := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", strings.NewReader(tc.second))
			req2.Header.Set("X-EXTERNAL-ID", "extid-mismatch")
			rec2 := httptest.NewRecorder()
			assert.NoError(t, handler(e.NewContext(req2, rec2)))

			assert.Equal(t, 1, calls, "only the double flag may reach the handler")
			assert.Equal(t, http.StatusConflict, rec2.Code)
			assert.Contains(t, rec2.Body.String(), "4092500")
		})
	}
}

// Inquiry and status get no exemption at all: their own tech docs list only
// "Cannot use the same X-EXTERNAL-ID → 409", so even a repeat that would be a
// double flag on payment is a conflict here.
func TestIdempotencyMiddleware_NoPassthroughWithoutTheOption(t *testing.T) {
	for _, path := range []string{"/openapi/v1.0/transfer-va/inquiry", "/openapi/v2.0/transfer-va/status"} {
		e := echo.New()
		store := newFakeIdempotencyStore()
		mw := IdempotencyMiddleware(store, time.Second, time.Second,
			WithDoubleFlagPassthroughFor(func(c echo.Context) bool {
				return strings.HasSuffix(c.Request().URL.Path, "/transfer-va/payment")
			}))

		calls := 0
		handler := mw(func(c echo.Context) error {
			calls++
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		body := `{"paymentRequestId":"PAY-1"}`
		req1 := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req1.Header.Set("X-EXTERNAL-ID", "extid-"+path)
		assert.NoError(t, handler(e.NewContext(req1, httptest.NewRecorder())))

		req2 := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req2.Header.Set("X-EXTERNAL-ID", "extid-"+path)
		rec2 := httptest.NewRecorder()
		assert.NoError(t, handler(e.NewContext(req2, rec2)))

		assert.Equal(t, 1, calls, path)
		assert.Equal(t, http.StatusConflict, rec2.Code, path)
	}
}

func TestIdempotencyMiddleware_RepeatWithDifferentBodyIsAConflict(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"bar"}`))
	req1.Header.Set("X-EXTERNAL-ID", "key-3")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	assert.NoError(t, handler(c1))

	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"different"}`))
	req2.Header.Set("X-EXTERNAL-ID", "key-3")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	assert.NoError(t, handler(c2))

	// SNAP has no 422. BCA documents a reused X-EXTERNAL-ID carrying a
	// different payload as 409 Conflict.
	assert.Equal(t, http.StatusConflict, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Conflict")
}

func TestIdempotencyMiddleware_LockFailureReturns409(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("X-EXTERNAL-ID", "key-4")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	notLocked := false
	store.lockOK = &notLocked

	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "Conflict")
}

func TestIdempotencyMiddleware_LockErrorReturns409(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("X-EXTERNAL-ID", "key-5")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	store.lockErr = errors.New("redis unavailable")

	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestIdempotencyMiddleware_ServerErrorNotCached(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("X-EXTERNAL-ID", "key-6")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusInternalServerError, map[string]string{"status": "error"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	store.mu.Lock()
	_, cached := store.cache["key-6"]
	store.mu.Unlock()
	assert.False(t, cached, "5xx responses must not be cached")
}

func TestIdempotencyMiddleware_CorruptCacheEntryFallsThroughToHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("X-EXTERNAL-ID", "key-7")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	store.cache["key-7"] = []byte("not-json")

	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	called := false
	handler := mw(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.True(t, called, "handler should run when the cache entry can't be unmarshalled")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// "Unique in the same day" is a CALENDAR day, not a rolling twenty-four hours.
// An X-EXTERNAL-ID spent at 23:50 must be usable again at 00:00 — a window
// that runs into the next day rejects a re-use BCA is entitled to make, and on
// the payment endpoint a 409 is what BCA reads as a failed transaction.
func TestIdempotencyMiddleware_KeyIsFreedByTheCalendarDay(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()

	day := "20260817"
	mw := IdempotencyMiddleware(store, time.Second, 24*time.Hour,
		WithDayStamp(func() string { return day }))

	calls := 0
	handler := mw(func(c echo.Context) error {
		calls++
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	hit := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"bar"}`))
		r.Header.Set("X-EXTERNAL-ID", "reused-id")
		rec := httptest.NewRecorder()
		_ = handler(e.NewContext(r, rec))
		return rec
	}

	require.Equal(t, http.StatusOK, hit().Code)
	assert.Equal(t, http.StatusConflict, hit().Code, "same day, same id → conflict")
	assert.Equal(t, 1, calls)

	day = "20260818"
	assert.Equal(t, http.StatusOK, hit().Code, "new calendar day frees the id")
	assert.Equal(t, 2, calls)

	assert.Equal(t, http.StatusConflict, hit().Code, "and it is spent again for the new day")
	assert.Equal(t, 2, calls)
}

// The stored key carries the date, so the boundary is drawn by the key itself
// rather than by a TTL landing on the right second.
func TestIdempotencyMiddleware_StoredKeyIsScopedByDay(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()
	handler := IdempotencyMiddleware(store, time.Second, 24*time.Hour,
		WithDayStamp(func() string { return "20260817" }),
	)(func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })

	r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	r.Header.Set("X-EXTERNAL-ID", "abc123")
	require.NoError(t, handler(e.NewContext(r, httptest.NewRecorder())))

	store.mu.Lock()
	defer store.mu.Unlock()
	_, ok := store.cache["20260817:abc123"]
	assert.True(t, ok, "record must be scoped by date, got keys: %v", store.cache)
}

func TestDayTimezone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err, "runtime must ship tzdata for this to be meaningful")

	assert.Equal(t, jakarta, DayTimezone(""), "empty falls back to WIB")
	assert.Equal(t, jakarta, DayTimezone("Not/AZone"), "unknown falls back to WIB")
	assert.Equal(t, jakarta, DayTimezone("Asia/Jakarta"))

	utc := DayTimezone("UTC")
	assert.Equal(t, time.UTC.String(), utc.String(), "an explicit valid zone is honoured")

	// The fallback must land on the same wall clock as WIB, since that is the
	// whole point of falling back to it.
	at := time.Date(2026, 8, 17, 17, 30, 0, 0, time.UTC)
	assert.Equal(t, at.In(jakarta).Format("20060102"),
		at.In(time.FixedZone("WIB", 7*60*60)).Format("20060102"))
}

// The middleware must draw the boundary in WIB even when no option is passed,
// so a deployment that forgets to wire one is not silently measuring days in
// whatever zone the container happens to run.
func TestIdempotencyMiddleware_DefaultsToJakartaDayBoundary(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()
	handler := IdempotencyMiddleware(store, time.Second, 24*time.Hour)(
		func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })

	r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	r.Header.Set("X-EXTERNAL-ID", "abc123")
	require.NoError(t, handler(e.NewContext(r, httptest.NewRecorder())))

	wantKey := time.Now().In(DayTimezone("")).Format("20060102") + ":abc123"
	store.mu.Lock()
	defer store.mu.Unlock()
	_, ok := store.cache[wantKey]
	assert.True(t, ok, "want %s, got keys: %v", wantKey, store.cache)
}
