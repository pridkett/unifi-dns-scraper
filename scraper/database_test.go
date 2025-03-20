package scraper

import (
	"os"
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
				IP        string
				Hostnames []string
				Name      string
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
