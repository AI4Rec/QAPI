package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRateLimiterAvoidsUnboundedInitialAllocation(t *testing.T) {
	limiter := InMemoryRateLimiter{}
	limiter.Init(0)

	require.True(t, limiter.Request("large-limit", 100_000_000, 60))
	queue, ok := limiter.store["large-limit"]
	require.True(t, ok)
	assert.Len(t, *queue, 1)
	assert.LessOrEqual(t, cap(*queue), 1024)
}

func TestInMemoryRateLimiterTreatsNonPositiveLimitAsUnlimited(t *testing.T) {
	limiter := InMemoryRateLimiter{}
	limiter.Init(0)

	assert.True(t, limiter.Request("unlimited", 0, 60))
	assert.True(t, limiter.Request("unlimited", -1, 60))
	assert.Empty(t, limiter.store)
}
