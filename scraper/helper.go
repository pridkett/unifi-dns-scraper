package scraper

import (
	"net/netip"
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
