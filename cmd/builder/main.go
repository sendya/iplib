package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sendya/iplib"
)

var (
	flagSourcePath string = filepath.Dir("./data/")
)

func main() {
	meta := &iplib.Meta{
		DatabaseType: "GeoCity",
		Description:  "Example GeoIP database",
		BuildTime:    time.Now(),
	}

	w, err := iplib.NewWriter(filepath.Join("./bin/geoip.ipdb"), meta)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	// load ipv4
	f, err := os.OpenFile(filepath.Join(flagSourcePath, "ipv4_source.txt"), os.O_RDONLY, 0o666)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	loadSegments(w, scanner)

	_ = f.Close()

	// load ipv6

	f, err = os.OpenFile(filepath.Join(flagSourcePath, "ipv6_source.txt"), os.O_RDONLY, 0o666)
	if err != nil {
		panic(err)
	}

	scanner = bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	loadSegments(w, scanner)

	if err := w.Build(); err != nil {
		log.Fatal(err)
	}
}

func loadSegments(w *iplib.Writer, scanner *bufio.Scanner) {

	for scanner.Scan() {

		fields := strings.Split(strings.TrimSpace(scanner.Text()), "|")
		fmt.Printf("fields: %#+v\n", fields)

		record := &iplib.Record{
			Country:     fields[6],
			CountryName: fields[2],
			Region:      fields[3],
			City:        fields[4],
			ISP:         fields[5],
		}

		_ = w.Add(fields[0], fields[1], record)
	}
}
