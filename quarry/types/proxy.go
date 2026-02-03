// Package types defines core domain types for the Quarry runtime.
package types

import "fmt"

// ProxyProtocol is the allowed proxy protocol.
// Note: socks5 is best-effort with Puppeteer.
type ProxyProtocol string

const (
	ProxyProtocolHTTP   ProxyProtocol = "http"
	ProxyProtocolHTTPS  ProxyProtocol = "https"
	ProxyProtocolSOCKS5 ProxyProtocol = "socks5"
)

// ProxyStrategy is the proxy selection strategy for pools.
type ProxyStrategy string

const (
	ProxyStrategyRoundRobin ProxyStrategy = "round_robin"
	ProxyStrategyRandom     ProxyStrategy = "random"
	ProxyStrategySticky     ProxyStrategy = "sticky"
)

// ProxyStickyScope determines what key is used for sticky assignment.
type ProxyStickyScope string

const (
	ProxyStickyJob    ProxyStickyScope = "job"
	ProxyStickyDomain ProxyStickyScope = "domain"
	ProxyStickyOrigin ProxyStickyScope = "origin"
)

// ProxyEndpoint is a resolved proxy endpoint the executor can dial.
// Emitted by runtime in run requests.
type ProxyEndpoint struct {
	// Protocol is the proxy protocol.
	Protocol ProxyProtocol `json:"protocol" msgpack:"protocol"`
	// Host is the proxy host.
	Host string `json:"host" msgpack:"host"`
	// Port is the proxy port (1-65535).
	Port int `json:"port" msgpack:"port"`
	// Username is the optional username for authentication.
	Username *string `json:"username,omitempty" msgpack:"username,omitempty"`
	// Password is the optional password for authentication.
	Password *string `json:"password,omitempty" msgpack:"password,omitempty"`
}

// Validate validates a proxy endpoint per CONTRACT_PROXY.md hard validation rules.
func (p *ProxyEndpoint) Validate() error {
	// Protocol validation
	switch p.Protocol {
	case ProxyProtocolHTTP, ProxyProtocolHTTPS, ProxyProtocolSOCKS5:
		// valid
	default:
		return fmt.Errorf("invalid protocol %q: must be http, https, or socks5", p.Protocol)
	}

	// Port validation
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", p.Port)
	}

	// Auth pair validation
	hasUsername := p.Username != nil && *p.Username != ""
	hasPassword := p.Password != nil && *p.Password != ""
	if hasUsername != hasPassword {
		return fmt.Errorf("username and password must be provided together")
	}

	return nil
}

// Redact returns a copy of the endpoint without the password.
func (p *ProxyEndpoint) Redact() ProxyEndpointRedacted {
	return ProxyEndpointRedacted{
		Protocol: p.Protocol,
		Host:     p.Host,
		Port:     p.Port,
		Username: p.Username,
	}
}

// ProxyEndpointRedacted is a proxy endpoint without password.
// Used in run results per CONTRACT_PROXY.md.
type ProxyEndpointRedacted struct {
	Protocol ProxyProtocol `json:"protocol" msgpack:"protocol"`
	Host     string        `json:"host" msgpack:"host"`
	Port     int           `json:"port" msgpack:"port"`
	Username *string       `json:"username,omitempty" msgpack:"username,omitempty"`
}

// ProxySticky is sticky configuration for a proxy pool.
type ProxySticky struct {
	// Scope is the scope for sticky key derivation.
	Scope ProxyStickyScope `json:"scope" msgpack:"scope"`
	// TTLMs is the optional TTL in milliseconds for sticky entries.
	TTLMs *int64 `json:"ttl_ms,omitempty" msgpack:"ttl_ms,omitempty"`
}

// ProxyPool defines a pool and rotation policy.
// Parsed and validated by runtime.
type ProxyPool struct {
	// Name is the pool name (unique identifier).
	Name string `json:"name" msgpack:"name"`
	// Strategy is the selection strategy.
	Strategy ProxyStrategy `json:"strategy" msgpack:"strategy"`
	// Endpoints is the list of available endpoints (must have at least one).
	Endpoints []ProxyEndpoint `json:"endpoints" msgpack:"endpoints"`
	// Sticky is the optional sticky configuration.
	Sticky *ProxySticky `json:"sticky,omitempty" msgpack:"sticky,omitempty"`
}

// Validate validates a proxy pool per CONTRACT_PROXY.md hard validation rules.
func (p *ProxyPool) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("pool name is required")
	}

	switch p.Strategy {
	case ProxyStrategyRoundRobin, ProxyStrategyRandom, ProxyStrategySticky:
		// valid
	default:
		return fmt.Errorf("invalid strategy %q: must be round_robin, random, or sticky", p.Strategy)
	}

	if len(p.Endpoints) == 0 {
		return fmt.Errorf("pool must have at least one endpoint")
	}

	for i, ep := range p.Endpoints {
		if err := ep.Validate(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
		}
	}

	if p.Sticky != nil {
		switch p.Sticky.Scope {
		case ProxyStickyJob, ProxyStickyDomain, ProxyStickyOrigin:
			// valid
		default:
			return fmt.Errorf("invalid sticky scope %q: must be job, domain, or origin", p.Sticky.Scope)
		}

		if p.Sticky.TTLMs != nil && *p.Sticky.TTLMs <= 0 {
			return fmt.Errorf("sticky TTL must be positive")
		}
	}

	return nil
}

// JobProxyRequest is a job-level proxy selection request.
type JobProxyRequest struct {
	// Pool is the pool name to select from.
	Pool string `json:"pool" msgpack:"pool"`
	// Strategy is the optional strategy override.
	Strategy *ProxyStrategy `json:"strategy,omitempty" msgpack:"strategy,omitempty"`
	// StickyKey is the optional sticky key override.
	StickyKey *string `json:"sticky_key,omitempty" msgpack:"sticky_key,omitempty"`
}

// LargePoolThreshold is the number of endpoints above which round_robin
// is discouraged in favor of random.
const LargePoolThreshold = 50

// Warnings returns soft warnings per CONTRACT_PROXY.md.
// These are non-fatal issues that should be surfaced to users.
func (p *ProxyEndpoint) Warnings() []string {
	var warnings []string

	// socks5 is best-effort with Puppeteer
	if p.Protocol == ProxyProtocolSOCKS5 {
		warnings = append(warnings, "socks5 protocol is best-effort with Puppeteer; consider http or https for reliable proxy support")
	}

	return warnings
}

// Warnings returns soft warnings per CONTRACT_PROXY.md.
// These are non-fatal issues that should be surfaced to users.
func (p *ProxyPool) Warnings() []string {
	var warnings []string

	// Very large endpoint lists with round_robin
	if p.Strategy == ProxyStrategyRoundRobin && len(p.Endpoints) > LargePoolThreshold {
		warnings = append(warnings, fmt.Sprintf("pool %q has %d endpoints with round_robin strategy; consider random for large pools", p.Name, len(p.Endpoints)))
	}

	// Check endpoint warnings
	hasSocks5 := false
	for _, ep := range p.Endpoints {
		if ep.Protocol == ProxyProtocolSOCKS5 {
			hasSocks5 = true
			break
		}
	}
	if hasSocks5 {
		warnings = append(warnings, fmt.Sprintf("pool %q contains socks5 endpoints; socks5 is best-effort with Puppeteer", p.Name))
	}

	return warnings
}
