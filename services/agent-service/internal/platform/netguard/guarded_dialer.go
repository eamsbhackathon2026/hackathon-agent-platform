package netguard

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
)

// NewHTTPClient disables proxies and redirects. Callers own streaming deadlines.
func (g *Guard) NewHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: g.dialContext, ForceAttemptHTTP2: true,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout: 90 * time.Second, ExpectContinueTimeout: time.Second,
		MaxIdleConns: 100, MaxIdleConnsPerHost: 10,
	}
	return &http.Client{
		Transport:     &guardedTransport{guard: g, transport: transport},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type guardedTransport struct {
	guard     *Guard
	transport *http.Transport
}

func (t *guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, domain.ErrProviderBadRequest
	}
	// SDK-generated query parameters are permitted, unlike stored base URLs.
	if err := t.guard.validateParsedURL(req.URL); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

func (t *guardedTransport) CloseIdleConnections() { t.transport.CloseIdleConnections() }

func (g *Guard) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, domain.ErrProviderBadRequest
	}
	ips := []netip.Addr{}
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		ips = append(ips, ip)
	} else {
		ips, err = g.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, domain.ErrProviderUnreachable
		}
	}
	if len(ips) == 0 {
		return nil, domain.ErrProviderUnreachable
	}
	// Validate the entire answer before any connection, including mixed answers.
	for _, ip := range ips {
		if !g.permitsIP(host, ip) {
			return nil, domain.ErrEgressDenied
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range ips {
		// Dial an IP literal; Transport retains the URL hostname for TLS SNI.
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
		if dialErr == nil {
			return conn, nil
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, domain.ErrProviderUnreachable
}
