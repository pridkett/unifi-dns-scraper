package scraper

import (
	"github.com/unpoller/unifi"
)

// MockUnifiClient implements a mock for the Unifi API client for testing
type MockUnifiClient struct {
	sites   []*unifi.Site
	devices *unifi.Devices
	clients []*unifi.Client
	err     error
}

// NewMockUnifiClient creates a new mock Unifi client for testing
func NewMockUnifiClient() *MockUnifiClient {
	return &MockUnifiClient{
		sites:   []*unifi.Site{},
		devices: &unifi.Devices{},
		clients: []*unifi.Client{},
	}
}

// AddSite adds a mock site
func (m *MockUnifiClient) AddSite(name string) *MockUnifiClient {
	m.sites = append(m.sites, &unifi.Site{
		Name: name,
	})
	return m
}

// AddClient adds a mock client
func (m *MockUnifiClient) AddClient(name, ip string, lastSeen float64) *MockUnifiClient {
	m.clients = append(m.clients, &unifi.Client{
		Name:     name,
		IP:       ip,
		Hostname: name,
		LastSeen: unifi.FlexInt{Val: lastSeen},
	})
	return m
}

// AddSwitch adds a mock switch
func (m *MockUnifiClient) AddSwitch(name, ip string, lastSeen float64) *MockUnifiClient {
	m.devices.USWs = append(m.devices.USWs, &unifi.USW{
		Name:     name,
		IP:       ip,
		LastSeen: unifi.FlexInt{Val: lastSeen},
	})
	return m
}

// AddAP adds a mock access point
func (m *MockUnifiClient) AddAP(name, ip string, lastSeen float64) *MockUnifiClient {
	m.devices.UAPs = append(m.devices.UAPs, &unifi.UAP{
		Name:     name,
		IP:       ip,
		LastSeen: unifi.FlexInt{Val: lastSeen},
	})
	return m
}

// SetError sets an error to be returned by the mock client
func (m *MockUnifiClient) SetError(err error) *MockUnifiClient {
	m.err = err
	return m
}

// GetSites implements the method to return mock sites
func (m *MockUnifiClient) GetSites() ([]*unifi.Site, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.sites, nil
}

// GetClients implements the method to return mock clients
func (m *MockUnifiClient) GetClients(_ []*unifi.Site) ([]*unifi.Client, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clients, nil
}

// GetDevices implements the method to return mock devices
func (m *MockUnifiClient) GetDevices(_ []*unifi.Site) (*unifi.Devices, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.devices, nil
}

// UnifiClientInterface defines an interface that both the real and mock clients can implement
type UnifiClientInterface interface {
	GetSites() ([]*unifi.Site, error)
	GetClients([]*unifi.Site) ([]*unifi.Client, error)
	GetDevices([]*unifi.Site) (*unifi.Devices, error)
}

// GetUnifiElementsWithClient is a version of getUnifiElements that accepts an interface
func GetUnifiElementsWithClient(cfg *TomlConfig, client UnifiClientInterface) ([]*unifi.Site, *unifi.Devices, []*unifi.Client, error) {
	sites, err := client.GetSites()
	if err != nil {
		logger.Errorf("Error getting sites: %s", err)
		logger.Warnf("Not updating list of hosts this round - will try again later")
		return nil, nil, nil, err
	}

	clients, err := client.GetClients(sites)
	if err != nil {
		logger.Errorf("Error getting clients: %s", err)
		return sites, nil, nil, err
	}

	devices, err := client.GetDevices(sites)
	if err != nil {
		logger.Errorf("Error getting devices: %s", err)
		return sites, nil, clients, err
	}

	logger.Infof("%d Unifi Sites Found", len(sites))
	logger.Infof("%d Clients connected", len(clients))
	logger.Infof("%d Unifi Switches Found", len(devices.USWs))
	logger.Infof("%d Unifi Gateways Found", len(devices.USGs))
	logger.Infof("%d Unifi Wireless APs Found", len(devices.UAPs))

	return sites, devices, clients, nil
}

// GenerateHostsFileWithClient is a version of GenerateHostsFile that accepts a client interface
func GenerateHostsFileWithClient(cfg *TomlConfig, hostmaps []*Hostmap, client UnifiClientInterface) ([]*Hostmap, error) {
	logger.Infof("Starting new host file generation")
	logger.Infof("%d existing hosts in the hostmap", len(hostmaps))

	_, devices, clients, err := GetUnifiElementsWithClient(cfg, client)
	if err != nil {
		logger.Errorf("Error getting Unifi elements: %s", err)
		return hostmaps, err
	}

	fullHostmap := createHostmap(clients, devices.USWs, devices.UAPs, cfg, hostmaps)

	return fullHostmap, nil
}

// GetRemovalCode allows access to the removalCode field for testing
func (h *Hostmap) GetRemovalCode() RemovalCode {
	return h.removalCode
}
