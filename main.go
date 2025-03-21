//go:build !standalone_test
// +build !standalone_test

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/naoina/toml"
	"github.com/pridkett/unifi-dns-scraper/scraper"
	"github.com/withmandala/go-log"
	"gorm.io/gorm"
)

// Version information set by build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// set up a global logger...
// see: https://stackoverflow.com/a/43827612/57626
var globalLogger *log.Logger

func main() {
	var config scraper.TomlConfig

	globalLogger = log.New(os.Stderr).WithColor()
	scraper.SetLogger(globalLogger)

	// Command line flags
	configFile := flag.String("config", "", "Filename with configuration")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("unifi-dns-scraper version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built at: %s\n", date)
		os.Exit(0)
	}

	var hostmaps = []*scraper.Hostmap{}
	var db *gorm.DB
	var err error

	loop_count := 0
	for {
		loop_count++
		globalLogger.Infof("** Starting loop %d **", loop_count)
		if *configFile != "" {
			globalLogger.Infof("opening configuration file: %s", *configFile)
			f, err := os.Open(*configFile)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			if err := toml.NewDecoder(f).Decode(&config); err != nil {
				panic(err)
			}

			// Update config from environment variables (environment variables will override TOML values)
			scraper.UpdateConfigFromEnv(&config)
		} else {
			globalLogger.Fatal("Must specify configuration file with -config FILENAME")
		}

		// only connect to the database the first time through the loop
		if config.Database.Driver != "" && config.Database.DSN != "" && db == nil {
			db, err = scraper.OpenDatabase(config.Database.Driver, config.Database.DSN)
			if err != nil {
				globalLogger.Fatalf("Fatal error opening database: %s", err)
			}

			// Ensure the database connection is closed when no longer needed
			sqlDB, err := db.DB()
			if err != nil {
				globalLogger.Fatalf("Error trying to get underlying database connection: %s", err)
			}
			defer sqlDB.Close()
			globalLogger.Infof("Database connection opened driver=%s", config.Database.Driver)
		}

		hostmaps, err = scraper.GenerateHostsFile(&config, hostmaps)
		if err != nil {
			globalLogger.Fatalf("Fatal error generating hosts file: %s", err)
		}

		if config.Hostsfile != (scraper.HostsfileConfig{}) {
			scraper.SaveHostsFile(hostmaps, &config)
		}

		if config.Database != (scraper.DatabaseConfig{}) {
			scraper.SaveDatabase(db, hostmaps, &config)
		}

		if config.Daemonize {
			sleep_dur := config.Sleep
			if sleep_dur == 0 {
				sleep_dur = 120
			}
			globalLogger.Infof("Sleeping for %d seconds", sleep_dur)
			globalLogger.Infof("** Ending loop %d **", loop_count)
			time.Sleep(time.Duration(sleep_dur) * time.Second)
		} else {
			break
		}
	}
}
