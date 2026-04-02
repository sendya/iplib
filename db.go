package iplib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
)

// DB is an open, read-only iplib database. It is safe for concurrent use
// from multiple goroutines after it has been opened.
type DB struct {
	meta *Meta
	// IPv4 index, sorted ascending by StartIP.
	idx4 []idxEntry4
	// IPv6 index, sorted ascending by StartIP.
	idx6 []idxEntry6
	// data is the raw data section; index entries reference slices of it.
	data []byte
}

// idxEntry4 is one row in the in-memory IPv4 lookup table.
type idxEntry4 struct {
	StartIP    uint32
	EndIP      uint32
	DataOffset uint32
	DataLength uint16
}

// idxEntry6 is one row in the in-memory IPv6 lookup table.
type idxEntry6 struct {
	StartIP    [16]byte
	EndIP      [16]byte
	DataOffset uint32
	DataLength uint16
}

// Open reads the file at path into memory and returns a ready-to-use DB.
func Open(path string) (*DB, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("iplib: open %q: %w", path, err)
	}
	return openBytes(raw)
}

// openBytes parses an in-memory .ipdb blob and returns a DB.
func openBytes(raw []byte) (*DB, error) {
	h, err := decodeHeader(raw)
	if err != nil {
		return nil, err
	}

	// Compute section offsets.
	idx4Start := headerSize
	idx4End := idx4Start + int(h.IPv4Count)*indexEntry4Size
	idx6Start := idx4End
	idx6End := idx6Start + int(h.IPv6Count)*indexEntry6Size
	dataStart := idx6End
	dataEnd := dataStart + int(h.DataSize)

	if dataEnd > len(raw) {
		return nil, errors.New("iplib: file is truncated")
	}

	db := &DB{
		data: raw[dataStart:dataEnd],
		idx4: make([]idxEntry4, h.IPv4Count),
		idx6: make([]idxEntry6, h.IPv6Count),
	}

	// Parse IPv4 index.
	idx4Buf := raw[idx4Start:idx4End]
	for i := range db.idx4 {
		off := i * indexEntry4Size
		e := &db.idx4[i]
		e.StartIP = le.Uint32(idx4Buf[off : off+4])
		e.EndIP = le.Uint32(idx4Buf[off+4 : off+8])
		e.DataOffset = le.Uint32(idx4Buf[off+8 : off+12])
		e.DataLength = le.Uint16(idx4Buf[off+12 : off+14])
	}

	// Parse IPv6 index.
	idx6Buf := raw[idx6Start:idx6End]
	for i := range db.idx6 {
		off := i * indexEntry6Size
		e := &db.idx6[i]
		copy(e.StartIP[:], idx6Buf[off:off+16])
		copy(e.EndIP[:], idx6Buf[off+16:off+32])
		e.DataOffset = le.Uint32(idx6Buf[off+32 : off+36])
		e.DataLength = le.Uint16(idx6Buf[off+36 : off+38])
	}

	// Parse metadata.
	if h.MetaLength > 0 {
		metaSlice := db.data[h.MetaOffset : h.MetaOffset+h.MetaLength]
		db.meta = &Meta{}
		if err := json.Unmarshal(metaSlice, db.meta); err != nil {
			return nil, fmt.Errorf("iplib: parse metadata: %w", err)
		}
	}

	return db, nil
}

// Close releases resources held by the DB. After Close, the DB must not be used.
func (db *DB) Close() error {
	db.data = nil
	db.idx4 = nil
	db.idx6 = nil
	return nil
}

// Meta returns the database metadata, or nil if none was stored.
func (db *DB) Meta() *Meta {
	return db.meta
}

// Lookup parses ipStr and returns the GeoIP record for that address.
// It accepts both IPv4 and IPv6 addresses in any format accepted by
// net.ParseIP.
func (db *DB) Lookup(ipStr string) (*Record, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("iplib: %q is not a valid IP address", ipStr)
	}
	return db.LookupIP(ip)
}

// LookupNet parses the host part of addr (e.g. from an http.Request.RemoteAddr)
// and returns the GeoIP record for that address.
func (db *DB) LookupNet(addr string) (*Record, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// addr may not have a port; try parsing as a bare IP.
		host = addr
	}
	return db.Lookup(host)
}

// LookupIP returns the GeoIP record for ip. IPv4-mapped IPv6 addresses
// (::ffff:x.x.x.x) are looked up in the IPv4 index.
func (db *DB) LookupIP(ip net.IP) (*Record, error) {
	// Normalise to 16-byte form; check for IPv4-mapped.
	ip16 := ip.To16()
	if ip16 == nil {
		return nil, fmt.Errorf("iplib: invalid IP %v", ip)
	}
	if ip4 := ip.To4(); ip4 != nil {
		return db.lookup4(ip4)
	}
	return db.lookup6(ip16)
}

// lookup4 searches the IPv4 index for ip (a 4-byte slice).
func (db *DB) lookup4(ip net.IP) (*Record, error) {
	needle := ip4ToUint32(ip)
	// Binary search: find the last entry whose StartIP <= needle.
	n := len(db.idx4)
	i := sort.Search(n, func(i int) bool {
		return db.idx4[i].StartIP > needle
	}) - 1
	if i < 0 {
		return nil, ErrNotFound
	}
	e := &db.idx4[i]
	if needle < e.StartIP || needle > e.EndIP {
		return nil, ErrNotFound
	}
	return db.decodeRecord(e.DataOffset, e.DataLength)
}

// lookup6 searches the IPv6 index for ip (a 16-byte slice).
func (db *DB) lookup6(ip net.IP) (*Record, error) {
	var needle [16]byte
	copy(needle[:], ip)

	n := len(db.idx6)
	i := sort.Search(n, func(i int) bool {
		return compareIP6(db.idx6[i].StartIP, needle) > 0
	}) - 1
	if i < 0 {
		return nil, ErrNotFound
	}
	e := &db.idx6[i]
	if compareIP6(needle, e.StartIP) < 0 || compareIP6(needle, e.EndIP) > 0 {
		return nil, ErrNotFound
	}
	return db.decodeRecord(e.DataOffset, e.DataLength)
}

// decodeRecord decodes the JSON record at dataOffset with dataLength bytes.
func (db *DB) decodeRecord(dataOffset uint32, dataLength uint16) (*Record, error) {
	end := int(dataOffset) + int(dataLength)
	if end > len(db.data) {
		return nil, errors.New("iplib: data section is corrupted (record out of bounds)")
	}
	rec := &Record{}
	if err := json.Unmarshal(db.data[dataOffset:end], rec); err != nil {
		return nil, fmt.Errorf("iplib: decode record: %w", err)
	}
	rec.ContinentCode = GetContinentByCountry(rec.Country)
	// CN only
	if rec.Country == "CN" {
		rec.ContinentName = GetContinentName(rec.ContinentCode).NameCN
	} else {
		rec.ContinentName = GetContinentName(rec.ContinentCode).Name
	}
	return rec, nil
}

// ErrNotFound is returned by Lookup when no record covers the queried IP.
var ErrNotFound = errors.New("iplib: no record found for this IP address")
