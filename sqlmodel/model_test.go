package sqlmodel

import (
	"database/sql"
	"net"
	"testing"
	"time"
)

// TestDomainModel tests the Domain model structure
func TestDomainModel(t *testing.T) {
	domain := Domain{
		ID:             1,
		Name:           "example.com",
		Master:         sql.NullString{String: "ns1.example.com", Valid: true},
		LastCheck:      sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		Type:           "MASTER",
		NotifiedSerial: sql.NullInt64{Int64: 123456, Valid: true},
		Account:        sql.NullString{String: "admin", Valid: true},
	}

	// Verify fields
	if domain.ID != 1 {
		t.Errorf("Domain.ID = %d; want 1", domain.ID)
	}
	if domain.Name != "example.com" {
		t.Errorf("Domain.Name = %s; want example.com", domain.Name)
	}
	if domain.Master.String != "ns1.example.com" {
		t.Errorf("Domain.Master = %s; want ns1.example.com", domain.Master.String)
	}
	if domain.Type != "MASTER" {
		t.Errorf("Domain.Type = %s; want MASTER", domain.Type)
	}
}

// TestRecordModel tests the Record model structure
func TestRecordModel(t *testing.T) {
	record := Record{
		ID:        1,
		DomainId:  sql.NullInt64{Int64: 1, Valid: true},
		Name:      "host.example.com",
		Type:      "A",
		Content:   "192.168.1.1",
		Ttl:       3600,
		Prio:      0,
		ChangDate: int(time.Now().Unix()),
		Disabled:  false,
	}

	// Verify fields
	if record.ID != 1 {
		t.Errorf("Record.ID = %d; want 1", record.ID)
	}
	if record.DomainId.Int64 != 1 {
		t.Errorf("Record.DomainId = %d; want 1", record.DomainId.Int64)
	}
	if record.Name != "host.example.com" {
		t.Errorf("Record.Name = %s; want host.example.com", record.Name)
	}
	if record.Type != "A" {
		t.Errorf("Record.Type = %s; want A", record.Type)
	}
	if record.Content != "192.168.1.1" {
		t.Errorf("Record.Content = %s; want 192.168.1.1", record.Content)
	}
	if record.Ttl != 3600 {
		t.Errorf("Record.Ttl = %d; want 3600", record.Ttl)
	}
}

// TestUnifiHostModel tests the UnifiHost model structure
func TestUnifiHostModel(t *testing.T) {
	now := time.Now()
	host := UnifiHost{
		ID:        1,
		Name:      "device1",
		IP:        net.ParseIP("192.168.1.100"),
		LastSeen:  now,
		FirstSeen: now.Add(-24 * time.Hour),
	}

	// Verify fields
	if host.ID != 1 {
		t.Errorf("UnifiHost.ID = %d; want 1", host.ID)
	}
	if host.Name != "device1" {
		t.Errorf("UnifiHost.Name = %s; want device1", host.Name)
	}
	if host.IP.String() != "192.168.1.100" {
		t.Errorf("UnifiHost.IP = %s; want 192.168.1.100", host.IP.String())
	}

	// Check that LastSeen is within a second of now
	if host.LastSeen.Sub(now) > time.Second {
		t.Errorf("UnifiHost.LastSeen is not within 1 second of expected time")
	}

	// Check that FirstSeen is about 24 hours before now
	expectedDiff := 24 * time.Hour
	actualDiff := now.Sub(host.FirstSeen)
	if actualDiff < expectedDiff-time.Second || actualDiff > expectedDiff+time.Second {
		t.Errorf("UnifiHost.FirstSeen is not about 24 hours before LastSeen")
	}
}
