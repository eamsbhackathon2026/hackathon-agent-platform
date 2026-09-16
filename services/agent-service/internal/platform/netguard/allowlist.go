// Package netguard validates outbound destinations and dials only checked IPs.
package netguard

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
)

// Resolver supplies DNS answers; nil Config.Resolver uses the system resolver.
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// Config controls local-provider access and administrator-approved destinations.
type Config struct {
	Development bool
	Allowlist   []string
	Resolver    Resolver
}

// Guard enforces a fixed egress policy across URL validation and HTTP clients.
type Guard struct {
	development bool
	hosts       map[string]bool
	prefixes    []netip.Prefix
	resolver    Resolver
}

// New validates administrator-supplied exact hostnames and CIDRs at startup.
func New(cfg Config) (*Guard, error) {
	g := &Guard{development: cfg.Development, hosts: make(map[string]bool), resolver: cfg.Resolver}
	if g.resolver == nil {
		g.resolver = net.DefaultResolver
	}
	for _, entry := range cfg.Allowlist {
		entry = strings.TrimSpace(entry)
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err != nil || prefix.Addr().Is4In6() {
				return nil, errors.New("invalid egress allowlist CIDR")
			}
			g.prefixes = append(g.prefixes, prefix.Masked())
			continue
		}
		if _, err := netip.ParseAddr(entry); err == nil || !validHostname(entry) {
			return nil, errors.New("invalid egress allowlist hostname; use CIDR for IP addresses")
		}
		g.hosts[canonicalHost(entry)] = true
	}
	return g, nil
}

func canonicalHost(host string) string { return strings.ToLower(strings.TrimSuffix(host, ".")) }

func validHostname(host string) bool {
	host = canonicalHost(host)
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	hasLetter := false
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if c >= 'a' && c <= 'z' {
				hasLetter = true
			} else if (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return hasLetter
}
