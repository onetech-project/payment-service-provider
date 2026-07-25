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

	getErr   error
	lockErr  error
	lockOK   *bool // nil means "compute from locked map"
	setErr   error
	relErr   error
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
	assert.Contains(t, rec.Body.String(), "4007300")
}

func TestIdempotencyMiddleware_FirstRequestCachesResponse(t *testing.T) {
	e := echo.New()
	body := `{"foo":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "key-1")
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
	req1.Header.Set("Idempotency-Key", "key-2")
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
	req2.Header.Set("Idempotency-Key", "key-2")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	assert.NoError(t, handler(c2))

	assert.Equal(t, 1, calls, "handler must not run again on a cache replay")
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "true", rec2.Header().Get("X-Cache-Replay"))
	assert.Contains(t, rec2.Body.String(), "ok")
}

func TestIdempotencyMiddleware_PayloadMismatchReturns422(t *testing.T) {
	e := echo.New()
	store := newFakeIdempotencyStore()
	mw := IdempotencyMiddleware(store, time.Second, time.Second)
	handler := mw(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"bar"}`))
	req1.Header.Set("Idempotency-Key", "key-3")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	assert.NoError(t, handler(c1))

	req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"foo":"different"}`))
	req2.Header.Set("Idempotency-Key", "key-3")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	assert.NoError(t, handler(c2))

	assert.Equal(t, http.StatusUnprocessableEntity, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "4227300")
}

func TestIdempotencyMiddleware_LockFailureReturns409(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "key-4")
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
	assert.Contains(t, rec.Body.String(), "4097300")
}

func TestIdempotencyMiddleware_LockErrorReturns409(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "key-5")
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
	req.Header.Set("Idempotency-Key", "key-6")
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
	req.Header.Set("Idempotency-Key", "key-7")
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
