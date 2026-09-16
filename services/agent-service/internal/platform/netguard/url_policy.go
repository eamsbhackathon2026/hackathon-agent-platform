package netguard

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
)

const maxURLLength = 8192

// ValidateURL checks a stored base URL without DNS or outbound requests.
func (g *Guard) ValidateURL(raw string) error {
	if len(raw) > maxURLLength || strings.Contains(raw, "#") {
		return domain.ErrProviderBadRequest
	}
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery != "" || u.ForceQuery {
		return domain.ErrProviderBadRequest
	}
	return g.validateParsedURL(u)
}

func (g *Guard) validateParsedURL(u *url.URL) error {
	if u == nil || len(u.String()) > maxURLLength || u.Opaque != "" || u.User != nil ||
		u.Fragment != "" || u.RawFragment != "" || u.Host == "" ||
		(u.Scheme != "https" && u.Scheme != "http") {
		return domain.ErrProviderBadRequest
	}
	if u.Scheme != "https" && !g.development {
		return domain.ErrEgressDenied
	}
	if strings.HasSuffix(u.Host, ":") {
		return domain.ErrProviderBadRequest
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return domain.ErrProviderBadRequest
		}
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if !g.permitsIP(host, ip) {
			return domain.ErrEgressDenied
		}
	} else if !validHostname(host) {
		return domain.ErrProviderBadRequest
	}
	return nil
}
