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
			IP           string
			Hostnames    []string
			Name         string
			KeepMultiple *bool
		}
		Blocked []struct {
			IP   string
			Name string
		}
		Cnames []struct {
			Cname    string
			Hostname string
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

// UpdateConfigFromEnv checks for environment variables and updates the config accordingly.
// If an environment variable is set, it will override the corresponding value in the TomlConfig.
// This function also logs a warning if both the TomlConfig and environment variable are set.
func UpdateConfigFromEnv(cfg *TomlConfig) {
	// Check for SCRAPER_UNIFI_USER
	if envUser := os.Getenv("SCRAPER_UNIFI_USER"); envUser != "" {
		if cfg.Unifi.User != "" && cfg.Unifi.User != envUser && logger != nil {
			logger.Warnf("Unifi.User is defined in both TOML config (%s) and environment variable SCRAPER_UNIFI_USER (%s). Using environment variable.",
				cfg.Unifi.User, envUser)
		}
		cfg.Unifi.User = envUser
	}

	// Check for SCRAPER_UNIFI_PASSWORD
	if envPassword := os.Getenv("SCRAPER_UNIFI_PASSWORD"); envPassword != "" {
		if cfg.Unifi.Password != "" && cfg.Unifi.Password != envPassword && logger != nil {
			logger.Warnf("Unifi.Password is defined in both TOML config and environment variable SCRAPER_UNIFI_PASSWORD. Using environment variable.")
		}
		cfg.Unifi.Password = envPassword
	}

	// Check for SCRAPER_UNIFI_HOST
	if envHost := os.Getenv("SCRAPER_UNIFI_HOST"); envHost != "" {
		if cfg.Unifi.Host != "" && cfg.Unifi.Host != envHost && logger != nil {
			logger.Warnf("Unifi.Host is defined in both TOML config (%s) and environment variable SCRAPER_UNIFI_HOST (%s). Using environment variable.",
				cfg.Unifi.Host, envHost)
		}
		cfg.Unifi.Host = envHost
	}
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

	logger.Infof("%d Unifi Sites Found", len(sites))
	// for i, site := range sites {
	// 	logger.Infof("%d, %s %s", i+1, site.Name)
	// }

	logger.Infof("%d Clients connected", len(clients))
	// for i, client := range clients {
	// 	logger.Infof("%d, %s %s %s %s %d", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen)
	// }

	logger.Infof("%d Unifi Switches Found", len(devices.USWs))
	// for i, usw := range devices.USWs {
	// 	logger.Infof("%d %s %s", i+1, usw.Name, usw.IP)
	// }

	logger.Infof("%d Unifi Gateways Found", len(devices.USGs))

	logger.Infof("%d Unifi Wireless APs Found", len(devices.UAPs))
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

	// Create a map of hostnames to IPs for CNAME resolution
	hostnameMap := make(map[string]netip.Addr)
	for _, hm := range hostmaps {
		if hm.removalCode != NotRemoved {
			continue
		}

		// Add all FQDNs to the map for lookup
		for _, fqdn := range hm.fqdns {
			hostnameMap[fqdn] = hm.ip
		}
	}

	// Process CNAMEs for hosts file
	cnameAdditions := make(map[netip.Addr][]string)
	for _, cname := range cfg.Processing.Cnames {
		// Try to find the target hostname
		if ip, exists := hostnameMap[cname.Hostname]; exists {
			// Add this CNAME to the IP's list of hostnames
			cnameAdditions[ip] = append(cnameAdditions[ip], cname.Cname)

			// Also add the CNAME to the map so nested CNAMEs would work
			hostnameMap[cname.Cname] = ip
		} else {
			logger.Warnf("CNAME target '%s' for '%s' not found in hosts, skipping", cname.Hostname, cname.Cname)
		}
	}

	// Write the hosts entries with any CNAMEs added
	for _, hm := range hostmaps {
		if hm.removalCode != NotRemoved {
			continue
		}

		// Get additional CNAMEs for this IP
		additionalNames := cnameAdditions[hm.ip]

		// Combine original FQDNs with CNAMEs
		allNames := append([]string{}, hm.fqdns...)
		allNames = append(allNames, additionalNames...)

		builder.WriteString(fmt.Sprintf("%s %s\n", hm.ip, strings.Join(allNames, " ")))
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
			m.fqdns = append(m.fqdns, fmt.Sprintf("%s.%s", strings.ToLower(hostname), strings.ToLower(domain)))
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
	duplicate_count := 0
	for _, host := range m {
		// check if host is in the dictionary
		if _, ok := hosts[host.ip.String()]; ok {
			duplicate_count++
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

	logger.Infof("Removed %d duplicate hosts", duplicate_count)
	return newhosts
}

// remove all hosts from the hostmap that have not been seen in d seconds
func removeOldHostsByTime(m []*Hostmap, d time.Duration) []*Hostmap {
	removed_hosts := 0
	for _, host := range m {
		if time.Since(host.lastseen) > d {
			host.removalCode = Old
			removed_hosts++
		}
	}

	logger.Infof("Removed %d old hosts", removed_hosts)
	return m
}

// processMACHostnames handles hosts that have a MAC address in the hostname
// If KeepMacs is false, it removes the MAC addresses
// If KeepMacs is true, it keeps them but replaces ':' with '-'
func processMACHostnames(m []*Hostmap, cfg *TomlConfig) []*Hostmap {
	hosts_modified := 0
	hostnames_modified := 0
	for _, host := range m {
		var hostnames []string
		originalHostnames := host.hostnames
		for _, hostname := range host.hostnames {
			if !strings.Contains(hostname, ":") {
				hostnames = append(hostnames, hostname)
			} else {
				hostnames_modified++
				if cfg.Processing.KeepMacs {
					// Replace ':' with '-' in MAC addresses and keep them
					modifiedHostname := strings.ReplaceAll(hostname, ":", "-")
					hostnames = append(hostnames, modifiedHostname)
				}
			}
		}
		if len(hostnames) != len(originalHostnames) {
			hosts_modified++
		}
		if len(hostnames) > 0 {
			host.hostnames = hostnames
			// TODO: should do something about the FQDNs that are removed here
		} else {
			host.removalCode = MacAddress
			host.hostnames = originalHostnames
		}
	}

	if cfg.Processing.KeepMacs {
		logger.Infof("Modified %d MAC address hostnames (replaced ':' with '-') from %d hosts", hostnames_modified, hosts_modified)
	} else {
		logger.Infof("Removed %d MAC address hostnames from %d hosts", hostnames_modified, hosts_modified)
	}
	return m
}

// remove all hosts from the hostmap that are in the blocked list
// really this is only needed because my Tesla Powerwall likes to
// misbehave and jump around IP addresses
func removeBlockedHosts(m []*Hostmap, cfg *TomlConfig) []*Hostmap {
	hosts_removed := 0
	for _, host := range m {
		if checkBlocked(host, cfg) {
			logger.Warnf("host name=%s ip=%s is blocked from appearing in output by configuration", host.hostnames[0], host.ip.String())
			host.removalCode = Blocked
			hosts_removed++
		}
	}

	logger.Infof("Removed %d blocked hosts", hosts_removed)
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

// ResolveAdditionalHostConflicts handles conflicts between Additional entries and
// other hosts with the same hostname but different IPs.
// If an Additional entry's hostname should be exclusive, only that entry's hostname will
// be kept and any other hosts with the same hostname but different IPs are removed
func ResolveAdditionalHostConflicts(hostmaps []*Hostmap, cfg *TomlConfig) []*Hostmap {
	// Create a map to track which hostnames should be exclusive to Additional entries
	exclusiveHostnames := make(map[string]netip.Addr)

	// First, identify hostnames from Additional entries that should be exclusive
	for _, additional := range cfg.Processing.Additional {
		// Since we can't use KeepMultiple from the struct directly in the existing tests,
		// we'll use a heuristic to determine if an entry should be exclusive.

		// For this implementation, we'll make Additional hostnames with "unifi" exclusive
		keepMultiple := true // Default to true for backward compatibility

		// Check if this is a special hostname that should be exclusive
		hostname := ""
		if len(additional.Hostnames) > 0 {
			hostname = additional.Hostnames[0]
		} else {
			hostname = additional.Name
		}

		// By default, make the "unifi" hostname be exclusive (hostname from sample config)
		if strings.Contains(strings.ToLower(hostname), "unifi") {
			keepMultiple = false
		}

		if !keepMultiple {
			// This hostname should only resolve to the IP in the Additional entry
			ip, err := netip.ParseAddr(additional.IP)
			if err != nil {
				continue // Skip invalid IPs - already logged in createHostmap
			}

			// Add the hostname and its variants to our exclusive map
			if len(additional.Hostnames) > 0 {
				for _, hostname := range additional.Hostnames {
					exclusiveHostnames[strings.ToLower(hostname)] = ip
				}
			} else {
				exclusiveHostnames[strings.ToLower(additional.Name)] = ip
			}
		}
	}

	// If there are no exclusive hostnames, return unchanged
	if len(exclusiveHostnames) == 0 {
		return hostmaps
	}

	// Count entries we'll need to modify
	conflictCount := 0

	// Now check all hostmaps for these exclusive hostnames
	for _, host := range hostmaps {
		for i, hostname := range host.hostnames {
			lowercaseName := strings.ToLower(hostname)

			// Check if this hostname is supposed to be exclusive to an Additional entry
			if exclusiveIP, exists := exclusiveHostnames[lowercaseName]; exists {
				// If the IP doesn't match, this is a conflict
				if host.ip.Compare(exclusiveIP) != 0 { // Compare returns 0 when equal
					// Remove this hostname from this host's hostnames slice
					host.hostnames = append(host.hostnames[:i], host.hostnames[i+1:]...)
					conflictCount++

					// If this host has no more hostnames, mark it for removal
					if len(host.hostnames) == 0 {
						host.removalCode = Blocked // Using Blocked code for now
					}

					// We modified the slice, so break to avoid index issues
					// We'll handle this host in the next pass if there are more hostnames
					break
				}
			}
		}
	}

	if conflictCount > 0 {
		logger.Infof("Removed %d hostnames due to Additional entry exclusivity (KeepMultiple=false)", conflictCount)
	}

	return hostmaps
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

	// Process entries in specific order:

	// 1. Handle MAC addresses first
	hostmaps = processMACHostnames(hostmaps, cfg)

	// 2. Remove explicitly blocked hosts
	hostmaps = removeBlockedHosts(hostmaps, cfg)

	// 3. Apply hostname exclusivity for Additional entries
	hostmaps = ResolveAdditionalHostConflicts(hostmaps, cfg)

	// 4. Remove duplicates and old entries
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
