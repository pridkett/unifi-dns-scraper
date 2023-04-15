unifi-dns-scraper
=================

Patrick Wagstrom &lt;patrick@wagstrom.net&gt;

December 2021

Overview
--------

This program is _incredibly_ niche. If you are running a Unifi network, and you are running a local DNS server outside of the Unifi equipment such as Pi-Hole or AdGuard Home, and that system can read a `hosts.txt` file, then this program might be useful to you.

Usage
-----

```bash
./unifi-dns-scraper -config config.toml
```

Configuration
-------------

You'll need to create a file called `config.toml` that has the configuration and credentials needed to connect to your Unifi system.

### Global Settings
There are a couple of global settings that affect the overall execution of the program.

* **`Daemonize`**: A boolean (true/false) about whether or not the application should continue to run forever. Yes, I know this isn't the actual Unix definition of daemon.
* **`Sleep`**: An integer for the number of seconds between polling your Unifi system for IP addresses.
* **`MaxAge`**: An integer for the maximum number of seconds to keep stale entries in your hosts file. If this is not set, then stale hosts will never time out (or rather time out whenever the application gets restarted).

### The **`[unifi]`** block

This block contains all of the information necessary to connect to your Unifi system. This needs to be a local account. I recommend creating a one off account just for this purpose.

* **`host`**: A string for the URL of the Unifi system to connect to. This should be seomthing like `https://192.168.1.1`, or if you're fancy and have a hostname for your console, you can put that here.
* **`user`**: A string for the username to connect to the Unifi system. This should be a local account.
* **`password`**: A string for the password for the account. As this is stored in plaintext, this is part of the reason why I recommend a throwaway account.

### The **`[hostsfile]`** block

This block contains the settings for generation of the hosts file.

* **`domains`**: A list of strings that represent the domains that will be appended to each of the hostnames.
* **`filename`**: A string for the output file
* **`additional`**: A list of objects, each containing an `ip` and `name` field. This can be used to inject additional hostnames into your host file for systems that don't appear in the Unifi interface.
* **`blocked`**: A list of objects, with an `ip` and `name`, or just an `ip`, or just a `name` that is used to block those entries from appearing in your host file output. The use case for this is that that I have a Tesla Powerwall Gateway that keeps on bouncing over to 192.168.90.2 for some reason and I don't want that entry appearing in my host file. This can also be used to ensure that some devices don't get hostnames in the file for other reasons.
* **`keep_macs`**: A boolean (`true`/`false`) that indicates whether or not hostnames that are returned as MAC addresses should be included. Defaults to `false`. I haven't yet figured out what causes this, but hostnames with colons are not valid hostnames.

### Example Configuration

The following is an example configuration file that will create entries in three domains - `example.com`, `yourhome.example.com`, and `local` for each of the hosts that appears in your Unifi controller. It also creates an additional entry for `unifi` in each of the domains because the controller doesn't return information for themselves and I haven't found an easy way to do this yet.

```toml
# Unifi DNS Scraper Sample Configuration File
Daemonize = true
Sleep = 60

[unifi]
host = "https://unifi.example.com/"
user = "dns-scraper"
password = "DNS-scraper-PASSWORD-123!@#"

[hostsfile]
domains = ["example.come", "yourhome.example.com", "local"]
filename = "hosts.txt"
additional = [{ ip="192.168.1.1", name="unifi" }]
blocked = [{ ip="192.168.90.2", name="powerwall" }]
keep_macs = false
```

License
-------

Copyright (c) 2023 Patrick Wagstrom

Licensed under terms of the MIT License