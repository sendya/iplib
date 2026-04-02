package iplib_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sendya/iplib"
)

// buildTime is a fixed timestamp used in all tests so output is deterministic.
var buildTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

var testMeta = &iplib.Meta{
	DatabaseType: "GeoCity-Test",
	Description:  "Test database",
	BuildTime:    buildTime,
	Languages:    []string{"en"},
}

// newTmpDB builds an .ipdb file in a temp directory and returns its path.
func newTmpDB(t *testing.T, setup func(w *iplib.Writer)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.ipdb")
	w, err := iplib.NewWriter(path, testMeta)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	setup(w)
	if err := w.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	w.Close()
	return path
}

// openDB opens the database at path and fails the test on error.
func openDB(t *testing.T, path string) *iplib.DB {
	t.Helper()
	db, err := iplib.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ── IPv4 round-trip ──────────────────────────────────────────────────────────

func TestIPv4_LookupHit(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{
			Country: "AU", CountryName: "Australia", City: "Brisbane",
			Latitude: -27.47, Longitude: 153.02,
		})
		mustAdd(t, w, "8.8.8.0", "8.8.8.255", &iplib.Record{
			Country: "US", ISP: "Google LLC", ASN: 15169,
		})
	})
	db := openDB(t, path)

	cases := []struct {
		ip      string
		country string
		city    string
	}{
		{"1.0.0.1", "AU", "Brisbane"},
		{"1.0.0.0", "AU", "Brisbane"},
		{"1.0.0.255", "AU", "Brisbane"},
		{"8.8.8.8", "US", ""},
	}
	for _, tc := range cases {
		rec, err := db.Lookup(tc.ip)
		if err != nil {
			t.Errorf("Lookup(%s): unexpected error: %v", tc.ip, err)
			continue
		}
		if rec.Country != tc.country {
			t.Errorf("Lookup(%s).Country = %q, want %q", tc.ip, rec.Country, tc.country)
		}
		if rec.City != tc.city {
			t.Errorf("Lookup(%s).City = %q, want %q", tc.ip, rec.City, tc.city)
		}
	}
}

func TestIPv4_LookupMiss(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{Country: "AU"})
	})
	db := openDB(t, path)

	misses := []string{"0.255.255.255", "1.0.1.0", "8.8.8.8", "255.255.255.255"}
	for _, ip := range misses {
		_, err := db.Lookup(ip)
		if !errors.Is(err, iplib.ErrNotFound) {
			t.Errorf("Lookup(%s): want ErrNotFound, got %v", ip, err)
		}
	}
}

func TestIPv4_MultipleRanges(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		// Add in non-sorted order to verify Writer sorts them.
		mustAdd(t, w, "200.0.0.0", "200.0.0.255", &iplib.Record{Country: "BR"})
		mustAdd(t, w, "1.0.0.0", "1.255.255.255", &iplib.Record{Country: "AU"})
		mustAdd(t, w, "100.0.0.0", "100.0.0.255", &iplib.Record{Country: "US"})
	})
	db := openDB(t, path)

	want := map[string]string{
		"1.0.0.1":     "AU",
		"1.255.255.0": "AU",
		"100.0.0.1":   "US",
		"200.0.0.200": "BR",
	}
	for ip, country := range want {
		rec, err := db.Lookup(ip)
		if err != nil {
			t.Errorf("Lookup(%s): %v", ip, err)
			continue
		}
		if rec.Country != country {
			t.Errorf("Lookup(%s).Country = %q, want %q", ip, rec.Country, country)
		}
	}
}

func TestIPv4_ASN(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "8.8.8.0", "8.8.8.255", &iplib.Record{
			ASN: 15169, ASOrg: "Google LLC",
		})
	})
	db := openDB(t, path)

	rec, err := db.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.ASN != 15169 {
		t.Errorf("ASN = %d, want 15169", rec.ASN)
	}
	if rec.ASOrg != "Google LLC" {
		t.Errorf("ASOrg = %q, want %q", rec.ASOrg, "Google LLC")
	}
}

func TestIPv4_ExtraFields(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "10.0.0.0", "10.255.255.255", &iplib.Record{
			Country: "ZZ",
			Extra:   map[string]string{"datacenter": "us-east-1", "type": "hosting"},
		})
	})
	db := openDB(t, path)

	rec, err := db.Lookup("10.1.2.3")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Extra["datacenter"] != "us-east-1" {
		t.Errorf("Extra[datacenter] = %q, want %q", rec.Extra["datacenter"], "us-east-1")
	}
}

// ── IPv6 round-trip ──────────────────────────────────────────────────────────

func TestIPv6_LookupHit(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "2001:db8::", "2001:db8::ffff", &iplib.Record{
			Country: "US", City: "Test City",
		})
		mustAdd(t, w, "2400:cb00::", "2400:cb00::ffff", &iplib.Record{
			Country: "US", ISP: "Cloudflare",
		})
	})
	db := openDB(t, path)

	cases := []struct {
		ip      string
		country string
	}{
		{"2001:db8::1", "US"},
		{"2001:db8::ffff", "US"},
		{"2400:cb00::1", "US"},
	}
	for _, tc := range cases {
		rec, err := db.Lookup(tc.ip)
		if err != nil {
			t.Errorf("Lookup(%s): %v", tc.ip, err)
			continue
		}
		if rec.Country != tc.country {
			t.Errorf("Lookup(%s).Country = %q, want %q", tc.ip, rec.Country, tc.country)
		}
	}
}

func TestIPv6_LookupMiss(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "2001:db8::", "2001:db8::ffff", &iplib.Record{Country: "US"})
	})
	db := openDB(t, path)

	misses := []string{"2001:db7::1", "2001:db9::1", "::1"}
	for _, ip := range misses {
		_, err := db.Lookup(ip)
		if !errors.Is(err, iplib.ErrNotFound) {
			t.Errorf("Lookup(%s): want ErrNotFound, got %v", ip, err)
		}
	}
}

// ── Mixed IPv4 + IPv6 in one file ────────────────────────────────────────────

func TestMixed_IPv4andIPv6(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{Country: "AU"})
		mustAdd(t, w, "2001:db8::", "2001:db8::ffff", &iplib.Record{Country: "DE"})
	})
	db := openDB(t, path)

	rec4, err := db.Lookup("1.0.0.1")
	if err != nil {
		t.Fatalf("Lookup IPv4: %v", err)
	}
	if rec4.Country != "AU" {
		t.Errorf("Country = %q, want %q", rec4.Country, "AU")
	}

	rec6, err := db.Lookup("2001:db8::1")
	if err != nil {
		t.Fatalf("Lookup IPv6: %v", err)
	}
	if rec6.Country != "DE" {
		t.Errorf("Country = %q, want %q", rec6.Country, "DE")
	}
}

// ── LookupIP and LookupNet ───────────────────────────────────────────────────

func TestLookupIP(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "8.8.8.0", "8.8.8.255", &iplib.Record{Country: "US"})
	})
	db := openDB(t, path)

	ip := net.ParseIP("8.8.8.8")
	rec, err := db.LookupIP(ip)
	if err != nil {
		t.Fatalf("LookupIP: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("Country = %q, want %q", rec.Country, "US")
	}
}

func TestLookupNet(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "8.8.8.0", "8.8.8.255", &iplib.Record{Country: "US"})
	})
	db := openDB(t, path)

	rec, err := db.LookupNet("8.8.8.8:12345")
	if err != nil {
		t.Fatalf("LookupNet: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("Country = %q, want %q", rec.Country, "US")
	}
}

// ── AddNet ───────────────────────────────────────────────────────────────────

func TestAddNet_IPv4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.ipdb")
	w, _ := iplib.NewWriter(path, testMeta)
	if err := w.AddNet("8.8.8.0/24", &iplib.Record{Country: "US"}); err != nil {
		t.Fatalf("AddNet: %v", err)
	}
	w.Build()

	db := openDB(t, path)
	rec, err := db.Lookup("8.8.8.1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("Country = %q, want US", rec.Country)
	}
}

func TestAddNet_IPv6(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.ipdb")
	w, _ := iplib.NewWriter(path, testMeta)
	if err := w.AddNet("2001:db8::/32", &iplib.Record{Country: "US"}); err != nil {
		t.Fatalf("AddNet: %v", err)
	}
	w.Build()

	db := openDB(t, path)
	rec, err := db.Lookup("2001:db8::1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("Country = %q, want US", rec.Country)
	}
}

// ── Metadata ─────────────────────────────────────────────────────────────────

func TestMeta(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{Country: "AU"})
	})
	db := openDB(t, path)

	m := db.Meta()
	if m == nil {
		t.Fatal("Meta() returned nil")
	}
	if m.DatabaseType != testMeta.DatabaseType {
		t.Errorf("DatabaseType = %q, want %q", m.DatabaseType, testMeta.DatabaseType)
	}
	if !m.BuildTime.Equal(testMeta.BuildTime) {
		t.Errorf("BuildTime = %v, want %v", m.BuildTime, testMeta.BuildTime)
	}
}

// ── Error handling ───────────────────────────────────────────────────────────

func TestOpen_MissingFile(t *testing.T) {
	_, err := iplib.Open("/nonexistent/file.ipdb")
	if err == nil {
		t.Error("expected error opening nonexistent file")
	}
}

func TestOpen_BadMagic(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bad-*.ipdb")
	if err != nil {
		t.Fatal(err)
	}
	garbage := make([]byte, 64)
	copy(garbage, "XXXX") // wrong magic
	f.Write(garbage)
	f.Close()

	_, err = iplib.Open(f.Name())
	if err == nil {
		t.Error("expected error for bad magic")
	}
}

func TestLookup_InvalidIP(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{Country: "AU"})
	})
	db := openDB(t, path)

	_, err := db.Lookup("not-an-ip")
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestWriter_NilMeta(t *testing.T) {
	_, err := iplib.NewWriter("/tmp/x.ipdb", nil)
	if err == nil {
		t.Error("expected error for nil meta")
	}
}

func TestWriter_StartAfterEnd(t *testing.T) {
	w, _ := iplib.NewWriter(filepath.Join(t.TempDir(), "x.ipdb"), testMeta)
	err := w.Add("1.0.0.255", "1.0.0.0", &iplib.Record{Country: "AU"})
	if err == nil {
		t.Error("expected error when start > end")
	}
}

func TestWriter_MixedFamily(t *testing.T) {
	w, _ := iplib.NewWriter(filepath.Join(t.TempDir(), "x.ipdb"), testMeta)
	err := w.Add("1.0.0.0", "2001:db8::1", &iplib.Record{Country: "AU"})
	if err == nil {
		t.Error("expected error for mixed IP families")
	}
}

func TestWriter_BuildTwice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.ipdb")
	w, _ := iplib.NewWriter(path, testMeta)
	mustAdd(t, w, "1.0.0.0", "1.0.0.255", &iplib.Record{Country: "AU"})
	if err := w.Build(); err != nil {
		t.Fatal(err)
	}
	if err := w.Build(); err == nil {
		t.Error("expected error on second Build call")
	}
}

// ── IPv4-mapped IPv6 ─────────────────────────────────────────────────────────

func TestIPv4MappedIPv6(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		mustAdd(t, w, "8.8.8.0", "8.8.8.255", &iplib.Record{Country: "US"})
	})
	db := openDB(t, path)

	// ::ffff:8.8.8.8 is an IPv4-mapped IPv6 address; should resolve via IPv4 index.
	ip := net.ParseIP("::ffff:8.8.8.8")
	rec, err := db.LookupIP(ip)
	if err != nil {
		t.Fatalf("LookupIP(::ffff:8.8.8.8): %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("Country = %q, want US", rec.Country)
	}
}

// ── Empty database ───────────────────────────────────────────────────────────

func TestEmptyDatabase(t *testing.T) {
	path := newTmpDB(t, func(w *iplib.Writer) {
		// no entries
	})
	db := openDB(t, path)
	_, err := db.Lookup("1.2.3.4")
	if !errors.Is(err, iplib.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func mustAdd(t *testing.T, w *iplib.Writer, start, end string, rec *iplib.Record) {
	t.Helper()
	if err := w.Add(start, end, rec); err != nil {
		t.Fatalf("Add(%s, %s): %v", start, end, err)
	}
}
