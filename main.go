package main

import (
	"flag"
	"os"
	"time"

	"github.com/naoina/toml"
	"github.com/pridkett/unifi-dns-scraper/scraper"
	"github.com/withmandala/go-log"
	"gorm.io/gorm"
)

// set up a global logger...
// see: https://stackoverflow.com/a/43827612/57626
var logger *log.Logger

func main() {
	var config scraper.TomlConfig

	logger = log.New(os.Stderr).WithColor()
	scraper.SetLogger(logger)

	configFile := flag.String("config", "", "Filename with configuration")
	flag.Parse()

	var hostmaps = []*scraper.Hostmap{}
	var db *gorm.DB
	var err error

	loop_count := 0
	for {
		loop_count++
		logger.Infof("** Starting loop %d **", loop_count)
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

		// only connect to the database the first time through the loop
		if config.Database.Driver != "" && config.Database.DSN != "" && db == nil {
			db, err = scraper.OpenDatabase(config.Database.Driver, config.Database.DSN)
			if err != nil {
				logger.Fatalf("Fatal error opening database: %s", err)
			}

			// Ensure the database connection is closed when no longer needed
			sqlDB, err := db.DB()
			if err != nil {
				logger.Fatalf("Error trying to get underlying database connection: %s", err)
			}
			defer sqlDB.Close()
			logger.Infof("Database connection opened driver=%s", config.Database.Driver)
		}

		hostmaps, err := scraper.GenerateHostsFile(&config, hostmaps)
		if err != nil {
			logger.Fatalf("Fatal error generating hosts file: %s", err)
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
			logger.Infof("Sleeping for %d seconds", sleep_dur)
			logger.Infof("** Ending loop %d **", loop_count)
			time.Sleep(time.Duration(sleep_dur) * time.Second)
		} else {
			break
		}
	}
}
