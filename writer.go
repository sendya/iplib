package iplib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
)

// Writer builds an iplib database file from a set of IP-range records.
//
// Call NewWriter to create a Writer, Add to insert records, Build to finalise
// and write the file, and Close to release resources.
//
//	w, err := iplib.NewWriter("out.ipdb", meta)
//	w.Add("1.0.0.0", "1.0.0.255", rec)
//	w.Build()
//	w.Close()
type Writer struct {
	path string
	meta *Meta

	entries4 []writerEntry4
	entries6 []writerEntry6
	built    bool
}

type writerEntry4 struct {
	StartIP uint32
	EndIP   uint32
	Record  *Record
}

type writerEntry6 struct {
	StartIP [16]byte
	EndIP   [16]byte
	Record  *Record
}

// NewWriter returns a Writer that will create (or overwrite) the file at path.
// meta must not be nil.
func NewWriter(path string, meta *Meta) (*Writer, error) {
	if meta == nil {
		return nil, errors.New("iplib: meta must not be nil")
	}
	return &Writer{path: path, meta: meta}, nil
}

// Add registers a GeoIP record for the inclusive IP range [startStr, endStr].
// Both addresses must be of the same family (both IPv4 or both IPv6).
// IPv4-mapped IPv6 addresses are treated as IPv4.
func (w *Writer) Add(startStr, endStr string, rec *Record) error {
	if w.built {
		return errors.New("iplib: cannot Add after Build")
	}
	if rec == nil {
		return errors.New("iplib: record must not be nil")
	}

	startIP := net.ParseIP(startStr)
	endIP := net.ParseIP(endStr)
	if startIP == nil {
		return fmt.Errorf("iplib: %q is not a valid IP address", startStr)
	}
	if endIP == nil {
		return fmt.Errorf("iplib: %q is not a valid IP address", endStr)
	}

	start4 := startIP.To4()
	end4 := endIP.To4()

	if (start4 == nil) != (end4 == nil) {
		return errors.New("iplib: start and end IP must be in the same address family")
	}

	if start4 != nil {
		s := ip4ToUint32(start4)
		e := ip4ToUint32(end4)
		if s > e {
			return fmt.Errorf("iplib: start IP %s is after end IP %s", startStr, endStr)
		}
		w.entries4 = append(w.entries4, writerEntry4{StartIP: s, EndIP: e, Record: rec})
		return nil
	}

	// IPv6
	var s, e [16]byte
	copy(s[:], startIP.To16())
	copy(e[:], endIP.To16())
	if compareIP6(s, e) > 0 {
		return fmt.Errorf("iplib: start IP %s is after end IP %s", startStr, endStr)
	}
	w.entries6 = append(w.entries6, writerEntry6{StartIP: s, EndIP: e, Record: rec})
	return nil
}

// AddNet registers a GeoIP record for the IP network cidr.
func (w *Writer) AddNet(cidr string, rec *Record) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("iplib: invalid CIDR %q: %w", cidr, err)
	}
	start := ipNet.IP
	// Compute broadcast / last address.
	end := make(net.IP, len(start))
	copy(end, start)
	for i := range end {
		end[i] |= ^ipNet.Mask[i]
	}
	return w.Add(start.String(), end.String(), rec)
}

// Build writes the database file to disk. It may be called only once.
func (w *Writer) Build() error {
	if w.built {
		return errors.New("iplib: Build has already been called")
	}
	w.built = true

	// Sort IPv4 entries by StartIP.
	sort.Slice(w.entries4, func(i, j int) bool {
		return w.entries4[i].StartIP < w.entries4[j].StartIP
	})
	// Sort IPv6 entries by StartIP.
	sort.Slice(w.entries6, func(i, j int) bool {
		return compareIP6(w.entries6[i].StartIP, w.entries6[j].StartIP) < 0
	})

	// Build data section: encode each unique record as JSON.
	// We deduplicate records that are pointer-equal.
	var dataBuf []byte
	type dataRef struct {
		offset uint32
		length uint16
	}
	recordCache := map[*Record]dataRef{}

	refFor := func(rec *Record) (dataRef, error) {
		if ref, ok := recordCache[rec]; ok {
			return ref, nil
		}
		b, err := json.Marshal(rec)
		if err != nil {
			return dataRef{}, fmt.Errorf("iplib: encode record: %w", err)
		}
		if len(b) > 65535 {
			return dataRef{}, errors.New("iplib: record JSON exceeds 64 KiB limit")
		}
		ref := dataRef{offset: uint32(len(dataBuf)), length: uint16(len(b))}
		dataBuf = append(dataBuf, b...)
		recordCache[rec] = ref
		return ref, nil
	}

	// Pre-encode all records (IPv4 first, then IPv6).
	refs4 := make([]dataRef, len(w.entries4))
	for i, e := range w.entries4 {
		ref, err := refFor(e.Record)
		if err != nil {
			return err
		}
		refs4[i] = ref
	}
	refs6 := make([]dataRef, len(w.entries6))
	for i, e := range w.entries6 {
		ref, err := refFor(e.Record)
		if err != nil {
			return err
		}
		refs6[i] = ref
	}

	// Encode metadata.
	metaBytes, err := json.Marshal(w.meta)
	if err != nil {
		return fmt.Errorf("iplib: encode metadata: %w", err)
	}
	metaOffset := uint32(len(dataBuf))
	metaLength := uint32(len(metaBytes))
	dataBuf = append(dataBuf, metaBytes...)

	// Determine flags.
	var flags uint8
	if len(w.entries4) > 0 {
		flags |= flagHasIPv4
	}
	if len(w.entries6) > 0 {
		flags |= flagHasIPv6
	}

	// Build header.
	hdr := &dbHeader{
		Version:    fileVersion,
		Flags:      flags,
		BuildTime:  w.meta.BuildTime.Unix(),
		IPv4Count:  uint32(len(w.entries4)),
		IPv6Count:  uint32(len(w.entries6)),
		DataSize:   uint32(len(dataBuf)),
		MetaOffset: metaOffset,
		MetaLength: metaLength,
	}
	copy(hdr.Magic[:], Magic)

	// Serialise to a single buffer.
	out := encodeHeader(hdr)

	// IPv4 index.
	for i, e := range w.entries4 {
		out = encodeIndex4(out, e.StartIP, e.EndIP, refs4[i].offset, refs4[i].length)
	}
	// IPv6 index.
	for i, e := range w.entries6 {
		out = encodeIndex6(out, e.StartIP, e.EndIP, refs6[i].offset, refs6[i].length)
	}
	// Data section.
	out = append(out, dataBuf...)

	return os.WriteFile(w.path, out, 0o644)
}

// Close is a no-op that exists to support deferred cleanup patterns.
func (w *Writer) Close() error { return nil }
