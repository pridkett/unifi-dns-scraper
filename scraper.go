package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/naoina/toml"
	"github.com/unpoller/unifi"
	"github.com/withmandala/go-log"

	"inet.af/netaddr"
)

type tomlConfig struct {
	Daemonize bool
	Sleep     int
	Unifi     struct {
		Host     string
		User     string
		Password string
	}
	Hostsfile struct {
		Filename   string
		Domains    []string
		Additional []struct {
			IP        string
			Hostnames []string
			Name      string
		}
	}
}

type hostmap struct {
	ip        netaddr.IP
	hostnames []string
	fqdns     []string
}

// set up a global logger...
// see: https://stackoverflow.com/a/43827612/57626
var logger *log.Logger
var config tomlConfig

func main() {

	logger = log.New(os.Stderr).WithColor()

	configFile := flag.String("config", "", "Filename with configuration")
	flag.Parse()

	for {
		if *configFile != "" {
			logger.Infof("opening configuration file: %s", *configFile)
			f, err := os.Open(*configFile)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			if err := toml.NewDecoder(f).Decode(&config); err != nil {
				panic(err)
			}
		} else {
			logger.Fatal("Must specify configuration file with -config FILENAME")
		}

		c := &unifi.Config{
			User:     config.Unifi.User,
			Pass:     config.Unifi.Password,
			URL:      config.Unifi.Host,
			ErrorLog: logger.Debugf,
			DebugLog: logger.Debugf,
		}

		uni, err := unifi.NewUnifi(c)
		if err != nil {
			logger.Fatalf("Error: %s", err)
		}

		sites, err := uni.GetSites()
		if err != nil {
			logger.Fatalf("Error: %s", err)
		}
		clients, err := uni.GetClients(sites)
		if err != nil {
			logger.Fatalf("Error: %s", err)
		}
		devices, err := uni.GetDevices(sites)
		if err != nil {
			logger.Fatalf("Error: %s", err)
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

		createHostsFile(clients, devices.USWs, devices.UAPs)

		if config.Daemonize {
			sleep_dur := config.Sleep
			if sleep_dur == 0 {
				sleep_dur = 120
			}
			logger.Infof("Sleeping for %d seconds", sleep_dur)
			time.Sleep(time.Duration(sleep_dur) * time.Second)
		} else {
			break
		}
	}
}

func updateHostsFile(m *hostmap) *hostmap {
	for _, domain := range config.Hostsfile.Domains {
		for _, hostname := range m.hostnames {
			m.fqdns = append(m.fqdns, fmt.Sprintf("%s.%s", hostname, domain))
		}
	}
	return m
}

func createHostsFile(clients []*unifi.Client, switches []*unifi.USW, aps []*unifi.UAP) {
	var builder strings.Builder

	var hostmaps = []*hostmap{}
	if config.Hostsfile.Filename == "" {
		logger.Warn("Hostfile output filename is nil - skipping")
		return
	}

	builder.WriteString("# This file created by unifi-dns-scraper\n")
	builder.WriteString("# Do not manually edit\n\n")

	for _, additional := range config.Hostsfile.Additional {
		var m hostmap
		var err error
		m.ip, err = netaddr.ParseIP(additional.IP)
		if err != nil {
			logger.Fatalf("unable to parse IP address: %s", additional.IP)
		}
		if len(additional.Hostnames) > 0 {
			m.hostnames = append(m.hostnames, additional.Hostnames...)
		} else {
			m.hostnames = append(m.hostnames, additional.Name)
		}
		hostmaps = append(hostmaps, updateHostsFile(&m))
	}

	for _, client := range clients {
		// logger.Debugf("%d, %s %s %s %s %d", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen)
		var m hostmap
		var err error
		m.ip, err = netaddr.ParseIP(client.IP)
		if err != nil {
			logger.Fatalf("unable to parse IP address: %s", client.IP)
		}
		m.hostnames = append(m.hostnames, client.Name)
		hostmaps = append(hostmaps, updateHostsFile(&m))
	}

	for _, usw := range switches {
		var m hostmap
		var err error
		m.ip, err = netaddr.ParseIP(usw.IP)
		if err != nil {
			logger.Fatalf("unable to parse IP address: %s", usw.IP)
		}
		m.hostnames = append(m.hostnames, usw.Name)
		hostmaps = append(hostmaps, updateHostsFile(&m))
	}

	for _, ap := range aps {
		var m hostmap
		var err error
		m.ip, err = netaddr.ParseIP(ap.IP)
		if err != nil {
			logger.Fatalf("unable to parse IP address: %s", ap.IP)
		}

		m.hostnames = append(m.hostnames, ap.Name)
		hostmaps = append(hostmaps, updateHostsFile(&m))
	}

	sort.Slice(hostmaps, func(i, j int) bool {
		return hostmaps[i].ip.Less(hostmaps[j].ip)
	})

	for _, hm := range hostmaps {
		builder.WriteString(fmt.Sprintf("%s %s\n", hm.ip, strings.Join(hm.fqdns, " ")))
	}

	err := ioutil.WriteFile(config.Hostsfile.Filename, []byte(builder.String()), 0666)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("Wrote hosts to %s", config.Hostsfile.Filename)
}
