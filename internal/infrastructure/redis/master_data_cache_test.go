package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewMasterDataCache_Constructs is a smoke test: Get/Set themselves
// require a live Redis connection to exercise meaningfully (same limitation
// as ClientKeyCache in this package, which also has no dedicated unit test).
// The cache-aside/fallback logic that matters most is tested against a mock
// of this cache's interface in internal/infrastructure/cache's test suite;
// live behavior is covered by quickstart.md's Scenarios 8/9.
func TestNewMasterDataCache_Constructs(t *testing.T) {
	cache := NewMasterDataCache(nil)
	assert.NotNil(t, cache)
}
