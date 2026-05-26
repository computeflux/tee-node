package tailscale

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FullConfig is the complete configuration for a Tailscale-enabled TEE node.
type FullConfig struct {
	// Tailscale connection settings.
	Tailscale Config `json:"tailscale" yaml:"tailscale"`

	// Proxy settings.
	Proxy ProxyConfig `json:"proxy" yaml:"proxy"`

	// Static routing rules.
	Routes []StaticRoute `json:"routes" yaml:"routes"`

	// Fallback target for unmatched routes.
	Fallback string `json:"fallback" yaml:"fallback"`

	// Local services this node advertises.
	Services []ServiceTag `json:"services" yaml:"services"`
}

// DefaultConfig returns a minimal default configuration.
func DefaultConfig(hostname string) FullConfig {
	return FullConfig{
		Tailscale: Config{
			Hostname:  hostname,
			Ephemeral: false,
		},
		Proxy: DefaultProxyConfig(),
	}
}

// LoadConfig reads configuration from a JSON or YAML file.
func LoadConfig(path string) (*FullConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &FullConfig{}

	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, cfg); err == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config (yaml/json): %w", err)
	}
	return cfg, nil
}

// Setup creates a fully configured Tailscale node + proxy + router
// from a FullConfig and starts the proxy.
func Setup(cfg *FullConfig) (*Node, *Proxy, error) {
	// 1. Create router
	rt := NewRouteTable(nil) // nil node — will be set after node is created

	// 2. Create Tailscale node
	node, err := New(cfg.Tailscale, rt)
	if err != nil {
		return nil, nil, fmt.Errorf("tailscale: %w", err)
	}

	// 3. Wire up node to router for peer discovery
	rt.node = node

	// 4. Add static routes
	rt.AddStaticRoutes(cfg.Routes)
	if cfg.Fallback != "" {
		rt.SetFallback(cfg.Fallback)
	}

	// 5. Advertise local services
	node.Advertise(cfg.Services)

	// 6. Create proxy
	proxy := NewProxy(cfg.Proxy, node, rt)

	return node, proxy, nil
}

// SetupAndServe is a convenience function that sets up and starts the proxy.
// It blocks until the proxy stops.
func SetupAndServe(cfg *FullConfig) error {
	_, proxy, err := Setup(cfg)
	if err != nil {
		return err
	}
	return proxy.Start()
}
