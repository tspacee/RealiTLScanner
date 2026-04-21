package main

import (
	"github.com/oschwald/geoip2-golang"
	"log/slog"
	"net"
	"sync"
)

type Geo struct {
	geoReader *geoip2.Reader
	mu        sync.Mutex
}

func NewGeo() *Geo {
	geo := &Geo{
		mu: sync.Mutex{},
	}
	// Try GeoLite2-Country.mmdb as a fallback in addition to Country.mmdb
	for _, dbPath := range []string{"Country.mmdb", "GeoLite2-Country.mmdb"} {
		reader, err := geoip2.Open(dbPath)
		if err != nil {
			continue
		}
		slog.Info("Enabled GeoIP", "db", dbPath)
		geo.geoReader = reader
		return geo
	}
	slog.Warn("Cannot open Country.mmdb or GeoLite2-Country.mmdb, GeoIP lookup will be disabled")
	return geo
}

func (o *Geo) GetGeo(ip net.IP) string {
	if o.geoReader == nil {
		return "N/A"
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	country, err := o.geoReader.Country(ip)
	if err != nil {
		slog.Debug("Error reading geo", "err", err)
		return "N/A"
	}
	// Return "N/A" if IsoCode is empty (e.g. reserved/private ranges in the DB)
	if country.Country.IsoCode == "" {
		return "N/A"
	}
	return country.Country.IsoCode
}
