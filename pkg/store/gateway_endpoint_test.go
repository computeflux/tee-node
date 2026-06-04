package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointBalancer_Pick_NewUser(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: true},
		{ID: 2, URL: "http://b", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)
	require.NotNil(t, b)

	// Same user key should always get the same endpoint (sticky)
	ep1 := b.Pick("alice")
	require.NotNil(t, ep1)

	ep2 := b.Pick("alice")
	require.NotNil(t, ep2)
	assert.Equal(t, ep1.ID, ep2.ID, "same user should stick to same endpoint")
}

func TestEndpointBalancer_Pick_DifferentUsers(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: true},
		{ID: 2, URL: "http://b", Weight: 1, IsActive: true},
		{ID: 3, URL: "http://c", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	// Different users may get different endpoints
	m := make(map[uint64]int)
	for i := 0; i < 100; i++ {
		ep := b.Pick(string(rune('a' + i%26)))
		m[ep.ID]++
	}

	// All endpoints should have some users
	assert.GreaterOrEqual(t, len(m), 2, "should distribute across endpoints")
}

func TestEndpointBalancer_Pick_InactiveEndpoint(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: false},
		{ID: 2, URL: "http://b", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	// User assigned to inactive endpoint should be reassigned
	ep := b.Pick("user1")
	assert.Equal(t, uint64(2), ep.ID)
}

func TestEndpointBalancer_Pick_SingleEndpoint(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 5, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	ep := b.Pick("user1")
	assert.Equal(t, uint64(1), ep.ID)
}

func TestEndpointBalancer_Pick_Empty(t *testing.T) {
	b := NewEndpointBalancerFromEndpoints([]*GatewayEndpoint{})
	assert.Nil(t, b)
}

func TestEndpointBalancer_Release(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	b.Pick("user1")
	stats := b.Stats()
	assert.Equal(t, int64(1), stats[1].Inflight)

	b.Release(1)
	stats = b.Stats()
	assert.Equal(t, int64(0), stats[1].Inflight)
}

func TestEndpointBalancer_OnEndpointDisabled(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: true},
		{ID: 2, URL: "http://b", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	// Bind multiple users
	b.Pick("alice")
	b.Pick("bob")

	// Verify both are bound
	statsBefore := b.Stats()
	totalUsersBefore := 0
	for _, s := range statsBefore {
		totalUsersBefore += s.Users
	}
	assert.Equal(t, 2, totalUsersBefore)

	// Disable endpoint 1: clear all bindings to it
	b.OnEndpointDisabled(1)

	// Verify bindings are cleared
	statsAfter := b.Stats()
	assert.Equal(t, 0, statsAfter[1].Users)
}

func TestEndpointBalancer_Stats(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 1, IsActive: true},
		{ID: 2, URL: "http://b", Weight: 1, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	b.Pick("alice")
	b.Pick("bob")
	b.Pick("charlie")

	stats := b.Stats()
	assert.Len(t, stats, 2)
	assert.Equal(t, int64(3), stats[1].Inflight+stats[2].Inflight)
}

func TestEndpointBalancer_WeightedDistribution(t *testing.T) {
	eps := []*GatewayEndpoint{
		{ID: 1, URL: "http://a", Weight: 90, IsActive: true},
		{ID: 2, URL: "http://b", Weight: 10, IsActive: true},
	}
	b := NewEndpointBalancerFromEndpoints(eps)

	count1, count2 := 0, 0
	for i := 0; i < 1000; i++ {
		ep := b.Pick(string(rune('a' + i%26)))
		if ep.ID == 1 {
			count1++
		} else {
			count2++
		}
	}

	// Roughly 90% should go to endpoint 1
	assert.Greater(t, count1, count2*5)
}
