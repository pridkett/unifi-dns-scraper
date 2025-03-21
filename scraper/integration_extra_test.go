package scraper

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	logm "github.com/withmandala/go-log"
)

// TestHostnameExclusivity integrates the logic previously found in test-hostname-exclusivity.go.
func TestHostnameExclusivity(t *testing.T) {
	log := logm.New(os.Stdout).WithColor() // use standard out logging
	SetLogger(log)

	cfg := testConfigForHostnameExclusivity()

	hostmaps := testHostmapsForHostnameExclusivity()

	processed := ResolveAdditionalHostConflicts(hostmaps, cfg)

	// Verify that the Additional entry for "unifi" (with IP 192.168.151.1)
	// took precedence over the UDM-discovered "unifi" at 192.168.1.1.
	var hasConfigUnifi, hasUDMUnifi bool
	for _, hm := range processed {
		for _, h := range hm.hostnames {
			if h == "unifi" {
				// Determine IP address type based on our test hosts:
				if hm.ip.String() == "192.168.151.1" {
					hasConfigUnifi = true
				} else if hm.ip.String() == "192.168.1.1" {
					hasUDMUnifi = true
				}
			}
		}
	}

	if !hasConfigUnifi {
		t.Errorf("Expected Additional entry for 'unifi' (192.168.151.1) to be present")
	}
	if hasUDMUnifi {
		t.Errorf("UDM-discovered 'unifi' (192.168.1.1) should have been removed")
	}

	t.Log("TestHostnameExclusivity passed")
}

// testConfigForHostnameExclusivity mimics the configuration setup in test-hostname-exclusivity.go.
func testConfigForHostnameExclusivity() *TomlConfig {
	var cfg TomlConfig
	// Set domains for FQDN generation
	cfg.Processing.Domains = []string{"example.com", "local"}

	// Define Additional entries
	additional1 := struct {
		IP           string
		Hostnames    []string
		Name         string
		KeepMultiple *bool
	}{
		IP:           "192.168.151.1",
		Name:         "unifi",
		KeepMultiple: nil,
	}
	additional2 := struct {
		IP           string
		Hostnames    []string
		Name         string
		KeepMultiple *bool
	}{
		IP:           "192.168.151.10",
		Name:         "printer",
		KeepMultiple: nil,
	}
	var keepFalse = false
	additional3 := struct {
		IP           string
		Hostnames    []string
		Name         string
		KeepMultiple *bool
	}{
		IP:           "192.168.151.20",
		Name:         "server",
		KeepMultiple: &keepFalse,
	}

	cfg.Processing.Additional = append(cfg.Processing.Additional, additional1, additional2, additional3)
	return &cfg
}

// testHostmapsForHostnameExclusivity creates test hostmaps as in test-hostname-exclusivity.go.
func testHostmapsForHostnameExclusivity() []*Hostmap {
	// UDM-discovered hosts
	unifiIP, _ := netip.ParseAddr("192.168.1.1")
	printerIP, _ := netip.ParseAddr("192.168.1.10")
	serverIP, _ := netip.ParseAddr("192.168.1.20")
	unifiHost := CreateHostmapForTesting(unifiIP, []string{"unifi"}, time.Now())
	printerHost := CreateHostmapForTesting(printerIP, []string{"printer"}, time.Now())
	serverHost := CreateHostmapForTesting(serverIP, []string{"server"}, time.Now())

	// Additional entries as specified in the config
	configUnifiIP, _ := netip.ParseAddr("192.168.151.1")
	configPrinterIP, _ := netip.ParseAddr("192.168.151.10")
	configServerIP, _ := netip.ParseAddr("192.168.151.20")
	configUnifiHost := CreateHostmapForTesting(configUnifiIP, []string{"unifi"}, time.Now())
	configPrinterHost := CreateHostmapForTesting(configPrinterIP, []string{"printer"}, time.Now())
	configServerHost := CreateHostmapForTesting(configServerIP, []string{"server"}, time.Now())

	return []*Hostmap{
		unifiHost,
		printerHost,
		serverHost,
		configUnifiHost,
		configPrinterHost,
		configServerHost,
	}
}

// TestKeepMacsIntegration integrates logic from test-keep-macs.go and test-mac-handling.go,
// verifying the behavior of MAC address handling based on the KeepMacs configuration.
func TestKeepMacsIntegration(t *testing.T) {
	// Simulate two configurations: one with KeepMacs false and one with true.
	var cfgFalse, cfgTrue TomlConfig

	cfgFalse.Processing.KeepMacs = false
	cfgTrue.Processing.KeepMacs = true

	// Create sample hostmaps similar to those in the standalone tests.
	// Updated host1: add an extra non-MAC hostname so that valid hosts are processed without removal,
	// and pure MAC-only hosts are expected to be marked as removed.
	host1 := &Hostmap{
		ip:          mustParseIP("192.168.1.1"),
		hostnames:   []string{"00:11:22:33:44:55", "host1"},
		lastseen:    time.Now(),
		removalCode: NotRemoved,
	}
	host2 := &Hostmap{
		ip:          mustParseIP("192.168.1.2"),
		hostnames:   []string{"regular-host"},
		lastseen:    time.Now(),
		removalCode: NotRemoved,
	}
	// host3 remains the same (mixed: one MAC and one non-MAC, so valid)
	host3 := &Hostmap{
		ip:          mustParseIP("192.168.1.3"),
		hostnames:   []string{"00:aa:bb:cc:dd:ee", "mixed-host"},
		lastseen:    time.Now(),
		removalCode: NotRemoved,
	}
	// Additionally, add a pure MAC-only host (host4) to test removal.
	host4 := &Hostmap{
		ip:          mustParseIP("192.168.1.4"),
		hostnames:   []string{"11:22:33:44:55:66"},
		lastseen:    time.Now(),
		removalCode: NotRemoved,
	}
	hostmaps := []*Hostmap{host1, host2, host3, host4}

	// Process hostmaps with both configurations.
	resultFalse := processMACHostnames(copyHostmaps(hostmaps), &cfgFalse)
	resultTrue := processMACHostnames(copyHostmaps(hostmaps), &cfgTrue)

	// For KeepMacs=false:
	// If a host's hostnames are purely MAC addresses, it should be marked as removed (removalCode == MacAddress).
	// For valid hosts (removalCode != MacAddress), none of the hostnames should be MAC addresses.
	validCount := 0
	for _, hm := range resultFalse {
		nonMACs := []string{}
		for _, h := range hm.hostnames {
			if !isMACAddress(h) {
				nonMACs = append(nonMACs, h)
			}
		}
		if len(nonMACs) == 0 {
			// This is a pure MAC-only host. It should be marked as removed.
			if hm.removalCode != MacAddress {
				t.Errorf("KeepMacs=false: expected pure-MAC host to be marked as removed, got removalCode: %v", hm.removalCode)
			}
		} else {
			// Valid host: Expect that the processed hostnames consist only of non-MAC values.
			if len(nonMACs) != len(hm.hostnames) {
				t.Errorf("KeepMacs=false: expected valid host not to contain MAC addresses, but found: %v", hm.hostnames)
			}
			validCount++
		}
	}
	// Expected valid hosts: host1, host2, and host3 (host4 should be removed)
	if validCount != 3 {
		t.Errorf("KeepMacs=false: expected 3 valid hosts, got %d", validCount)
	}

	// For KeepMacs=true, validate that MAC addresses are converted from colon to hyphen format.
	macConverted := false
	expectedMAC := convertMAC("00:11:22:33:44:55") // Expected conversion: colons to hyphens.
	for _, hm := range resultTrue {
		for _, h := range hm.hostnames {
			if h == expectedMAC {
				macConverted = true
			}
		}
	}
	if !macConverted {
		t.Errorf("KeepMacs=true: expected MAC addresses to be converted to hyphen format (%s)", expectedMAC)
	}

	t.Log("TestKeepMacsIntegration passed")
}

// mustParseIP parses an IP address string or panics.
func mustParseIP(ipStr string) netip.Addr {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		panic(fmt.Sprintf("invalid IP: %s", ipStr))
	}
	return ip
}

// copyHostmaps creates a deep copy of the hostmaps slice.
func copyHostmaps(hosts []*Hostmap) []*Hostmap {
	var copies []*Hostmap
	for _, h := range hosts {
		newHost := *h
		newHost.hostnames = append([]string{}, h.hostnames...)
		copies = append(copies, &newHost)
	}
	return copies
}

// isMACAddress performs a simple check to determine if a hostname is in MAC address format.
func isMACAddress(hostname string) bool {
	// A naive check: MAC addresses in standard colon format have 17 characters (e.g., "00:11:22:33:44:55")
	return len(hostname) == 17 && strings.Contains(hostname, ":")
}

// convertMAC converts a MAC address from colon-separated to hyphen-separated format.
func convertMAC(mac string) string {
	return strings.ReplaceAll(mac, ":", "-")
}
