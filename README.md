# unifi-dns-scraper

Patrick Wagstrom &lt;patrick@wagstrom.net&gt;

December 2021 (Updated March 2025)

## Overview

This program is _incredibly_ niche. If you are running a Unifi network, and you are running a local DNS server outside of the Unifi equipment such as Pi-Hole or AdGuard Home, and that system can read a `hosts.txt` file or use PowerDNS with a database backend, then this program might be useful to you.

## Features

- Retrieves device information from a Unifi Controller
- Generates hostname entries in multiple domains
- Outputs DNS records to a hosts file
- Saves DNS records to a SQL database (PowerDNS format)
- Supports filtering by MAC address and specific blocklists
- Handles stale entries with configurable timeouts

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/pridkett/unifi-dns-scraper.git
cd unifi-dns-scraper

# Build the application
go build

# Run the tests
go test ./...
```

### Using Docker

```bash
# Build the Docker image
docker build -t unifi-dns-scraper .

# Run with Docker
docker run -v $(pwd)/config.toml:/app/config.toml -v $(pwd)/hosts.txt:/app/hosts.txt unifi-dns-scraper
```

## Usage

```bash
./unifi-dns-scraper -config config.toml
```

## Configuration

You'll need to create a file called `config.toml` that has the configuration and credentials needed to connect to your Unifi system.

### Global Settings
There are a couple of global settings that affect the overall execution of the program.

* **`Daemonize`**: A boolean (true/false) about whether or not the application should continue to run forever. Yes, I know this isn't the actual Unix definition of daemon.
* **`Sleep`**: An integer for the number of seconds between polling your Unifi system for IP addresses.
* **`MaxAge`**: An integer for the maximum number of seconds to keep stale entries in your hosts file. If this is not set, then stale hosts will never time out (or rather time out whenever the application gets restarted).

### The **`[unifi]`** block

This block contains all of the information necessary to connect to your Unifi system. This needs to be a local account. I recommend creating a one off account just for this purpose.

* **`host`**: A string for the URL of the Unifi system to connect to. This should be something like `https://192.168.1.1`, or if you're fancy and have a hostname for your console, you can put that here.
* **`user`**: A string for the username to connect to the Unifi system. This should be a local account.
* **`password`**: A string for the password for the account. As this is stored in plaintext, this is part of the reason why I recommend a throwaway account.

### The **`[processing]`** block

This block contains settings for processing the hostname data:

* **`domains`**: A list of strings that represent the domains that will be appended to each of the hostnames.
* **`additional`**: A list of objects, each containing an `ip` and `name` field. This can be used to inject additional hostnames into your host file for systems that don't appear in the Unifi interface.
* **`blocked`**: A list of objects, with an `ip` and `name`, or just an `ip`, or just a `name` that is used to block those entries from appearing in your output. The use case for this is that that I have a device that keeps on bouncing over to another IP address and I don't want that entry appearing in my host file. This can also be used to ensure that some devices don't get hostnames in the file for other reasons.
* **`keep_macs`**: A boolean (`true`/`false`) that indicates whether or not hostnames that are returned as MAC addresses should be included. Defaults to `false`. I haven't yet figured out what causes this, but hostnames with colons are not valid hostnames.

### The **`[hostsfile]`** block

This block contains the settings for generation of the hosts file.

* **`filename`**: A string for the output file path

### The **`[database]`** block

This block contains settings for database connectivity:

* **`driver`**: The database driver to use. Currently supports `sqlite` and `mysql`.
* **`dsn`**: The Data Source Name (connection string) for the database.

For SQLite, the DSN is a path to the database file, e.g. `database.db` or `:memory:` for an in-memory database.
For MySQL, the DSN format is `username:password@tcp(host:port)/dbname?parseTime=true`.

### Example Configuration

The following is an example configuration file that will create entries in three domains - `example.local`, `device.example.local`, and `home.local` for each of the hosts that appears in your Unifi controller and saves them both to a hosts file and an SQLite database.

```toml
# Unifi DNS Scraper Sample Configuration File
Daemonize = true
Sleep = 60

[processing]
domains = ["example.local", "device.example.local", "home.local"]
additional = [{ ip = "192.168.1.1", name = "unifi" }]
blocked = [{ ip = "192.168.90.2", name = "naughtyhost" }]
keep_macs = false

[unifi]
host = "https://192.168.1.1/"
user = "dns-scraper-admin-account"
password = "your-secure-password"

[hostsfile]
filename = "hosts.txt"

[database]
driver = "sqlite"
dsn = "dns-records.db"
```

## Development and Testing

### Running Tests

To run all tests:

```bash
go test ./...
```

To run tests with coverage:

```bash
go test -cover ./...
```

To generate a coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Organization

The test suite includes:

1. **Unit Tests**: Test individual functions and components
2. **Mock Tests**: Test functionality using mock implementations of the Unifi API
3. **Database Tests**: Test database operations with an in-memory SQLite database
4. **Integration Tests**: Test workflow from data retrieval to output generation

## Continuous Integration and Releases

This project uses GitHub Actions for continuous integration and automated releases:

### Automated Tests

Tests are automatically run on every push to the main branch and on every pull request.

### Creating a Release

To create a new release:

1. Tag a commit with the version number using semantic versioning:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. GitHub Actions will automatically:
   - Create a new GitHub release
   - Build binaries for multiple platforms:
     - Linux (x86_64, arm64, armv7)
     - macOS (x86_64, arm64)
   - Attach the compiled binaries to the GitHub release

### Cross-Platform Binaries

The release process creates statically linked binaries for the following platforms:
- Linux x86_64 (64-bit Intel/AMD)
- Linux arm64 (64-bit ARM)
- Linux armv7 (32-bit ARM)
- macOS x86_64 (Intel Mac)
- macOS arm64 (Apple Silicon)

## License

Copyright (c) 2023 Patrick Wagstrom

Licensed under terms of the MIT License
