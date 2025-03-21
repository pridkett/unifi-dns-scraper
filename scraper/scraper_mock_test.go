package scraper

import (
	"fmt"
	"testing"
	"time"

	"github.com/withmandala/go-log"
)

func TestGenerateHostsFileWithMock(t *testing.T) {
	// Create a mock client
	mock := NewMockUnifiClient()
	mock.AddSite("Default")
	mock.AddClient("client1", "192.168.1.100", float64(time.Now().Unix()))
	mock.AddClient("client2", "192.168.1.101", float64(time.Now().Unix()))
	mock.AddSwitch("switch1", "192.168.1.2", float64(time.Now().Unix()))
	mock.AddAP("ap1", "192.168.1.3", float64(time.Now().Unix()))

	// Create a test config
	config := &TomlConfig{
		Processing: struct {
			Domains    []string
			Additional []struct {
				IP           string
				Hostnames    []string
				Name         string
				KeepMultiple *bool
			}
			Blocked  []struct{ IP, Name string }
			Cnames   []struct{ Cname, Hostname string }
			KeepMacs bool
		}{
			Domains: []string{"test.local"},
			Additional: []struct {
				IP           string
				Hostnames    []string
				Name         string
				KeepMultiple *bool
			}{
				{IP: "192.168.1.1", Name: "gateway", KeepMultiple: nil},
			},
		},
	}

	// Set global logger for testing
	if logger == nil {
		dummyLogger := log.New(nil)
		logger = dummyLogger
	}

	// Generate hosts file using mock client
	hostmaps, err := GenerateHostsFileWithClient(config, nil, mock)
	if err != nil {
		t.Errorf("GenerateHostsFileWithClient() error = %v", err)
		return
	}

	// Verify the results
	expectedHosts := 5 // 2 clients + 1 switch + 1 AP + 1 additional
	validHosts := 0
	for _, host := range hostmaps {
		if host.removalCode == NotRemoved {
			validHosts++
		}
	}

	if validHosts != expectedHosts {
		t.Errorf("Expected %d valid hosts, got %d", expectedHosts, validHosts)
	}

	// Verify each host has a FQDN
	for _, host := range hostmaps {
		if len(host.fqdns) != len(host.hostnames)*len(config.Processing.Domains) {
			t.Errorf("Host %v has %d FQDNs, expected %d",
				host.hostnames, len(host.fqdns),
				len(host.hostnames)*len(config.Processing.Domains))
		}
	}
}

func TestErrorHandlingWithMock(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func() *MockUnifiClient
		wantErr       bool
		expectedHosts int
	}{
		{
			name: "site error",
			setupMock: func() *MockUnifiClient {
				return NewMockUnifiClient().SetError(fmt.Errorf("site error"))
			},
			wantErr:       true,
			expectedHosts: 0,
		},
		{
			name: "client error",
			setupMock: func() *MockUnifiClient {
				mock := NewMockUnifiClient()
				mock.AddSite("Default")
				// Set error after sites are fetched but before clients
				mock.err = fmt.Errorf("client error")
				return mock
			},
			wantErr:       true,
			expectedHosts: 0,
		},
		{
			name: "device error",
			setupMock: func() *MockUnifiClient {
				mock := NewMockUnifiClient()
				mock.AddSite("Default")
				mock.AddClient("client1", "192.168.1.100", float64(time.Now().Unix()))
				// Set error after clients are fetched but before devices
				mock.err = fmt.Errorf("device error")
				return mock
			},
			wantErr:       true,
			expectedHosts: 0,
		},
	}

	// Create a test config
	config := &TomlConfig{
		Processing: struct {
			Domains    []string
			Additional []struct {
				IP           string
				Hostnames    []string
				Name         string
				KeepMultiple *bool
			}
			Blocked  []struct{ IP, Name string }
			Cnames   []struct{ Cname, Hostname string }
			KeepMacs bool
		}{
			Domains: []string{"test.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()

			// Set global logger for testing
			if logger == nil {
				dummyLogger := log.New(nil)
				logger = dummyLogger
			}

			hostmaps, err := GenerateHostsFileWithClient(config, nil, mock)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateHostsFileWithClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			validHosts := 0
			for _, host := range hostmaps {
				if host.removalCode == NotRemoved {
					validHosts++
				}
			}

			if validHosts != tt.expectedHosts {
				t.Errorf("Expected %d valid hosts, got %d", tt.expectedHosts, validHosts)
			}
		})
	}
}
