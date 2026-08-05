package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVALocker is a mock for the vaLocker interface used to guard
// static/dynamic customerNo generation and registration (feature
// 006-static-dynamic-va). Methods requiring a live PostgreSQL connection
// (NextCustomerNoSequence, RegisterStaticCustomerNo, SaveVAPayment) are
// covered by quickstart.md's integration scenarios; this file exercises the
// pure-Go locking/parsing logic that doesn't require a database.
type MockVALocker struct {
	mock.Mock
}

func (m *MockVALocker) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockVALocker) ReleaseLock(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func TestVARepository_WithLock_RunsFnWhenLockAcquired(t *testing.T) {
	locker := new(MockVALocker)
	locker.On("AcquireLock", mock.Anything, "key1", mock.Anything).Return(true, nil)
	locker.On("ReleaseLock", mock.Anything, "key1").Return(nil)

	repo := NewVARepositoryWithLocker(nil, locker)

	called := false
	err := repo.withLock(context.Background(), "key1", func() error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
	locker.AssertExpectations(t)
}

func TestVARepository_WithLock_RetriesUntilAcquired(t *testing.T) {
	locker := new(MockVALocker)
	locker.On("AcquireLock", mock.Anything, "key2", mock.Anything).Return(false, nil).Once()
	locker.On("AcquireLock", mock.Anything, "key2", mock.Anything).Return(true, nil).Once()
	locker.On("ReleaseLock", mock.Anything, "key2").Return(nil)

	repo := NewVARepositoryWithLocker(nil, locker)

	called := false
	err := repo.withLock(context.Background(), "key2", func() error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
	locker.AssertExpectations(t)
}

func TestVARepository_WithLock_TimesOutWhenNeverAcquired(t *testing.T) {
	locker := new(MockVALocker)
	locker.On("AcquireLock", mock.Anything, "key3", mock.Anything).Return(false, nil)

	repo := NewVARepositoryWithLocker(nil, locker)
	repo.lockWaitOverride(50 * time.Millisecond)

	called := false
	err := repo.withLock(context.Background(), "key3", func() error {
		called = true
		return nil
	})

	assert.Error(t, err)
	assert.False(t, called)
}

func TestVARepository_WithLock_NoLockerRunsUnlocked(t *testing.T) {
	repo := NewVARepository(nil)

	called := false
	err := repo.withLock(context.Background(), "key4", func() error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, called)
}

// T003: VANotificationDeliveryRepository (Create, GetLatestByVirtualAccountNo,
// ExistsByVirtualAccountNoAndEventType). These methods require a live
// PostgreSQL connection to exercise the actual SQL (query construction and
// scanning), consistent with this package's existing convention for
// SQL-heavy methods (see MockVALocker doc comment above) — they are covered
// end-to-end by quickstart.md's integration scenarios. Here we verify the
// compile-time contract: VARepository satisfies both
// domain.VARepository and domain.VANotificationDeliveryRepository, and that
// constructing a repository doesn't panic before a real pool is attached.
func TestVARepository_ImplementsVANotificationDeliveryRepository(t *testing.T) {
	var _ domain.VANotificationDeliveryRepository = NewVARepository(nil)
	var _ domain.VARepository = NewVARepository(nil)
}

func TestParseAmount(t *testing.T) {
	v, err := parseAmount("150000.00")
	assert.NoError(t, err)
	assert.InDelta(t, 150000.00, v, 0.001)

	v2, err := parseAmount("0")
	assert.NoError(t, err)
	assert.InDelta(t, 0, v2, 0.001)
}

// VA registry error mapping (feature 013-no-bill-payment-transaction, T013).
//
// As with the rest of this file, the SQL itself is exercised by quickstart.md's
// integration scenarios against a live PostgreSQL. What IS unit-testable — and
// what the usecase layer's correctness hinges on — is the error translation:
// callers must be able to tell "no registration, fall through to the legacy
// path" from "the database is broken", and "duplicate payment, replay the
// original response" from a genuine failure. Getting either wrong is silent
// and expensive.

// fakeRow is a pgx.Row whose Scan always returns a preset error, letting the
// error-mapping branches of scanVAAccount be exercised without a pool.
type fakeRow struct{ err error }

func (r fakeRow) Scan(_ ...any) error { return r.err }

func TestScanVAAccount_NoRowsMapsToAccountNotFound(t *testing.T) {
	account, err := scanVAAccount(fakeRow{err: pgx.ErrNoRows})

	assert.Nil(t, account)
	assert.ErrorIs(t, err, domain.ErrVAAccountNotFound)
}

func TestScanVAAccount_QueryFailureIsReturnedVerbatim(t *testing.T) {
	// A broken query must NOT be flattened into ErrVAAccountNotFound: that
	// would send the caller down the legacy fall-through path and quietly
	// produce a wrong answer instead of a 500.
	boom := errors.New("connection refused")

	account, err := scanVAAccount(fakeRow{err: boom})

	assert.Nil(t, account)
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, domain.ErrVAAccountNotFound)
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unique violation", &pgconn.PgError{Code: "23505"}, true},
		{"wrapped unique violation", fmt.Errorf("insert failed: %w", &pgconn.PgError{Code: "23505"}), true},
		{"foreign key violation", &pgconn.PgError{Code: "23503"}, false},
		{"check violation", &pgconn.PgError{Code: "23514"}, false},
		{"non-pg error", errors.New("connection refused"), false},
		{"no rows", pgx.ErrNoRows, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUniqueViolation(tt.err))
		})
	}
}
