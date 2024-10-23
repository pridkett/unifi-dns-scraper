package scraper

import (
	"fmt"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/unpoller/unifi"
	"github.com/withmandala/go-log"
)

type HostsfileConfig struct {
	Filename string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type TomlConfig struct {
	Daemonize bool
	Sleep     int
	MaxAge    int
	Unifi     struct {
		Host     string
		User     string
		Password string
	}
	Processing struct {
		Domains    []string
		Additional []struct {
			IP        string
			Hostnames []string
			Name      string
		}
		Blocked []struct {
			IP   string
			Name string
		}
		KeepMacs bool
	}
	Hostsfile HostsfileConfig
	Database  DatabaseConfig
}

type RemovalCode int

const (
	NotRemoved RemovalCode = iota
	MacAddress
	Blocked
	Old
)

type Hostmap struct {
	ip            netip.Addr
	hostnames     []string
	fqdns         []string
	lastseen      time.Time
	lastseenUnifi time.Time
	removalCode   RemovalCode
}

// set up a global logger...
// see: https://stackoverflow.com/a/43827612/57626
var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

func getUnifiElements(cfg *TomlConfig) ([]*unifi.Site, *unifi.Devices, []*unifi.Client, error) {
	c := &unifi.Config{
		User:     cfg.Unifi.User,
		Pass:     cfg.Unifi.Password,
		URL:      cfg.Unifi.Host,
		ErrorLog: logger.Errorf,
		DebugLog: logger.Debugf,
	}

	uni, err := unifi.NewUnifi(c)
	if err != nil {
		logger.Errorf("Error conncting to Unifi: %s", err)
		logger.Warnf("Not updating list of hosts this round - will try again later")
		return nil, nil, nil, err
	}

	sites, err := uni.GetSites()
	if err != nil {
		logger.Errorf("Error getting sites: %s", err)
		logger.Warnf("Not updating list of hosts this round - will try again later")
		return nil, nil, nil, err
	}

	clients, err := uni.GetClients(sites)
	if err != nil {
		logger.Errorf("Error getting clients: %s", err)
		return sites, nil, nil, err
	}

	devices, err := uni.GetDevices(sites)
	if err != nil {
		logger.Errorf("Error getting devices: %s", err)
		return sites, nil, clients, err
	}

	logger.Infof("%d Unifi Sites Found:", len(sites))
	// for i, site := range sites {
	// 	logger.Infof("%d, %s %s", i+1, site.Name)
	// }

	logger.Infof("%d Clients connected:", len(clients))
	// for i, client := range clients {
	// 	logger.Infof("%d, %s %s %s %s %d", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen)
	// }

	logger.Infof("%d Unifi Switches Found", len(devices.USWs))
	// for i, usw := range devices.USWs {
	// 	logger.Infof("%d %s %s", i+1, usw.Name, usw.IP)
	// }

	logger.Infof("%d Unifi Gateways Found", len(devices.USGs))

	logger.Infof("%d Unifi Wireless APs Found:", len(devices.UAPs))
	// for i, uap := range devices.UAPs {
	// 	logger.Infof("%d %s %s", i+1, uap.Name, uap.IP)
	// }

	return sites, devices, clients, nil
}

func GenerateHostsFile(cfg *TomlConfig, hostmaps []*Hostmap) ([]*Hostmap, error) {

	logger.Infof("Starting new host file generation")
	logger.Infof("%d existing hosts in the hostmap", len(hostmaps))

	_, devices, clients, err := getUnifiElements(cfg)
	if err != nil {
		logger.Errorf("Error getting Unifi elements: %s", err)
		return hostmaps, err
	}

	fullHostmap := createHostmap(clients, devices.USWs, devices.UAPs, cfg, hostmaps)

	return fullHostmap, nil
}

func SaveHostsFile(hostmaps []*Hostmap, cfg *TomlConfig) error {
	var builder strings.Builder

	if cfg.Hostsfile.Filename == "" {
		logger.Warn("Hostfile output filename is nil - skipping")
		return nil
	}

	builder.WriteString("# This file created by unifi-dns-scraper\n")
	builder.WriteString("# Do not manually edit\n\n")
	for _, hm := range hostmaps {
		if hm.removalCode != NotRemoved {
			continue
		}

		builder.WriteString(fmt.Sprintf("%s %s\n", hm.ip, strings.Join(hm.fqdns, " ")))
	}

	err := os.WriteFile(cfg.Hostsfile.Filename, []byte(builder.String()), 0666)
	if err != nil {
		logger.Fatal(err)
		return err
	}
	logger.Infof("Wrote %d hosts to %s", len(hostmaps), cfg.Hostsfile.Filename)

	return nil
}

// iterate over the hostmap and add all the FQDNs to each host on the hostmap
func addDomainsToHostmap(m *Hostmap, domains []string) *Hostmap {
	for _, domain := range domains {
		for _, hostname := range m.hostnames {
			m.fqdns = append(m.fqdns, fmt.Sprintf("%s.%s", hostname, domain))
		}
	}
	return m
}

// given a hostmap, remove entries that are older than the lastseen time
// this iterates over all of the hosts and if we have different IP addresses
// for the same hostname, then we will only keep the most recent one
func removeOldHosts(m []*Hostmap) []*Hostmap {
	// create a dictionary for hosts
	hosts := make(map[string]*Hostmap)

	for _, host := range m {
		// check if host is in the dictionary
		if _, ok := hosts[host.ip.String()]; ok {
			// if it is, then check if the lastseen time is newer
			if host.lastseen.After(hosts[host.hostnames[0]].lastseen) {
				// if it is, then replace the existing entry
				hosts[host.hostnames[0]] = host
			}
		} else {
			// if it isn't, then add it
			hosts[host.hostnames[0]] = host
		}
	}

	// create a new slices of hostsmaps from the dictionary
	var newhosts []*Hostmap
	for _, host := range hosts {
		newhosts = append(newhosts, host)
	}

	return newhosts
}

// given a hostmap, remove entires that share the same IP address
// this iterates over all of the hosts and if two share the same
// IP address - it keeps only the IP address that is the most recent
func removeDuplicateHosts(m []*Hostmap) []*Hostmap {
	// create a dictionary for hosts
	hosts := make(map[string]*Hostmap)

	for _, host := range m {
		// check if host is in the dictionary
		if _, ok := hosts[host.ip.String()]; ok {
			// if it is, then check if the lastseen time is newer
			if host.lastseen.After(hosts[host.ip.String()].lastseen) {
				// if it is, then replace the existing entry
				hosts[host.ip.String()] = host
			}
		} else {
			// if it isn't, then add it
			hosts[host.ip.String()] = host
		}
	}

	// create a new slices of hostsmaps from the dictionary
	var newhosts []*Hostmap
	for _, host := range hosts {
		newhosts = append(newhosts, host)
	}
	return newhosts
}

// remove all hosts from the hostmap that have not been seen in d seconds
func removeOldHostsByTime(m []*Hostmap, d time.Duration) []*Hostmap {
	for _, host := range m {
		if time.Since(host.lastseen) > d {
			host.removalCode = Old
		}
	}
	return m
}

// remove all hosts from the hostmap that have a MAC address in the hostname
// I still don't know why this happens sometimes
func removeMACHosts(m []*Hostmap) []*Hostmap {
	for _, host := range m {
		var hostnames []string
		originalHostnames := host.hostnames
		for _, hostname := range host.hostnames {
			if !strings.Contains(hostname, ":") {
				hostnames = append(hostnames, hostname)
			}
		}
		if len(hostnames) > 0 {
			host.hostnames = hostnames
			// TODO: should do something about the FQDNs that are removed here
		} else {
			host.removalCode = MacAddress
			host.hostnames = originalHostnames
		}
	}
	return m
}

// remove all hosts from the hostmap that are in the blocked list
// really this is only needed because my Tesla Powerwall likes to
// misbehave and jump around IP addresses
func removeBlockedHosts(m []*Hostmap, cfg *TomlConfig) []*Hostmap {
	for _, host := range m {
		if checkBlocked(host, cfg) {
			logger.Warnf("host name=%s ip=%s is blocked from appearing in output by configuration", host.hostnames[0], host.ip.String())
			host.removalCode = Blocked
		}
	}
	return m
}

// check to see if the IP address or hostname is in the blocked list
func checkBlocked(h *Hostmap, cfg *TomlConfig) bool {
	for _, blocked := range cfg.Processing.Blocked {
		if blocked.Name != "" {
			// iterate over all of the given hostnames for the host
			for _, hostname := range h.hostnames {
				if strings.EqualFold(strings.TrimSpace(blocked.Name), strings.TrimSpace(hostname)) {
					if blocked.IP != "" {
						if blocked.IP == h.ip.String() {
							return true
						}
					} else {
						return true
					}
				}
			}
		} else if blocked.IP != "" {
			if strings.EqualFold(strings.TrimSpace(blocked.IP), strings.TrimSpace(h.ip.String())) {
				return true
			}
		}
	}
	return false
}

func createHostmap(clients []*unifi.Client, switches []*unifi.USW, aps []*unifi.UAP, cfg *TomlConfig, hostmaps []*Hostmap) []*Hostmap {

	if hostmaps == nil {
		hostmaps = []*Hostmap{}
	}

	// add in any of the statically defined hosts
	for _, additional := range cfg.Processing.Additional {
		var m Hostmap
		var err error
		m.ip, err = netip.ParseAddr(additional.IP)
		if err != nil {
			logger.Warnf("unable to parse IP address: %s", additional.IP)
			continue
		}
		if len(additional.Hostnames) > 0 {
			m.hostnames = append(m.hostnames, additional.Hostnames...)
		} else {
			m.hostnames = append(m.hostnames, additional.Name)
		}
		hostmaps = append(hostmaps, addDomainsToHostmap(&m, cfg.Processing.Domains))
	}

	for i, client := range clients {
		// logger.Infof("%d, %s %s %s %s %d", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen)
		var m Hostmap
		var err error
		m.lastseenUnifi = time.Unix(int64(client.LastSeen.Val), 0)
		m.lastseen = time.Now()
		m.ip, err = netip.ParseAddr(client.IP)
		if err != nil {
			logger.Warnf("Error Parsing Record: line=%d, ID=%s, hostname=%s, IP=%s, name=%s, lastseen=%f", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen.Val)
			continue
		}
		m.hostnames = append(m.hostnames, client.Name)
		hostmaps = append(hostmaps, addDomainsToHostmap(&m, cfg.Processing.Domains))
	}

	for _, usw := range switches {
		var m Hostmap
		var err error
		m.ip, err = netip.ParseAddr(usw.IP)

		m.lastseenUnifi = time.Unix(int64(usw.LastSeen.Val), 0)
		m.lastseen = time.Now()

		if err != nil {
			logger.Warnf("unable to parse IP address: %s", usw.IP)
			continue
		}

		m.hostnames = append(m.hostnames, usw.Name)
		hostmaps = append(hostmaps, addDomainsToHostmap(&m, cfg.Processing.Domains))
	}

	for _, ap := range aps {
		var m Hostmap
		var err error
		m.ip, err = netip.ParseAddr(ap.IP)
		m.lastseenUnifi = time.Unix(int64(ap.LastSeen.Val), 0)
		m.lastseen = time.Now()

		if err != nil {
			logger.Warnf("unable to parse IP address: %s", ap.IP)
			continue
		}

		m.hostnames = append(m.hostnames, ap.Name)
		hostmaps = append(hostmaps, addDomainsToHostmap(&m, cfg.Processing.Domains))
	}

	// blocked hosts get removed first - otherwise their "correct" IP address
	// may be hidden by one of the incorrect host addresses
	if !cfg.Processing.KeepMacs {
		logger.Warnf("Skipping MAC addresses")
		hostmaps = removeMACHosts(hostmaps)
	}

	hostmaps = removeBlockedHosts(hostmaps, cfg)
	hostmaps = removeDuplicateHosts(hostmaps)
	hostmaps = removeOldHosts(hostmaps)

	if cfg.MaxAge > 0 {
		hostmaps = removeOldHostsByTime(hostmaps, time.Duration(cfg.MaxAge))
	}

	sort.Slice(hostmaps, func(i, j int) bool {
		return hostmaps[i].ip.Less(hostmaps[j].ip)
	})

	return hostmaps
}
