package netguard

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

func newGuard(t *testing.T, cfg Config) *Guard {
	t.Helper()
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAllowlistValidation(t *testing.T) {
	for _, entry := range []string{"", "*.example.com", "https://example.com", "example.com:443", "user@example.com", "127.0.0.1", "10.0.0.0/99", "::ffff:10.0.0.0/104", "a..com", "-a.com", "secret?token=value"} {
		t.Run(entry, func(t *testing.T) {
			if _, err := New(Config{Allowlist: []string{entry}}); err == nil || strings.Contains(err.Error(), entry) && entry == "secret?token=value" {
				t.Fatalf("expected sanitized configuration error, got %v", err)
			}
		})
	}
	newGuard(t, Config{Allowlist: []string{" Internal.Example. ", "10.0.0.0/8", "::1/128"}})
}

func TestURLPolicy(t *testing.T) {
	g := newGuard(t, Config{Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("base URL validation must not resolve DNS")
		return nil, nil
	})})
	for _, raw := range []string{"https://example.com/v1", "https://example.com:8443/v1", "https://8.8.8.8", "https://[2606:4700:4700::1111]"} {
		if err := g.ValidateURL(raw); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "/v1", "ftp://example.com", "https://user:password@example.com", "https://example.com?key=secret", "https://example.com?", "https://example.com#fragment", "https://example.com:0", "https://example.com:65536", "https://example.com:abc", "https://example.com:", "https://2130706433", "https://127.1", "https://[fe80::1%25eth0]", "https://example.com/" + strings.Repeat("x", maxURLLength)} {
		if err := g.ValidateURL(raw); err == nil {
			t.Errorf("unexpectedly allowed %q", raw)
		} else if strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "secret") {
			t.Errorf("error contains credentials: %v", err)
		}
	}
	if !errors.Is(g.ValidateURL("http://example.com"), domain.ErrEgressDenied) {
		t.Fatal("production must require HTTPS")
	}
}

func TestIPPolicy(t *testing.T) {
	prod := newGuard(t, Config{})
	dev := newGuard(t, Config{Development: true})
	allow := newGuard(t, Config{Allowlist: []string{"trusted.example", "0.0.0.0/0", "::/0"}})
	for _, raw := range []string{"169.254.169.254", "169.254.1.1", "::ffff:169.254.169.254", "fe80::1", "100.100.100.200", "168.63.129.16", "fd00:ec2::254", "0.0.0.0", "0.1.2.3", "::", "224.0.0.1", "ff02::1", "240.0.0.1", "64:ff9b::a00:1", "64:ff9b:1::1", "2002:7f00:1::", "2001::1", "::127.0.0.1", "fec0::1"} {
		for _, g := range []*Guard{prod, dev, allow} {
			if g.permitsIP("trusted.example", netip.MustParseAddr(raw)) {
				t.Errorf("restricted IP permitted: %s", raw)
			}
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "::1", "fd12::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1"} {
		ip := netip.MustParseAddr(raw)
		if prod.permitsIP("untrusted.example", ip) || !dev.permitsIP("untrusted.example", ip) || !allow.permitsIP("trusted.example", ip) {
			t.Errorf("incorrect private IP handling: %s", raw)
		}
	}
	for _, raw := range []string{"100.64.0.1", "192.0.2.1", "198.18.0.1", "2001:db8::1"} {
		ip := netip.MustParseAddr(raw)
		if dev.permitsIP("trusted.example", ip) || prod.permitsIP("trusted.example", ip) {
			t.Errorf("non-public IP permitted by default: %s", raw)
		}
	}
	if !newGuard(t, Config{Allowlist: []string{"100.64.0.0/10"}}).permitsIP("host", netip.MustParseAddr("100.64.0.1")) {
		t.Fatal("explicit CGNAT CIDR should be usable")
	}
}

func TestExactHostTrustAndMixedDNS(t *testing.T) {
	g := newGuard(t, Config{Allowlist: []string{"trusted.example"}})
	for _, host := range []string{"unrelated.example", "sub.trusted.example", "trusted.example.attacker.test"} {
		if g.permitsIP(host, netip.MustParseAddr("10.0.0.1")) {
			t.Errorf("inherited trust for %s", host)
		}
	}
	if !g.permitsIP("TRUSTED.EXAMPLE.", netip.MustParseAddr("10.0.0.1")) {
		t.Fatal("exact canonical hostname should be allowed")
	}
	for _, answers := range [][]netip.Addr{{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, {netip.MustParseAddr("::ffff:127.0.0.1")}} {
		g.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) { return answers, nil })
		_, err := g.dialContext(context.Background(), "tcp", "unrelated.example:443")
		if !errors.Is(err, domain.ErrEgressDenied) {
			t.Fatalf("mixed/private DNS should fail before dialing: %v", err)
		}
	}
}

func TestResolverFailureAndCancellation(t *testing.T) {
	for _, resolver := range []resolverFunc{
		func(context.Context, string, string) ([]netip.Addr, error) { return nil, nil },
		func(context.Context, string, string) ([]netip.Addr, error) {
			return nil, errors.New("upstream secret details")
		},
	} {
		g := newGuard(t, Config{Resolver: resolver})
		_, err := g.dialContext(context.Background(), "tcp", "provider.example:443")
		if !errors.Is(err, domain.ErrProviderUnreachable) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected sanitized unreachable error: %v", err)
		}
	}
	g := newGuard(t, Config{Resolver: resolverFunc(func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.dialContext(ctx, "tcp", "provider.example:443"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
