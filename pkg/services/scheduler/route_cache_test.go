package scheduler

import (
	"testing"
	"time"
)

func sampleRoute(model ModelKey) *ModelRoute {
	return &ModelRoute{
		Model:          model,
		MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{
				Provider:  ProviderID("p1"),
				KeyID:     "k1",
				Enabled:   true,
				Weight:    1,
				Models:    []ModelKey{model},
				metrics:   &EndpointMetrics{},
			},
		},
	}
}

func TestModelRouteCache_GetSet(t *testing.T) {
	c := NewModelRouteCache(5*time.Second, nil)
	m := ModelKey("gpt-4o")
	p := ProviderID("openai")
	r := sampleRoute(m)

	if got := c.Get(m, p); got != nil {
		t.Fatalf("expected nil before set, got %v", got)
	}

	c.Set(m, p, r)
	got := c.Get(m, p)
	if got != r {
		t.Fatalf("expected cached route %p, got %p", r, got)
	}
	if got := c.Get(m, p); got == nil {
		t.Fatal("expected cached route still in cache")
	}
}

func TestModelRouteCache_Expiration(t *testing.T) {
	c := NewModelRouteCache(50*time.Millisecond, nil)
	m := ModelKey("gpt-4o")
	p := ProviderID("openai")
	c.Set(m, p, sampleRoute(m))

	if got := c.Get(m, p); got == nil {
		t.Fatal("expected cache hit before expiration")
	}

	time.Sleep(60 * time.Millisecond)
	if got := c.Get(m, p); got != nil {
		t.Fatalf("expected cache miss after expiration, got %v", got)
	}
}

func TestModelRouteCache_DifferentKeys(t *testing.T) {
	c := NewModelRouteCache(5*time.Second, nil)
	r1 := sampleRoute(ModelKey("m1"))
	r2 := sampleRoute(ModelKey("m2"))
	c.Set(ModelKey("m1"), ProviderID("p1"), r1)
	c.Set(ModelKey("m2"), ProviderID("p2"), r2)

	if got := c.Get(ModelKey("m1"), ProviderID("p1")); got != r1 {
		t.Fatalf("expected r1, got %p", got)
	}
	if got := c.Get(ModelKey("m2"), ProviderID("p2")); got != r2 {
		t.Fatalf("expected r2, got %p", got)
	}
	if got := c.Get(ModelKey("m1"), ProviderID("p2")); got != nil {
		t.Fatalf("expected nil for different provider, got %p", got)
	}
}

func TestModelRouteCache_Clear(t *testing.T) {
	c := NewModelRouteCache(5*time.Second, nil)
	c.Set(ModelKey("m1"), ProviderID("p1"), sampleRoute(ModelKey("m1")))
	c.Set(ModelKey("m2"), ProviderID("p2"), sampleRoute(ModelKey("m2")))

	c.Clear()
	if got := c.Get(ModelKey("m1"), ProviderID("p1")); got != nil {
		t.Fatalf("expected nil after clear, got %p", got)
	}
	if got := c.Get(ModelKey("m2"), ProviderID("p2")); got != nil {
		t.Fatalf("expected nil after clear, got %p", got)
	}
}

func TestModelRouteCache_Disabled(t *testing.T) {
	c := NewModelRouteCache(0, nil)
	c.Set(ModelKey("m1"), ProviderID("p1"), sampleRoute(ModelKey("m1")))
	if got := c.Get(ModelKey("m1"), ProviderID("p1")); got != nil {
		t.Fatalf("expected nil when disabled, got %p", got)
	}
	if got := c.Get(ModelKey("m1"), ProviderID("p1")); got != nil {
		t.Fatalf("expected nil when disabled, got %p", got)
	}
}
