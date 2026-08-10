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

func TestIdempotencyMiddleware_FirstRequestCachesResponse(t *testing.T) {
	e := echo.New()
	body := `{"foo":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("X-EXTERNAL-ID", "key-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"status": "created"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "created")

	store.mu.Lock()
	_, cached := store.cache["key-1"]
	_, stillLocked := store.locked["key-1"]
	store.mu.Unlock()
	assert.True(t, cached, "response should be cached after successful completion")
	assert.False(t, stillLocked, "lock should be released after the request completes")
}

func TestIdempotencyMiddleware_ReplaysCachedResponseOnMatchingPayload(t *testing.T) {
	e := echo.New()
	body := `{"foo":"bar"}`

	store := newFakeIdempotencyStore()

	// First request populates the cache.
	req1 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req1.Header.Set("X-EXTERNAL-ID", "key-2")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	calls := 0
	handler := mw(func(c echo.Context) error {
		calls++
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	assert.NoError(t, handler(c1))
	assert.Equal(t, 1, calls)

	// Second request with the same key and payload should replay the cache
	// without invoking the handler again.
	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req2.Header.Set("X-EXTERNAL-ID", "key-2")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	assert.NoError(t, handler(c2))

	assert.Equal(t, 1, calls, "handler must not run again on a cache replay")
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "true", rec2.Header().Get("X-Cache-Replay"))
	assert.Contains(t, rec2.Body.String(), "ok")
}

// WithReplaySuppressedFor lets the VA payment endpoint see its own duplicates
// so it can answer 4042518 instead of replaying the original success.
func TestIdempotencyMiddleware_SuppressedReplayReachesHandlerAgain(t *testing.T) {
	e := echo.New()
	body := `{"paymentRequestId":"PAY-1"}`

	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second,
		WithReplaySuppressedFor(func(c echo.Context) bool {
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

	// Same X-EXTERNAL-ID, same payload — normally a replay, here it must reach
	// the handler so the duplicate can be rejected on its merits.
	c2, rec2 := newReq()
	assert.NoError(t, handler(c2))

	assert.Equal(t, 2, calls, "suppressed replay must re-invoke the handler")
	assert.Empty(t, rec2.Header().Get("X-Cache-Replay"))
	assert.Equal(t, http.StatusNotFound, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "4042518")
}

// Suppressing the replay must not weaken the payload-mismatch guard: a reused
// X-EXTERNAL-ID carrying a different body is still a conflict, not a handler
// call.
func TestIdempotencyMiddleware_SuppressedReplayStillRejectsPayloadMismatch(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second,
		WithReplaySuppressedFor(func(echo.Context) bool { return true }))

	calls := 0
	handler := mw(func(c echo.Context) error {
		calls++
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"bar"}`))
	req1.Header.Set("X-EXTERNAL-ID", "extid-mismatch")
	assert.NoError(t, handler(e.NewContext(req1, httptest.NewRecorder())))

	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"different"}`))
	req2.Header.Set("X-EXTERNAL-ID", "extid-mismatch")
	rec2 := httptest.NewRecorder()
	assert.NoError(t, handler(e.NewContext(req2, rec2)))

	assert.Equal(t, 1, calls, "payload mismatch must not reach the handler")
	assert.Equal(t, http.StatusConflict, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "4097300")
}

func TestIdempotencyMiddleware_PayloadMismatchReturnsConflict(t *testing.T) {
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
