package main

import (
	"os"
	"testing"
	"time"

	"github.com/pridkett/unifi-dns-scraper/scraper"
	"github.com/withmandala/go-log"
)

// TestIntegrationWorkflow tests the basic workflow of the application
// This test uses mocks instead of connecting to a real Unifi device
func TestIntegrationWorkflow(t *testing.T) {
	// Setup temporary test files
	hostsFilePath := "test_hosts.txt"
	testDbPath := "test.db"

	// Cleanup after the test
	defer func() {
		os.Remove(hostsFilePath)
		os.Remove(testDbPath)
	}()

	// Setup mock Unifi client
	mock := scraper.NewMockUnifiClient()
	mock.AddSite("Default")
	mock.AddClient("laptop", "192.168.1.100", float64(time.Now().Unix()))
	mock.AddClient("phone", "192.168.1.101", float64(time.Now().Unix()))
	mock.AddSwitch("switch", "192.168.1.2", float64(time.Now().Unix()))
	mock.AddAP("ap", "192.168.1.3", float64(time.Now().Unix()))

	// Create a config
	config := &scraper.TomlConfig{
		Daemonize: false,
		Sleep:     60,
		MaxAge:    3600,
		Unifi: struct {
			Host     string
			User     string
			Password string
		}{
			Host:     "https://localhost:8443",
			User:     "test",
			Password: "test",
		},
		Processing: struct {
			Domains    []string
			Additional []struct {
				IP        string
				Hostnames []string
				Name      string
			}
			Blocked  []struct{ IP, Name string }
			KeepMacs bool
		}{
			Domains: []string{"test.local", "example.com"},
			Additional: []struct {
				IP        string
				Hostnames []string
				Name      string
			}{
				{IP: "192.168.1.1", Name: "router"},
			},
			Blocked: []struct{ IP, Name string }{
				{IP: "192.168.1.200", Name: "blocked-device"},
			},
			KeepMacs: false,
		},
		Hostsfile: scraper.HostsfileConfig{
			Filename: hostsFilePath,
		},
		Database: scraper.DatabaseConfig{
			Driver: "sqlite",
			DSN:    testDbPath,
		},
	}

	// Set up logger
	logger := log.New(os.Stderr)
	scraper.SetLogger(logger)

	// Run the workflow
	// 1. Generate the hosts file
	hostmaps, err := scraper.GenerateHostsFileWithClient(config, nil, mock)
	if err != nil {
		t.Fatalf("GenerateHostsFile failed: %v", err)
	}

	// Verify we have the expected number of host entries
	expectedHosts := 5 // 2 clients + 1 switch + 1 AP + 1 additional

	// All hosts should be valid in the initial creation
	validHosts := len(hostmaps)

	if validHosts != expectedHosts {
		t.Errorf("Expected %d valid hosts, got %d", expectedHosts, validHosts)
	}

	// 2. Save the hosts file
	err = scraper.SaveHostsFile(hostmaps, config)
	if err != nil {
		t.Fatalf("SaveHostsFile failed: %v", err)
	}

	// Verify the hosts file was created
	if _, err := os.Stat(hostsFilePath); os.IsNotExist(err) {
		t.Errorf("Hosts file was not created at %s", hostsFilePath)
	}

	// 3. Save to database
	db, err := scraper.OpenDatabase(config.Database.Driver, config.Database.DSN)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}

	err = scraper.SaveDatabase(db, hostmaps, config)
	if err != nil {
		t.Fatalf("SaveDatabase failed: %v", err)
	}

	// Count records in the database for our test IPs
	var count int64
	if err := db.Table("records").Where("content IN (?, ?, ?, ?, ?)",
		"192.168.1.1", "192.168.1.2", "192.168.1.3", "192.168.1.100", "192.168.1.101").Count(&count).Error; err != nil {
		t.Errorf("Failed to count records: %v", err)
	}

	// Each host should have 2 FQDNs (one for each domain)
	expectedRecords := int64(expectedHosts * 2)
	if count != expectedRecords {
		t.Errorf("Expected %d database records, got %d", expectedRecords, count)
	}
}
