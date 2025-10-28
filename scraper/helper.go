package scraper

import (
	"net/netip"
	"strings"
	"time"
)

// CreateHostmapForTesting creates a Hostmap instance for testing
// This is exported so it can be used in tests outside the package
func CreateHostmapForTesting(ip netip.Addr, hostnames []string, lastseen time.Time) *Hostmap {
	return &Hostmap{
		ip:          ip,
		hostnames:   hostnames,
		lastseen:    lastseen,
		removalCode: NotRemoved,
	}
}

// GetIP returns the IP address of the Hostmap
func (h *Hostmap) GetIP() netip.Addr {
	return h.ip
}

// GetHostnames returns the hostnames of the Hostmap
func (h *Hostmap) GetHostnames() []string {
	return h.hostnames
}

// GetRemovalCode is already defined in mock.go

// normalizeDNSName ensures a DNS name has a trailing dot (for FQDN format)
// This is required for PowerDNS CNAME content fields
func normalizeDNSName(name string) string {
	if name == "" {
		return name
	}
	if !strings.HasSuffix(name, ".") {
		return name + "."
	}
	return name
}

// normalizeHostname strips the trailing dot from a DNS name for lookups
// This ensures consistent map lookups regardless of whether the input has a trailing dot
func normalizeHostname(name string) string {
	return strings.TrimSuffix(name, ".")
}
