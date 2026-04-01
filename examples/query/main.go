package main

import (
	"fmt"
	"log"
	"net"

	"github.com/sendya/iplib"
)

func main() {
	db, err := iplib.Open("./bin/geoip.ipdb")
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
