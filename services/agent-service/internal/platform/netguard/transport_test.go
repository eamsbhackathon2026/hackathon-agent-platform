package netguard

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestGuardedHTTPClient(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Error("SDK query parameter lost")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	var proxyRequests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyRequests.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("ALL_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	var resolutions atomic.Int32
	g := newGuard(t, Config{Development: true, Resolver: resolverFunc(func(_ context.Context, _, host string) ([]netip.Addr, error) {
		if host != "provider.example" {
			t.Errorf("unexpected hostname %s", host)
		}
		resolutions.Add(1)
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})})
	client := g.NewHTTPClient()
	defer client.CloseIdleConnections()
	if client.Timeout != 0 {
		t.Fatal("streaming client must use caller deadlines")
	}
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	base := "http://provider.example:" + port
	for _, path := range []string{"/?alt=sse", "/redirect"} {
		res, err := client.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		if path == "/redirect" && res.StatusCode != http.StatusFound {
			t.Fatal("redirect must be returned to caller")
		}
	}
	if requests.Load() != 2 || proxyRequests.Load() != 0 || resolutions.Load() != 1 {
		t.Fatalf("requests=%d proxy=%d resolutions=%d", requests.Load(), proxyRequests.Load(), resolutions.Load())
	}
	// A fresh connection revalidates DNS instead of trusting the previous answer.
	client.CloseIdleConnections()
	g.resolver = resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
	})
	if _, err := client.Get(base); !errors.Is(err, domain.ErrEgressDenied) {
		t.Fatalf("DNS change bypassed validation: %v", err)
	}
}

func TestProductionTLSHostPreserved(t *testing.T) {
	var serverName string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverName = r.TLS.ServerName
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	// httptest's certificate includes example.com; trust its CA without disabling verification.
	g := newGuard(t, Config{Allowlist: []string{"example.com"}, Resolver: resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	})})
	client := g.NewHTTPClient()
	defer client.CloseIdleConnections()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	client.Transport.(*guardedTransport).transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	res, err := client.Get("https://example.com:" + port)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if serverName != "example.com" {
		t.Fatalf("TLS SNI changed: %s", serverName)
	}
}

func TestTransportChecksEveryRequest(t *testing.T) {
	client := newGuard(t, Config{}).NewHTTPClient()
	for _, raw := range []string{"http://example.com", "https://user:secret@example.com", "https://169.254.169.254", "https://example.com/#fragment"} {
		req, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.Transport.RoundTrip(req); err == nil {
			t.Errorf("request escaped validation: %s", raw)
		}
	}
}
