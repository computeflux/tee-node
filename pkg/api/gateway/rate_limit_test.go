package gateway

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── EndpointRateLimiter ────────────────────────────────────

func TestEndpointRateLimiter_AllowWithinLimit(t *testing.T) {
	lim := &EndpointRateLimiter{maxConc: 3}

	assert.True(t, lim.Allow())
	assert.True(t, lim.Allow())
	assert.True(t, lim.Allow())
	// 4th exceeds limit
	assert.False(t, lim.Allow())

	// Cleanup
	lim.Done()
	lim.Done()
	lim.Done()
	lim.Done() // extra Done for the 4th (which was rejected but still incremented)
}

func TestEndpointRateLimiter_DoneDecrements(t *testing.T) {
	lim := &EndpointRateLimiter{maxConc: 1}

	assert.True(t, lim.Allow())
	assert.False(t, lim.Allow())

	lim.Done()
	assert.True(t, lim.Allow())

	lim.Done()
}

func TestEndpointRateLimiter_NilAlwaysAllowed(t *testing.T) {
	var lim *EndpointRateLimiter
	assert.True(t, lim.Allow())
	lim.Done() // should not panic
}

func TestEndpointRateLimiter_SetLimits(t *testing.T) {
	lim := &EndpointRateLimiter{maxConc: 1}
	assert.True(t, lim.Allow())
	assert.False(t, lim.Allow())

	// Increase limit
	lim.SetLimits(0, 10)
	// Old inflight is still 2, so need to wait for Done
	lim.Done()
	lim.Done()

	// Now with limit 10, 5 more should pass
	for i := 0; i < 5; i++ {
		assert.True(t, lim.Allow())
	}

	// Cleanup
	for i := 0; i < 5; i++ {
		lim.Done()
	}
}

func TestEndpointRateLimiter_Stats(t *testing.T) {
	lim := &EndpointRateLimiter{maxConc: 10}

	rpm, inflight := lim.Stats()
	assert.Equal(t, 0, rpm)
	assert.Equal(t, int64(0), inflight)

	lim.Allow()
	lim.Allow()

	_, inflight = lim.Stats()
	assert.Equal(t, int64(2), inflight)

	lim.Done()
	lim.Done()
}

func TestEndpointRateLimiter_ConcurrentAccess(t *testing.T) {
	lim := &EndpointRateLimiter{maxConc: 100}

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lim.Allow() {
				time.Sleep(time.Millisecond)
				lim.Done()
			}
		}()
	}
	wg.Wait()

	_, inflight := lim.Stats()
	assert.Equal(t, int64(0), inflight)
}

// ── getEndpointLimiter ─────────────────────────────────────

func TestGetEndpointLimiter_CreatesNew(t *testing.T) {
	// Clear global state
	endpointLimiters = make(map[uint64]*EndpointRateLimiter)

	lim1 := getEndpointLimiter(42, 0)
	assert.NotNil(t, lim1)
	assert.Equal(t, int64(50), lim1.maxConc)

	// Same ID returns same instance
	lim2 := getEndpointLimiter(42, 0)
	assert.Equal(t, lim1, lim2)
}

func TestGetEndpointLimiter_DifferentIDs(t *testing.T) {
	// Use a fresh package-level map
	endpointLimiters = make(map[uint64]*EndpointRateLimiter)

	lim1 := getEndpointLimiter(1, 0)
	lim2 := getEndpointLimiter(2, 0)
	require.True(t, lim1 != lim2, "lim1 and lim2 should be different pointers")
}

// ── GetProviderLimits ──────────────────────────────────────

func TestGetProviderLimits_KnownProviders(t *testing.T) {
	tests := []struct {
		provider string
		wantConc int64
	}{
		{"openai", 50},
		{"anthropic", 20},
		{"gemini", 100},
		{"default", 50},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			rpm, maxConc := GetProviderLimits(tt.provider)
			assert.Equal(t, 0, rpm)
			assert.Equal(t, tt.wantConc, maxConc)
		})
	}
}

func TestGetProviderLimits_UnknownProvider(t *testing.T) {
	rpm, maxConc := GetProviderLimits("unknown")
	assert.Equal(t, 0, rpm)
	assert.Equal(t, int64(50), maxConc)
}
