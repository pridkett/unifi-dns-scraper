package main

import (
	"flag"
	"os"
	"time"

	"github.com/naoina/toml"
	"github.com/pridkett/unifi-dns-scraper/scraper"
	"github.com/withmandala/go-log"
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

	for {
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

		hostmaps = scraper.GenerateHostsFile(&config, hostmaps)

		if config.Daemonize {
			sleep_dur := config.Sleep
			if sleep_dur == 0 {
				sleep_dur = 120
			}
			logger.Infof("Sleeping for %d seconds", sleep_dur)
			time.Sleep(time.Duration(sleep_dur) * time.Second)
		} else {
			break
		}
	}
}
