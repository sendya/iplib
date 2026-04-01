// Package iplib provides a high-performance, offline-capable GeoIP query
// library for IPv4 and IPv6 addresses.
//
// # Database Format
//
// iplib uses a compact binary format (.ipdb) composed of three sections:
//
//  1. A fixed 64-byte header containing the magic number, format version,
//     build timestamp, record counts, and data-section size.
//
//  2. Sorted IP-range index sections (separate tables for IPv4 and IPv6).
//     Each entry stores the start address, end address, and the offset+length
//     of the corresponding record in the data section.  Binary search over
//     these sorted tables gives O(log n) lookup time.
//
//  3. A data section of variable-length, JSON-encoded [Record] values.
//     Database metadata ([Meta]) is also stored here and its position is
//     recorded in the header.
//
// # Writing a database
//
//	w, err := iplib.NewWriter("geoip.ipdb", &iplib.Meta{
//	    DatabaseType: "GeoCity",
//	    Description:  "Example GeoIP database",
//	    BuildTime:    time.Now(),
//	})
//	if err != nil { ... }
//	defer w.Close()
//
//	if err := w.Add("1.0.0.0", "1.0.0.255", &iplib.Record{
//	    Country: "AU", CountryName: "Australia", City: "Brisbane",
//	}); err != nil { ... }
//
//	if err := w.Build(); err != nil { ... }
//
// # Reading a database
//
//	db, err := iplib.Open("geoip.ipdb")
//	if err != nil { ... }
//	defer db.Close()
//
//	rec, err := db.Lookup("1.0.0.1")
//	if err != nil { ... }
//	fmt.Println(rec.Country, rec.City)
package iplib
