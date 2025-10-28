package scraper

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pridkett/unifi-dns-scraper/sqlmodel"
	"github.com/withmandala/go-log"
	"gorm.io/gorm"
)

// TestOpenDatabase tests various database connection methods
func TestOpenDatabase(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		dsn     string
		wantErr bool
	}{
		{
			name:    "sqlite in-memory",
			driver:  "sqlite",
			dsn:     ":memory:",
			wantErr: false,
		},
		{
			name:    "unsupported driver",
			driver:  "nonexistent",
			dsn:     "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := OpenDatabase(tt.driver, tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if db == nil {
					t.Errorf("OpenDatabase() returned nil database connection")
				} else {
					// Ensure db is recognized as *gorm.DB to force the import
					var _ *gorm.DB = db
					// Test we can perform a simple query
					var count int64
					if err := db.Model(&sqlmodel.Record{}).Count(&count).Error; err != nil {
						t.Errorf("Failed to perform a simple query: %v", err)
					}
				}
			}
		})
	}
}

// TestSaveDatabase tests the functionality to save records to the database
func TestSaveDatabase(t *testing.T) {
	// Create a temporary in-memory database
	db, err := OpenDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Set up test data
	// We'll create hostmaps that will be saved to the database
	hostmaps := []*Hostmap{
		{
			ip:          createIP("192.168.1.100"),
			hostnames:   []string{"test1"},
			fqdns:       []string{"test1.local", "test1.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
		{
			ip:          createIP("192.168.1.101"),
			hostnames:   []string{"test2"},
			fqdns:       []string{"test2.local", "test2.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
	}

	// Create a config
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
			Domains: []string{"local", "example.com"},
		},
	}

	// Set up global logger for testing (needed by SaveDatabase)
	if logger == nil {
		logger = log.New(os.Stderr)
	}

	// First test - initial save
	err = SaveDatabase(db, hostmaps, config)
	if err != nil {
		t.Errorf("SaveDatabase() error = %v", err)
		return
	}

	// Check that records were saved
	var count int64
	if err := db.Model(&sqlmodel.Record{}).Count(&count).Error; err != nil {
		t.Errorf("Failed to count records: %v", err)
	}

	// We should have 4 records (2 hostmaps with 2 FQDNs each)
	expectedRecords := int64(4)
	if count != expectedRecords {
		t.Errorf("Expected %d records, got %d", expectedRecords, count)
	}

	// Second test - update existing records with new IPs
	updatedHostmaps := []*Hostmap{
		{
			ip:          createIP("192.168.1.102"), // Changed IP
			hostnames:   []string{"test1"},
			fqdns:       []string{"test1.local", "test1.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
		{
			ip:          createIP("192.168.1.103"), // Changed IP
			hostnames:   []string{"test2"},
			fqdns:       []string{"test2.local", "test2.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
	}

	err = SaveDatabase(db, updatedHostmaps, config)
	if err != nil {
		t.Errorf("SaveDatabase() error on update = %v", err)
		return
	}

	// Verify that the records were updated and not duplicated
	if err := db.Model(&sqlmodel.Record{}).Count(&count).Error; err != nil {
		t.Errorf("Failed to count records after update: %v", err)
	}

	if count != expectedRecords {
		t.Errorf("Expected still %d records after update, got %d", expectedRecords, count)
	}

	// Verify that the IP addresses were actually updated
	var records []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Find(&records).Error; err != nil {
		t.Errorf("Failed to find records: %v", err)
	}

	// Check that all records have the new IP addresses
	for _, record := range records {
		if record.Name == "test1.local" || record.Name == "test1.example.com" {
			if record.Content != "192.168.1.102" {
				t.Errorf("Record %s not updated, expected IP 192.168.1.102, got %s", record.Name, record.Content)
			}
		} else if record.Name == "test2.local" || record.Name == "test2.example.com" {
			if record.Content != "192.168.1.103" {
				t.Errorf("Record %s not updated, expected IP 192.168.1.103, got %s", record.Name, record.Content)
			}
		}
	}
}

// Helper function to create a deterministic time for testing
var cTimeNow = createTestTime()

func createTestTime() time.Time {
	// Parse a fixed time string to get a deterministic time for tests
	t, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
	return t
}

// TestNormalizeDNSName tests the normalizeDNSName helper function
func TestNormalizeDNSName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "name without trailing dot",
			input:    "example.com",
			expected: "example.com.",
		},
		{
			name:     "name with trailing dot",
			input:    "example.com.",
			expected: "example.com.",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "subdomain without trailing dot",
			input:    "test.example.com",
			expected: "test.example.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDNSName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDNSName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeHostname tests the normalizeHostname helper function
func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "name with trailing dot",
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "name without trailing dot",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "subdomain with trailing dot",
			input:    "test.example.com.",
			expected: "test.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeHostname(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeHostname(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSaveDatabaseWithCNAME tests CNAME handling in the database
func TestSaveDatabaseWithCNAME(t *testing.T) {
	// Create a temporary in-memory database
	db, err := OpenDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Set up test data with hosts
	hostmaps := []*Hostmap{
		{
			ip:          createIP("192.168.1.100"),
			hostnames:   []string{"webserver"},
			fqdns:       []string{"webserver.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
		{
			ip:          createIP("192.168.1.101"),
			hostnames:   []string{"mailserver"},
			fqdns:       []string{"mailserver.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
	}

	// Create a config with CNAME entries
	// Test both with and without trailing dots
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
			Domains: []string{"example.com"},
			Cnames: []struct{ Cname, Hostname string }{
				{Cname: "www.example.com", Hostname: "webserver.example.com"},      // without trailing dots
				{Cname: "mail.example.com.", Hostname: "mailserver.example.com."}, // with trailing dots
			},
		},
	}

	// Set up global logger for testing
	if logger == nil {
		logger = log.New(os.Stderr)
	}

	// Save to database
	err = SaveDatabase(db, hostmaps, config)
	if err != nil {
		t.Errorf("SaveDatabase() error = %v", err)
		return
	}

	// Verify CNAME records were created
	var cnameRecords []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "CNAME").Find(&cnameRecords).Error; err != nil {
		t.Errorf("Failed to find CNAME records: %v", err)
	}

	// We should have 2 CNAME records
	if len(cnameRecords) != 2 {
		t.Errorf("Expected 2 CNAME records, got %d", len(cnameRecords))
	}

	// Verify that CNAME content fields have trailing dots (PowerDNS requirement)
	for _, record := range cnameRecords {
		if !strings.HasSuffix(record.Content, ".") {
			t.Errorf("CNAME record %s has content without trailing dot: %s", record.Name, record.Content)
		}

		// Verify the correct content
		if record.Name == "www.example.com" {
			if record.Content != "webserver.example.com." {
				t.Errorf("Expected CNAME content 'webserver.example.com.', got '%s'", record.Content)
			}
		} else if record.Name == "mail.example.com" {
			if record.Content != "mailserver.example.com." {
				t.Errorf("Expected CNAME content 'mailserver.example.com.', got '%s'", record.Content)
			}
		}
	}

	// Test updating CNAME records
	updatedConfig := &TomlConfig{
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
			Domains: []string{"example.com"},
			Cnames: []struct{ Cname, Hostname string }{
				{Cname: "www.example.com", Hostname: "mailserver.example.com"}, // Changed target
				{Cname: "mail.example.com", Hostname: "mailserver.example.com"},
			},
		},
	}

	err = SaveDatabase(db, hostmaps, updatedConfig)
	if err != nil {
		t.Errorf("SaveDatabase() update error = %v", err)
		return
	}

	// Verify the CNAME was updated
	var updatedRecords []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Where("type = ? AND name = ?", "CNAME", "www.example.com").Find(&updatedRecords).Error; err != nil {
		t.Errorf("Failed to find updated CNAME records: %v", err)
	}

	if len(updatedRecords) != 1 {
		t.Errorf("Expected 1 updated CNAME record, got %d", len(updatedRecords))
	} else {
		if updatedRecords[0].Content != "mailserver.example.com." {
			t.Errorf("Expected updated CNAME content 'mailserver.example.com.', got '%s'", updatedRecords[0].Content)
		}
	}

	// Verify still only 2 CNAME records (no duplicates)
	var finalCount int64
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "CNAME").Count(&finalCount).Error; err != nil {
		t.Errorf("Failed to count CNAME records: %v", err)
	}
	if finalCount != 2 {
		t.Errorf("Expected 2 CNAME records after update, got %d", finalCount)
	}
}

// TestSaveDatabaseCNAMENotFound tests CNAME handling when target doesn't exist
func TestSaveDatabaseCNAMENotFound(t *testing.T) {
	// Create a temporary in-memory database
	db, err := OpenDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Set up test data with hosts
	hostmaps := []*Hostmap{
		{
			ip:          createIP("192.168.1.100"),
			hostnames:   []string{"webserver"},
			fqdns:       []string{"webserver.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
	}

	// Create a config with CNAME pointing to non-existent host
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
			Domains: []string{"example.com"},
			Cnames: []struct{ Cname, Hostname string }{
				{Cname: "www.example.com", Hostname: "nonexistent.example.com"},
			},
		},
	}

	// Set up global logger for testing
	if logger == nil {
		logger = log.New(os.Stderr)
	}

	// Save to database
	err = SaveDatabase(db, hostmaps, config)
	if err != nil {
		t.Errorf("SaveDatabase() error = %v", err)
		return
	}

	// Verify no CNAME records were created (target doesn't exist)
	var cnameRecords []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "CNAME").Find(&cnameRecords).Error; err != nil {
		t.Errorf("Failed to find CNAME records: %v", err)
	}

	if len(cnameRecords) != 0 {
		t.Errorf("Expected 0 CNAME records for non-existent target, got %d", len(cnameRecords))
	}
}

// TestSaveDatabaseWithCNAMEChaining tests CNAME -> CNAME -> A record chains
func TestSaveDatabaseWithCNAMEChaining(t *testing.T) {
	// Create a temporary in-memory database
	db, err := OpenDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Set up test data with a base host
	hostmaps := []*Hostmap{
		{
			ip:          createIP("192.168.1.100"),
			hostnames:   []string{"docker-services"},
			fqdns:       []string{"docker-services.example.com"},
			lastseen:    cTimeNow,
			removalCode: NotRemoved,
		},
	}

	// Create a config with CNAME chain: paperless-ai -> paperless-ngx -> docker-services
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
			Domains: []string{"example.com"},
			Cnames: []struct{ Cname, Hostname string }{
				{Cname: "paperless-ai.example.com", Hostname: "paperless-ngx.example.com"},     // CNAME -> CNAME
				{Cname: "paperless-ngx.example.com", Hostname: "docker-services.example.com"}, // CNAME -> A
			},
		},
	}

	// Set up global logger for testing
	if logger == nil {
		logger = log.New(os.Stderr)
	}

	// Save to database
	err = SaveDatabase(db, hostmaps, config)
	if err != nil {
		t.Errorf("SaveDatabase() error = %v", err)
		return
	}

	// Verify CNAME records were created for the chain
	var cnameRecords []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "CNAME").Order("name").Find(&cnameRecords).Error; err != nil {
		t.Errorf("Failed to find CNAME records: %v", err)
	}

	// We should have 2 CNAME records
	if len(cnameRecords) != 2 {
		t.Errorf("Expected 2 CNAME records, got %d", len(cnameRecords))
		for _, rec := range cnameRecords {
			t.Logf("Found CNAME: %s -> %s", rec.Name, rec.Content)
		}
	}

	// Verify the chain is correct
	foundAI := false
	foundNGX := false

	for _, record := range cnameRecords {
		// All CNAME content should have trailing dots
		if !strings.HasSuffix(record.Content, ".") {
			t.Errorf("CNAME record %s has content without trailing dot: %s", record.Name, record.Content)
		}

		if record.Name == "paperless-ai.example.com" {
			foundAI = true
			if record.Content != "paperless-ngx.example.com." {
				t.Errorf("Expected paperless-ai to point to 'paperless-ngx.example.com.', got '%s'", record.Content)
			}
		}

		if record.Name == "paperless-ngx.example.com" {
			foundNGX = true
			if record.Content != "docker-services.example.com." {
				t.Errorf("Expected paperless-ngx to point to 'docker-services.example.com.', got '%s'", record.Content)
			}
		}
	}

	if !foundAI {
		t.Error("paperless-ai.example.com CNAME not found")
	}
	if !foundNGX {
		t.Error("paperless-ngx.example.com CNAME not found")
	}
}
