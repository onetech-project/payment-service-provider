package database

import (
	"context"
	"testing"
	"time"

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

func TestParseAmount(t *testing.T) {
	v, err := parseAmount("150000.00")
	assert.NoError(t, err)
	assert.InDelta(t, 150000.00, v, 0.001)

	v2, err := parseAmount("0")
	assert.NoError(t, err)
	assert.InDelta(t, 0, v2, 0.001)
}
