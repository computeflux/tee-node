package tailscale

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// StaticRoute defines a fixed routing rule.
type StaticRoute struct {
	ChainID string `json:"chain_id" yaml:"chain_id"`
	Service string `json:"service" yaml:"service"`
	Target  string `json:"target" yaml:"target"`
	Region  string `json:"region,omitempty" yaml:"region,omitempty"`
}

// Router decides which tailnet node handles a given request.
type Router interface {
	// Route returns the Tailscale host:port for a chain+service, preferring nodes
	// in the given region when possible. Falls back to any region.
	Route(chainID, service, region string) (string, error)

	// AllTargets returns all known targets for pre-connection.
	AllTargets() []string
}

// RouteTable implements Router using a configurable routing table.
//
// Resolution priority:
//  1. Static route with exact region match
//  2. Static route for chain+service (any region)
//  3. Discovered peer in preferred region
//  4. Discovered peer in any region
//  5. Fallback
type RouteTable struct {
	staticByRegion map[string]string   // "chainID:service:region" → target
	static         map[string]string   // "chainID:service" → target
	staticTargets  map[string]struct{} // deduped set of all static targets
	fallback       string
	node           *Node
	mu             sync.RWMutex
}

// NewRouteTable creates a new routing table.
func NewRouteTable(node *Node) *RouteTable {
	return &RouteTable{
		staticByRegion: make(map[string]string),
		static:         make(map[string]string),
		staticTargets:  make(map[string]struct{}),
		node:           node,
	}
}

// AddStatic adds a static routing rule.
func (rt *RouteTable) AddStatic(chainID, service, target string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := routeKey(chainID, service)
	rt.static[key] = target
	rt.staticTargets[target] = struct{}{}
	log.Printf("[tailscale-router] static route: %s → %s", key, target)
}

// AddStaticWithRegion adds a region-specific static route (higher priority).
func (rt *RouteTable) AddStaticWithRegion(chainID, service, region, target string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := routeKey(chainID, service, region)
	rt.staticByRegion[key] = target
	rt.staticTargets[target] = struct{}{}
	log.Printf("[tailscale-router] static route: %s:%s:%s → %s", chainID, service, region, target)
}

// AddStaticRoutes adds multiple static routes.
func (rt *RouteTable) AddStaticRoutes(routes []StaticRoute) {
	for _, r := range routes {
		if r.Region != "" {
			rt.AddStaticWithRegion(r.ChainID, r.Service, r.Region, r.Target)
		} else {
			rt.AddStatic(r.ChainID, r.Service, r.Target)
		}
	}
}

// SetFallback sets a fallback target for unmatched routes.
func (rt *RouteTable) SetFallback(target string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.fallback = target
	rt.staticTargets[target] = struct{}{}
	log.Printf("[tailscale-router] fallback set to %s", target)
}

// AllTargets returns all known targets for pre-connection warming.
func (rt *RouteTable) AllTargets() []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	targets := make([]string, 0, len(rt.staticTargets))
	for t := range rt.staticTargets {
		targets = append(targets, t)
	}

	// Also collect from discovered peers
	if rt.node != nil {
		rt.node.mu.RLock()
		for ip, services := range rt.node.peers {
			for _, s := range services {
				target := fmt.Sprintf("%s:%d", ip, s.Port)
				targets = append(targets, target)
			}
		}
		rt.node.mu.RUnlock()
	}

	return targets
}

// Route resolves a chainID+service to a Tailscale host:port.
func (rt *RouteTable) Route(chainID, service, region string) (string, error) {
	// 1. Exact region match in static routes
	if region != "" {
		rt.mu.RLock()
		regionKey := routeKey(chainID, service, region)
		if target, ok := rt.staticByRegion[regionKey]; ok {
			rt.mu.RUnlock()
			return target, nil
		}
		rt.mu.RUnlock()
	}

	// 2. Static route for chain+service (any region)
	rt.mu.RLock()
	key := routeKey(chainID, service)
	if target, ok := rt.static[key]; ok {
		rt.mu.RUnlock()
		return target, nil
	}
	rt.mu.RUnlock()

	// 3-4. Discovered peers
	if rt.node != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := rt.node.RefreshPeers(ctx); err == nil {
			// Prefer same region, then any region
			targets := rt.findMatchingPeers(chainID, service, region)
			if len(targets) > 0 {
				t := rt.pickBest(targets)
				return fmt.Sprintf("%s:%d", t.IP, t.Port), nil
			}
		}
	}

	// 5. Fallback
	rt.mu.RLock()
	fallback := rt.fallback
	rt.mu.RUnlock()
	if fallback != "" {
		return fallback, nil
	}

	return "", fmt.Errorf("no route for chain=%s service=%s region=%s", chainID, service, region)
}

// pickBest selects the best peer: prefer same region, then least loaded (random for now).
func (rt *RouteTable) pickBest(targets []peerTarget) peerTarget {
	if len(targets) == 1 {
		return targets[0]
	}

	// Separate same-region from other-region
	var sameRegion, otherRegion []peerTarget
	for _, t := range targets {
		if t.SameRegion {
			sameRegion = append(sameRegion, t)
		} else {
			otherRegion = append(otherRegion, t)
		}
	}

	pool := sameRegion
	if len(pool) == 0 {
		pool = otherRegion
	}

	// Sort by priority (lower = better), then randomize within same priority
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].Priority != pool[j].Priority {
			return pool[i].Priority < pool[j].Priority
		}
		return false // stable sort
	})

	// Random pick from top-priority group to distribute load
	topPriority := pool[0].Priority
	top := make([]peerTarget, 0)
	for _, t := range pool {
		if t.Priority == topPriority {
			top = append(top, t)
		}
	}

	return top[rand.Intn(len(top))]
}

func routeKey(parts ...string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ":"
		}
		result += p
	}
	return result
}

type peerTarget struct {
	IP         string
	Port       int
	Priority   int
	SameRegion bool
}

func (rt *RouteTable) findMatchingPeers(chainID, service, preferredRegion string) []peerTarget {
	rt.node.mu.RLock()
	defer rt.node.mu.RUnlock()

	var targets []peerTarget
	for ip, services := range rt.node.peers {
		for _, s := range services {
			if s.ChainID == chainID && s.Service == service {
				targets = append(targets, peerTarget{
					IP:         ip,
					Port:       s.Port,
					Priority:   s.Priority,
					SameRegion: preferredRegion != "" && s.Region == preferredRegion,
				})
			}
		}
	}
	return targets
}
