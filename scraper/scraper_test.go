package scraper

import (
	"net/netip"
	"testing"
	"time"
)

// import testing scaffolding

func createIP(ipaddr string) netip.Addr {
	ip, err := netip.ParseAddr(ipaddr)
	if err != nil {
		panic(err)
	}
	return ip
}

func TestRemoveOldHosts(t *testing.T) {
	// create the list of hostnames
	var hostmaps = []*Hostmap{}

	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.1"),
		hostnames: []string{"host1"},
		lastseen:  time.Now()})
	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.2"),
		hostnames: []string{"host1"},
		lastseen:  time.Unix(time.Now().Unix()-100, 0)})

	newhostmaps := removeOldHosts(hostmaps)

	if len(newhostmaps) != 1 {
		t.Errorf("Expected 1 hosts, got %d", len(newhostmaps))
	}
}

func TestRemoveDuplicateHosts(t *testing.T) {
	// create the list of hostnames
	var hostmaps = []*Hostmap{}

	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.1"),
		hostnames: []string{"host1"},
		lastseen:  time.Now()})
	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.1"),
		hostnames: []string{"host2"},
		lastseen:  time.Unix(time.Now().Unix()-100, 0)})

	newhostmaps := removeDuplicateHosts(hostmaps)

	if len(newhostmaps) != 1 {
		t.Errorf("Expected 1 hosts, got %d", len(newhostmaps))
	}

	if newhostmaps[0].hostnames[0] != "host1" {
		t.Errorf("Expected host1 as hostname, got %s", newhostmaps[0].hostnames[0])
	}
}

func TestRemoveOldHostsByTime(t *testing.T) {
	// create the list of hostnames
	var hostmaps = []*Hostmap{}

	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.1"),
		hostnames: []string{"host1"},
		lastseen:  time.Now()})
	hostmaps = append(hostmaps, &Hostmap{ip: createIP("192.168.1.2"),
		hostnames: []string{"host2"},
		lastseen:  time.Unix(time.Now().Unix()-4800, 0)})

	newhostmaps := removeOldHostsByTime(hostmaps, 3600)
	if len(newhostmaps) != 1 {
		t.Errorf("Expected 1 hosts, got %d", len(newhostmaps))
	}

	if newhostmaps[0].hostnames[0] != "host1" {
		t.Errorf("Expected host1 as hostname, got %s", newhostmaps[0].hostnames[0])
	}
}
