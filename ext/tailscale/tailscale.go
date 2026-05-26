// Package tailscale provides Tailscale-based cross-region traffic forwarding
// for blockchain TEE nodes.
//
// Architecture:
//
//	┌─────────────┐         ┌─────────────┐
//	│  Region A    │  TS VPN │  Region B    │
//	│  ┌─────────┐ │◄──────►│  ┌─────────┐ │
//	│  │ TEE Node│ │        │  │ TEE Node│ │
//	│  │ :3000   │ │        │  │ :4000   │ │
//	│  └────┬────┘ │        │  └────┬────┘ │
//	│       │      │        │       │      │
//	│  ┌────▼────┐ │        │       │      │
//	│  │  Proxy  │ │        │       │      │
//	│  │ :8080   ├─┼────────┼───────┘      │
//	│  └─────────┘ │        │              │
//	└─────────────┘        └─────────────┘
//
// User → Node A :8080 → Proxy routes to Node B :4000 via Tailscale mesh.
//
// The proxy inspects request headers (X-Chain-Id, X-Service) to determine
// the target node, then forwards via Tailscale IP.
package tailscale

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"tailscale.com/client/tailscale"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

// Config holds the Tailscale setup configuration.
type Config struct {
	// AuthKey is a pre-authorized Tailscale auth key (tskey-auth-...).
	// If empty, uses interactive login.
	AuthKey string `json:"auth_key" yaml:"auth_key"`

	// Hostname for this node on the tailnet (e.g. "tee-node-us-east").
	Hostname string `json:"hostname" yaml:"hostname"`

	// ControlURL is the Tailscale coordination server (default: https://controlplane.tailscale.com).
	ControlURL string `json:"control_url" yaml:"control_url"`

	// Ephemeral removes the node from tailnet on shutdown if true.
	Ephemeral bool `json:"ephemeral" yaml:"ephemeral"`

	// Routes are subnet routes this node advertises (CIDR strings).
	Routes []string `json:"routes,omitempty" yaml:"routes,omitempty"`
}

// ServiceTag advertises what blockchain services this node provides.
type ServiceTag struct {
	ChainID  string `json:"chain_id"` // e.g. "polkadot", "ethereum", "0x1"
	Service  string `json:"service"`  // "rpc", "graphql", "p2p", "indexer"
	Port     int    `json:"port"`     // local port the service listens on
	Region   string `json:"region"`   // "us-east", "eu-west", "ap-southeast"
	Priority int    `json:"priority"` // lower = higher priority for routing
}

// Node represents this TEE node's Tailscale connection.
type Node struct {
	cfg    Config
	server *tsnet.Server
	lc     *tailscale.LocalClient
	status *ipnstate.Status
	mu     sync.RWMutex

	// Discovered peers and their services
	peers map[string][]ServiceTag // TailscaleIP → services

	// Local services this node advertises
	localServices []ServiceTag

	// Router for traffic forwarding
	router Router
}

// New creates a new Tailscale node and connects to the tailnet.
func New(cfg Config, router Router) (*Node, error) {
	if cfg.Hostname == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("hostname required: %w", err)
		}
		cfg.Hostname = host
	}

	srv := &tsnet.Server{
		Hostname:   cfg.Hostname,
		AuthKey:    cfg.AuthKey,
		ControlURL: cfg.ControlURL,
		Ephemeral:  cfg.Ephemeral,
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("tailscale start: %w", err)
	}

	lc, err := srv.LocalClient()
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("tailscale local client: %w", err)
	}

	n := &Node{
		cfg:    cfg,
		server: srv,
		lc:     lc,
		router: router,
		peers:  make(map[string][]ServiceTag),
	}

	log.Printf("[tailscale] connected as %s", cfg.Hostname)
	return n, nil
}

// Close shuts down the Tailscale connection.
func (n *Node) Close() error {
	return n.server.Close()
}

// LocalIP returns this node's Tailscale IP address (100.x.x.x).
func (n *Node) LocalIP() (net.IP, error) {
	status, err := n.lc.Status(context.Background())
	if err != nil {
		return nil, err
	}
	return net.ParseIP(status.Self.TailscaleIPs[0].String()), nil
}

// LocalHostname returns this node's Tailscale hostname.
func (n *Node) LocalHostname() string {
	return n.cfg.Hostname
}

// Advertise registers local services so other nodes can discover them.
func (n *Node) Advertise(services []ServiceTag) {
	n.mu.Lock()
	n.localServices = services
	n.mu.Unlock()

	for _, s := range services {
		log.Printf("[tailscale] advertising: chain=%s service=%s port=%d region=%s",
			s.ChainID, s.Service, s.Port, s.Region)
	}
}

// RefreshPeers updates the peer list from Tailscale status.
func (n *Node) RefreshPeers(ctx context.Context) error {
	st, err := n.lc.Status(ctx)
	if err != nil {
		return err
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = st

	// Clear old peer data
	n.peers = make(map[string][]ServiceTag)

	for _, peer := range st.Peer {
		ip := peer.TailscaleIPs[0].String()
		// Parse services from peer's advertised tags (if any)
		// In production, this would use a service discovery protocol
		n.peers[ip] = parseServiceTags(peer.Tags.AsSlice())
	}

	return nil
}

// parseServiceTags extracts service tags from Tailscale node tags.
// Tags are stored as "tag:chain-<chainID>" and "tag:svc-<service>".
func parseServiceTags(tags []string) []ServiceTag {
	var services []ServiceTag
	var chainID, svc string

	for _, tag := range tags {
		// Parse structured tags like "chain:polkadot" "svc:rpc"
		parts := splitTag(tag)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "chain":
			chainID = parts[1]
		case "svc":
			svc = parts[1]
		}
	}

	if chainID != "" && svc != "" {
		services = append(services, ServiceTag{
			ChainID: chainID,
			Service: svc,
		})
	}

	return services
}

func splitTag(tag string) []string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == ':' {
			return []string{tag[:i], tag[i+1:]}
		}
	}
	return nil
}
