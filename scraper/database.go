package scraper

import (
	"fmt"

	"github.com/pridkett/unifi-dns-scraper/sqlmodel"

	"github.com/glebarez/sqlite" // pure go sqlite driver
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func OpenDatabase(driver string, dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	switch driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, err
		}
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Info)})
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	// Automatically migrate your schema, create tables if they do not exist
	err = db.AutoMigrate(&sqlmodel.Domain{}, &sqlmodel.Record{}, &sqlmodel.UnifiHost{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func SaveDatabase(db *gorm.DB, hostmaps []*Hostmap, config *TomlConfig) error {

	var records []sqlmodel.Record
	var updateRecords []sqlmodel.Record
	var newRecords []sqlmodel.Record

	// Get all existing A records from the database
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "A").Find(&records).Error; err != nil {
		return err
	}

	// Map domain names to their DB records for easy lookup
	// Normalize keys to ensure consistent lookups
	recordMap := make(map[string]sqlmodel.Record)
	for _, record := range records {
		recordMap[normalizeHostname(record.Name)] = record
	}

	// First, process host A records
	for _, hostmap := range hostmaps {
		for _, host := range hostmap.fqdns {
			normalizedHost := normalizeHostname(host)

			if record, ok := recordMap[normalizedHost]; ok {
				if record.Content != hostmap.ip.String() {
					record.Content = hostmap.ip.String()
					updateRecords = append(updateRecords, record)
				}
			} else {
				newRecords = append(newRecords, sqlmodel.Record{
					Name:    normalizedHost,
					Type:    "A",
					Content: hostmap.ip.String(),
					Ttl:     3600,
				})
			}
		}
	}

	// Now handle CNAMEs - first we need to get existing CNAME records
	var cnameRecords []sqlmodel.Record
	if err := db.Model(&sqlmodel.Record{}).Where("type = ?", "CNAME").Find(&cnameRecords).Error; err != nil {
		return err
	}

	// Create a map for CNAME lookups
	// Normalize keys to ensure consistent lookups
	cnameMap := make(map[string]sqlmodel.Record)
	for _, record := range cnameRecords {
		cnameMap[normalizeHostname(record.Name)] = record
	}

	// Create a map of hostnames for resolving CNAMEs
	// Normalize hostnames to ensure consistent lookups
	hostnameMap := make(map[string]bool)
	for _, hostmap := range hostmaps {
		if hostmap.removalCode == NotRemoved {
			for _, fqdn := range hostmap.fqdns {
				hostnameMap[normalizeHostname(fqdn)] = true
			}
		}
	}

	// Build a map of all CNAME targets from config for CNAME chaining support
	// This allows CNAME -> CNAME -> A record chains
	cnameTargetMap := make(map[string]bool)
	for _, cname := range config.Processing.Cnames {
		cnameTargetMap[normalizeHostname(cname.Cname)] = true
	}

	// Process CNAME records
	// We may need multiple passes to resolve CNAME chains, so keep trying until no progress
	processedCnames := make(map[string]bool)
	maxIterations := len(config.Processing.Cnames) + 1 // Prevent infinite loops

	for iteration := 0; iteration < maxIterations; iteration++ {
		progressMade := false

		for _, cname := range config.Processing.Cnames {
			normalizedCname := normalizeHostname(cname.Cname)

			// Skip if already processed
			if processedCnames[normalizedCname] {
				continue
			}

			normalizedTarget := normalizeHostname(cname.Hostname)

			// Check if target exists as an A record OR as a CNAME we've already processed
			// This allows CNAME chaining: CNAME -> CNAME -> A
			targetExists := hostnameMap[normalizedTarget] || processedCnames[normalizedTarget]

			if targetExists {
				// CNAME content must have trailing dot for PowerDNS
				cnameContent := normalizeDNSName(cname.Hostname)

				if record, ok := cnameMap[normalizedCname]; ok {
					// Update existing CNAME if the target changed
					if normalizeHostname(record.Content) != normalizedTarget {
						record.Content = cnameContent
						updateRecords = append(updateRecords, record)
					}
				} else {
					// Create new CNAME record
					newRecords = append(newRecords, sqlmodel.Record{
						Name:    normalizedCname,
						Type:    "CNAME",
						Content: cnameContent,
						Ttl:     3600,
					})
				}

				processedCnames[normalizedCname] = true
				progressMade = true
			}
		}

		// If we didn't process any new CNAMEs this iteration, we're done
		if !progressMade {
			break
		}
	}

	// Warn about any unresolved CNAMEs
	for _, cname := range config.Processing.Cnames {
		normalizedCname := normalizeHostname(cname.Cname)
		if !processedCnames[normalizedCname] {
			logger.Warnf("CNAME target '%s' for '%s' not found in hosts, skipping database entry",
				cname.Hostname, cname.Cname)
		}
	}

	// use GORM to update the records in updateRecords
	// and insert the records in newRecords
	if len(updateRecords) > 0 {
		if err := db.Save(&updateRecords).Error; err != nil {
			return err
		}
		logger.Infof("Updated %d database records", len(updateRecords))
	} else {
		logger.Infof("No database records to update")
	}

	if len(newRecords) > 0 {
		if err := db.Create(&newRecords).Error; err != nil {
			return err
		}
		logger.Infof("Inserted %d database records", len(newRecords))
	} else {
		logger.Infof("No database records to insert")
	}

	return nil
}
