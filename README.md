# iplib

A high-performance, offline-capable GeoIP query library for Go, supporting
both IPv4 and IPv6 addresses.

## Features

- **Custom binary format** (`.ipdb`) — compact, portable, no external dependencies
- **O(log n) lookup** — binary search over sorted IP-range index tables
- **IPv4 and IPv6** — separate index sections in a single file; IPv4-mapped
  IPv6 addresses (`::ffff:x.x.x.x`) resolve transparently via the IPv4 index
- **Offline** — the entire database is read into memory at open time; no
  network access is ever required
- **Extensible records** — strongly-typed core fields plus a free-form
  `Extra map[string]string` for custom data
- **Thread-safe** — the opened `DB` is read-only and safe for concurrent use

## Installation

```bash
go get github.com/sendya/iplib
```

## Quick start

### Building a database

```go
package main

import (
    "log"
    "time"

    "github.com/sendya/iplib"
)

func main() {
    meta := &iplib.Meta{
        DatabaseType: "GeoCity",
        Description:  "Example GeoIP database",
        BuildTime:    time.Now(),
    }

    w, err := iplib.NewWriter("geoip.ipdb", meta)
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    // Add records by IP range.
    _ = w.Add("1.0.0.0", "1.0.0.255", &iplib.Record{
        Country:     "AU",
        CountryName: "Australia",
        City:        "Brisbane",
        Latitude:    -27.47,
        Longitude:   153.02,
    })

    // Add records by CIDR.
    _ = w.AddNet("8.8.8.0/24", &iplib.Record{
        Country: "US",
        ISP:     "Google LLC",
        ASN:     15169,
        ASOrg:   "Google LLC",
    })

    if err := w.Build(); err != nil {
        log.Fatal(err)
    }
}
```

### Querying a database

```go
package main

import (
    "fmt"
    "log"
    "net"

    "github.com/sendya/iplib"
)

func main() {
    db, err := iplib.Open("geoip.ipdb")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Lookup by string.
    rec, err := db.Lookup("1.0.0.1")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rec.Country, rec.City) // AU Brisbane

    // Lookup by net.IP.
    rec, err = db.LookupIP(net.ParseIP("8.8.8.8"))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rec.ASN, rec.ISP) // 15169 Google LLC

    // Lookup from an HTTP request RemoteAddr (host:port).
    rec, err = db.LookupNet("8.8.8.8:54321")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(rec.Country) // US
}
```

## Database file format

```
┌───────────────────────────────┐
│  Header  (64 bytes, fixed)    │  magic · version · flags · build time
│                               │  IPv4/IPv6 counts · data-section size
│                               │  metadata offset + length
├───────────────────────────────┤
│  IPv4 Index  (n × 14 bytes)   │  sorted [startIP · endIP · offset · len]
├───────────────────────────────┤
│  IPv6 Index  (n × 38 bytes)   │  sorted [startIP · endIP · offset · len]
├───────────────────────────────┤
│  Data Section  (variable)     │  JSON-encoded Record values + metadata
└───────────────────────────────┘
```

All integers in the header and index are little-endian.  
IP addresses in the index are stored in big-endian (network) byte order so
that byte-level comparison preserves the natural ordering.

## API reference

| Symbol | Description |
|---|---|
| `Open(path) (*DB, error)` | Open a database file |
| `DB.Lookup(ip string) (*Record, error)` | Lookup by IP string |
| `DB.LookupIP(ip net.IP) (*Record, error)` | Lookup by `net.IP` |
| `DB.LookupNet(addr string) (*Record, error)` | Lookup from `host:port` |
| `DB.Meta() *Meta` | Return database metadata |
| `DB.Close() error` | Release resources |
| `NewWriter(path, meta) (*Writer, error)` | Create a database builder |
| `Writer.Add(start, end, rec) error` | Add a record for an IP range |
| `Writer.AddNet(cidr, rec) error` | Add a record for a CIDR block |
| `Writer.Build() error` | Write and finalise the file |
| `ErrNotFound` | Sentinel error returned when no record matches |

## License

MIT
