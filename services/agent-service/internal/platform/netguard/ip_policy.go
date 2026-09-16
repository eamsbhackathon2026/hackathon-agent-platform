package netguard

import "net/netip"

// These ranges never become reachable through development mode or allowlists.
// This includes metadata endpoints, link-local and address translation/tunnels.
var restricted = prefixes(
	"0.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "240.0.0.0/4",
	"100.100.100.200/32", "168.63.129.16/32", "fd00:ec2::254/128",
	"::/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64",
	"2001::/23", "2002::/16", "fe80::/10", "fec0::/10", "ff00::/8",
)

var nonPublic = prefixes(
	"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32", "3fff::/20",
)

func prefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(values))
	for i, value := range values {
		result[i] = netip.MustParsePrefix(value)
	}
	return result
}

func contains(ranges []netip.Prefix, ip netip.Addr) bool {
	for _, prefix := range ranges {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (g *Guard) permitsIP(host string, ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	// ::1 is a deliberate local-provider exception to the obsolete ::/96 range.
	if !ip.IsLoopback() && contains(restricted, ip) {
		return false
	}
	if contains(g.prefixes, ip) {
		return true
	}
	if ip.IsPrivate() || ip.IsLoopback() {
		return g.development || g.hosts[canonicalHost(host)]
	}
	if contains(nonPublic, ip) {
		return false
	}
	return ip.IsGlobalUnicast() && (ip.Is4() || netip.MustParsePrefix("2000::/3").Contains(ip))
}
