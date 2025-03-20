package scraper

import (
	"net/netip"
	"testing"
	"time"
)

// Helper function to create IP addresses for testing
func createIP(ipaddr string) netip.Addr {
	ip, err := netip.ParseAddr(ipaddr)
	if err != nil {
		panic(err)
	}
	return ip
}

// Helper function to create test hostmaps
func setupTestHostmaps(t *testing.T) []*Hostmap {
	t.Helper()
	var hostmaps []*Hostmap

	hostmaps = append(hostmaps, &Hostmap{
		ip:        createIP("192.168.1.1"),
		hostnames: []string{"host1"},
		lastseen:  time.Now(),
	})
	hostmaps = append(hostmaps, &Hostmap{
		ip:        createIP("192.168.1.2"),
		hostnames: []string{"host2"},
		lastseen:  time.Now(),
	})

	return hostmaps
}

func TestRemoveOldHosts(t *testing.T) {
	tests := []struct {
		name     string
		hostmaps []*Hostmap
		want     int
		wantName string
	}{
		{
			name: "newer host replaces older host",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.2"), hostnames: []string{"host1"}, lastseen: time.Unix(time.Now().Unix()-100, 0)},
			},
			want:     1,
			wantName: "host1",
		},
		{
			name: "different hosts are kept",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.2"), hostnames: []string{"host2"}, lastseen: time.Now()},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeOldHosts(tt.hostmaps)
			if len(got) != tt.want {
				t.Errorf("removeOldHosts() got %d hosts, want %d", len(got), tt.want)
			}
			if tt.wantName != "" && len(got) > 0 {
				if got[0].hostnames[0] != tt.wantName {
					t.Errorf("removeOldHosts() got hostname %s, want %s", got[0].hostnames[0], tt.wantName)
				}
			}
		})
	}
}

func TestRemoveDuplicateHosts(t *testing.T) {
	tests := []struct {
		name     string
		hostmaps []*Hostmap
		want     int
		wantName string
	}{
		{
			name: "newer duplicate host kept",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.1"), hostnames: []string{"host2"}, lastseen: time.Unix(time.Now().Unix()-100, 0)},
			},
			want:     1,
			wantName: "host1",
		},
		{
			name: "different hosts are kept",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.2"), hostnames: []string{"host2"}, lastseen: time.Now()},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDuplicateHosts(tt.hostmaps)
			if len(got) != tt.want {
				t.Errorf("removeDuplicateHosts() got %d hosts, want %d", len(got), tt.want)
			}
			if tt.wantName != "" && len(got) > 0 {
				if got[0].hostnames[0] != tt.wantName {
					t.Errorf("removeDuplicateHosts() got hostname %s, want %s", got[0].hostnames[0], tt.wantName)
				}
			}
		})
	}
}

func TestRemoveOldHostsByTime(t *testing.T) {
	now := time.Now()

	// Test case 1: Old hosts are marked for removal but still present in the array
	t.Run("old hosts are removed", func(t *testing.T) {
		// Create test hostmaps with one recent and one old host
		hostmaps := []*Hostmap{
			{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: now, removalCode: NotRemoved},
			{ip: createIP("192.168.1.2"), hostnames: []string{"host2"}, lastseen: now.Add(-2 * time.Hour), removalCode: NotRemoved},
		}

		// Use time.Second directly because the function already multiplies by the time.Duration
		duration := time.Hour
		result := removeOldHostsByTime(hostmaps, duration)

		// The function marks hosts as removed but leaves them in the array
		if len(result) != 2 {
			t.Errorf("removeOldHostsByTime() returned %d hosts, want %d", len(result), 2)
		}

		// Count hosts by removal code
		notRemovedCount := 0
		oldCount := 0
		for _, host := range result {
			if host.removalCode == NotRemoved {
				notRemovedCount++
			} else if host.removalCode == Old {
				oldCount++
			}
		}

		// We expect the recent host to be kept and the old one marked for removal
		if notRemovedCount != 1 {
			t.Errorf("removeOldHostsByTime() kept %d hosts as NotRemoved, want %d", notRemovedCount, 1)
		}
		if oldCount != 1 {
			t.Errorf("removeOldHostsByTime() marked %d hosts as Old, want %d", oldCount, 1)
		}
	})

	// Test case 2: All hosts within time limit are kept
	t.Run("all hosts within time limit are kept", func(t *testing.T) {
		hostmaps := []*Hostmap{
			{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: now, removalCode: NotRemoved},
			{ip: createIP("192.168.1.2"), hostnames: []string{"host2"}, lastseen: now.Add(-30 * time.Minute), removalCode: NotRemoved},
		}

		duration := time.Hour
		result := removeOldHostsByTime(hostmaps, duration)

		// Check that all hosts are kept
		notRemovedCount := 0
		for _, host := range result {
			if host.removalCode == NotRemoved {
				notRemovedCount++
			}
		}

		if notRemovedCount != 2 {
			t.Errorf("removeOldHostsByTime() kept %d hosts as NotRemoved, want %d", notRemovedCount, 2)
		}
	})
}

func TestRemoveMACHosts(t *testing.T) {
	tests := []struct {
		name           string
		hostmaps       []*Hostmap
		wantCount      int
		wantValidCount int
	}{
		{
			name: "MAC addresses are removed",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"00:00:00:00:00:00"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.2"), hostnames: []string{"host2"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.3"), hostnames: []string{"00:00:00:00:00:00", "testhost"}, lastseen: time.Now()},
			},
			wantCount:      3, // Total hostmaps
			wantValidCount: 2, // Hostmaps with valid names after filtering
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeMACHosts(tt.hostmaps)

			if len(got) != tt.wantCount {
				t.Errorf("removeMACHosts() returned %d hosts, want %d", len(got), tt.wantCount)
			}

			validCount := 0
			for _, host := range got {
				if host.removalCode != MacAddress {
					validCount++
				}
			}

			if validCount != tt.wantValidCount {
				t.Errorf("removeMACHosts() kept %d valid hosts, want %d", validCount, tt.wantValidCount)
			}
		})
	}
}

func TestAddDomainsToHostmap(t *testing.T) {
	// Test with a single domain
	t.Run("single domain", func(t *testing.T) {
		hostmap := &Hostmap{
			ip:        createIP("192.168.1.1"),
			hostnames: []string{"host1"},
			lastseen:  time.Now(),
		}
		domains := []string{"example.com"}

		result := addDomainsToHostmap(hostmap, domains)

		if len(result.fqdns) != 1 {
			t.Errorf("addDomainsToHostmap() got %d FQDNs, want %d", len(result.fqdns), 1)
		}

		expectedFQDN := "host1.example.com"
		if result.fqdns[0] != expectedFQDN {
			t.Errorf("addDomainsToHostmap() got FQDN %s, want %s", result.fqdns[0], expectedFQDN)
		}
	})

	// Test with multiple domains
	t.Run("multiple domains", func(t *testing.T) {
		hostmap := &Hostmap{
			ip:        createIP("192.168.1.1"),
			hostnames: []string{"host1"},
			lastseen:  time.Now(),
		}
		domains := []string{"example.com", "local"}

		result := addDomainsToHostmap(hostmap, domains)

		if len(result.fqdns) != 2 {
			t.Errorf("addDomainsToHostmap() got %d FQDNs, want %d", len(result.fqdns), 2)
		}

		// Check each FQDN individually since we know exactly what to expect
		expectedFQDNs := []string{"host1.example.com", "host1.local"}
		for i, expected := range expectedFQDNs {
			if i < len(result.fqdns) && result.fqdns[i] != expected {
				t.Errorf("addDomainsToHostmap() got FQDN %s at index %d, want %s", result.fqdns[i], i, expected)
			}
		}
	})

	// Test with multiple hostnames and domains
	t.Run("multiple hostnames and domains", func(t *testing.T) {
		hostmap := &Hostmap{
			ip:        createIP("192.168.1.1"),
			hostnames: []string{"host1", "alias1"},
			lastseen:  time.Now(),
		}
		domains := []string{"example.com", "local"}

		result := addDomainsToHostmap(hostmap, domains)

		if len(result.fqdns) != 4 {
			t.Errorf("addDomainsToHostmap() got %d FQDNs, want %d", len(result.fqdns), 4)
		}

		// Check each FQDN individually
		expectedFQDNs := []string{
			"host1.example.com", "host1.local",
			"alias1.example.com", "alias1.local",
		}

		// Since the implementation might order them differently, we need to check that
		// all expected FQDNs exist in the result.fqdns slice, without relying on order
		for _, expected := range expectedFQDNs {
			found := false
			for _, actual := range result.fqdns {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("addDomainsToHostmap() missing expected FQDN %s", expected)
			}
		}
	})
}

func TestRemoveBlockedHosts(t *testing.T) {
	tests := []struct {
		name          string
		hostmaps      []*Hostmap
		config        *TomlConfig
		wantBlocked   int
		wantUnblocked int
	}{
		{
			name: "block by name and IP",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.1.2"), hostnames: []string{"blocked"}, lastseen: time.Now()},
			},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "192.168.1.2", Name: "blocked"},
					},
				},
			},
			wantBlocked:   1,
			wantUnblocked: 1,
		},
		{
			name: "block by IP only",
			hostmaps: []*Hostmap{
				{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
				{ip: createIP("192.168.90.2"), hostnames: []string{"powerwall"}, lastseen: time.Now()},
			},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "192.168.90.2", Name: ""},
					},
				},
			},
			wantBlocked:   1,
			wantUnblocked: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deep copy of hostmaps to avoid modifying the test data
			var hostmapsCopy []*Hostmap
			for _, h := range tt.hostmaps {
				hostmapsCopy = append(hostmapsCopy, &Hostmap{
					ip:          h.ip,
					hostnames:   append([]string{}, h.hostnames...),
					lastseen:    h.lastseen,
					removalCode: h.removalCode,
				})
			}

			got := removeBlockedHosts(hostmapsCopy, tt.config)

			blockedCount := 0
			unblockedCount := 0
			for _, host := range got {
				if host.removalCode == Blocked {
					blockedCount++
				} else {
					unblockedCount++
				}
			}

			if blockedCount != tt.wantBlocked {
				t.Errorf("removeBlockedHosts() blocked %d hosts, want %d", blockedCount, tt.wantBlocked)
			}

			if unblockedCount != tt.wantUnblocked {
				t.Errorf("removeBlockedHosts() left %d unblocked hosts, want %d", unblockedCount, tt.wantUnblocked)
			}
		})
	}
}

func TestCheckBlocked(t *testing.T) {
	tests := []struct {
		name   string
		host   *Hostmap
		config *TomlConfig
		want   bool
	}{
		{
			name: "blocked by name and IP match",
			host: &Hostmap{ip: createIP("192.168.90.2"), hostnames: []string{"powerwall"}, lastseen: time.Now()},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "192.168.90.2", Name: "powerwall"},
					},
				},
			},
			want: true,
		},
		{
			name: "blocked by IP only",
			host: &Hostmap{ip: createIP("192.168.90.2"), hostnames: []string{"powerwall"}, lastseen: time.Now()},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "192.168.90.2", Name: ""},
					},
				},
			},
			want: true,
		},
		{
			name: "blocked by name only",
			host: &Hostmap{ip: createIP("192.168.90.2"), hostnames: []string{"powerwall"}, lastseen: time.Now()},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "", Name: "powerwall"},
					},
				},
			},
			want: true,
		},
		{
			name: "not blocked",
			host: &Hostmap{ip: createIP("192.168.1.1"), hostnames: []string{"host1"}, lastseen: time.Now()},
			config: &TomlConfig{
				Processing: struct {
					Domains    []string
					Additional []struct {
						IP        string
						Hostnames []string
						Name      string
					}
					Blocked  []struct{ IP, Name string }
					Cnames   []struct{ Cname, Hostname string }
					KeepMacs bool
				}{
					Blocked: []struct{ IP, Name string }{
						{IP: "192.168.90.2", Name: "powerwall"},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBlocked(tt.host, tt.config)
			if got != tt.want {
				t.Errorf("checkBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
