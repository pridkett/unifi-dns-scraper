package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/naoina/toml"
	"github.com/unpoller/unifi"
	"github.com/withmandala/go-log"

	"inet.af/netaddr"
)

type tomlConfig struct {
	Unifi struct {
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
		}
	}
}

type hostmap struct {
	ip        netaddr.IP
	hostnames []string
}

// set up a global logger...
// see: https://stackoverflow.com/a/43827612/57626
var logger *log.Logger
var config tomlConfig

func main() {

	logger = log.New(os.Stderr).WithColor()

	configFile := flag.String("config", "", "Filename with configuration")
	flag.Parse()

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

	createHostsFile(clients)
}

func updateHostsFile(m *hostmap) string {
	hostnames := []string{}
	for _, domain := range config.Hostsfile.Domains {
		for _, hostname := range m.hostnames {
			hostnames = append(hostnames, fmt.Sprintf("%s.%s", hostname, domain))
		}
	}
	outstr := fmt.Sprintf("%s %s\n", m.ip, strings.Join(hostnames, " "))
	return outstr
}

func createHostsFile(clients []*unifi.Client) {
	var builder strings.Builder

	if config.Hostsfile.Filename == "" {
		logger.Warn("Hostfile output filename is nil - skipping")
		return
	}

	builder.WriteString("# This file created by unifi-dns-scraper\n")
	builder.WriteString("# Do not manually edit\n\n")

	for _, client := range clients {
		// logger.Debugf("%d, %s %s %s %s %d", i+1, client.ID, client.Hostname, client.IP, client.Name, client.LastSeen)
		var m hostmap
		var err error
		m.ip, err = netaddr.ParseIP(client.IP)
		m.hostnames = append(m.hostnames, client.Name)
		if err != nil {
			logger.Fatalf("unable to parse IP address: %s", client.IP)
		}
		builder.WriteString(updateHostsFile(&m))
	}

	err := ioutil.WriteFile(config.Hostsfile.Filename, []byte(builder.String()), 0666)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("Wrote hosts to %s", config.Hostsfile.Filename)
}
