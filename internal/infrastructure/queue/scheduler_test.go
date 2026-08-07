package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A periodic task always carries the same payload, so asynq's unique lock is
// keyed identically on every firing. That makes the uniqueness window a
// throttle on the SCHEDULE itself, not just a guard against stacking: a window
// longer than the period silently drops firings.
//
// This is not hypothetical. A hardcoded one-hour window made an "@every 1m"
// sweep run once an hour, with nothing logged to say the configured interval
// was being ignored — the sweep simply appeared to do nothing. Tying the
// window to the interval is what makes RECONCILE_INTERVAL_MINUTES mean what it
// says.
func TestRegisterPeriodic_RejectsNonPositiveInterval(t *testing.T) {
	s := NewScheduler("127.0.0.1:6379", "", 0)
	defer s.Shutdown()

	for _, interval := range []time.Duration{0, -time.Minute} {
		err := s.RegisterPeriodic("@every 1m", "test:task", interval)
		require.Error(t, err, "interval %s must be rejected", interval)
		assert.Contains(t, err.Error(), "interval must be positive")
	}
}

// The guard must not reject a legitimate interval.
func TestRegisterPeriodic_AcceptsAPositiveInterval(t *testing.T) {
	s := NewScheduler("127.0.0.1:6379", "", 0)
	defer s.Shutdown()

	assert.NoError(t, s.RegisterPeriodic("@every 5m", "test:task", 5*time.Minute))
}
