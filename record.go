package iplib

import "time"

// Record holds the GeoIP information associated with an IP address or range.
// All fields are optional; unused fields are omitted when serialised to JSON.
type Record struct {
	// Country is the ISO 3166-1 alpha-2 country code (e.g. "US").
	Country string `json:"country,omitempty"`
	// CountryName is the human-readable country name (e.g. "United States").
	CountryName string `json:"country_name,omitempty"`
	// Region is the ISO 3166-2 region / subdivision code (e.g. "CA").
	Region string `json:"region,omitempty"`
	// RegionName is the human-readable region / state name.
	RegionName string `json:"region_name,omitempty"`
	// City is the city name.
	City string `json:"city,omitempty"`
	// PostalCode is the postal / ZIP code.
	PostalCode string `json:"postal_code,omitempty"`
	// Latitude is the geographic latitude in decimal degrees.
	Latitude float64 `json:"latitude,omitempty"`
	// Longitude is the geographic longitude in decimal degrees.
	Longitude float64 `json:"longitude,omitempty"`
	// TimeZone is the IANA time-zone identifier (e.g. "America/Los_Angeles").
	TimeZone string `json:"timezone,omitempty"`
	// ISP is the name of the Internet Service Provider.
	ISP string `json:"isp,omitempty"`
	// ASN is the Autonomous System Number.
	ASN uint32 `json:"asn,omitempty"`
	// ASOrg is the organisation name for the AS.
	ASOrg string `json:"as_org,omitempty"`
	// Extra holds any additional key/value fields not covered above.
	Extra map[string]string `json:"extra,omitempty"`
}

// Meta holds metadata that describes a database file.
type Meta struct {
	// DatabaseType is a short string identifying the kind of data stored
	// (e.g. "GeoCity", "GeoASN").
	DatabaseType string `json:"database_type"`
	// Description is a human-readable description of the database.
	Description string `json:"description,omitempty"`
	// BuildTime is the UTC time at which the database was built.
	BuildTime time.Time `json:"build_time"`
	// Languages is a list of BCP-47 language tags for which localised
	// names are available (informational only).
	Languages []string `json:"languages,omitempty"`
}
