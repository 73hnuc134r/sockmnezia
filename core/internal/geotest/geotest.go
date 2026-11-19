// Package geotest writes tiny synthetic geoip.dat / geosite.dat files for tests.
package geotest

import (
	"net/netip"
	"os"
	"path/filepath"

	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

// Write creates both files in dir:
//
//	geoip:ru            = 198.51.100.0/24
//	geoip:test          = 203.0.113.0/24
//	geosite:category-ru = domain ru.example, full exact.ru.example
//	geosite:category-ads-all = domain ads.example
func Write(dir string) error {
	cidr := func(s string) *geodata.CIDR {
		p := netip.MustParsePrefix(s)
		return &geodata.CIDR{Ip: p.Addr().AsSlice(), Prefix: uint32(p.Bits())}
	}
	ips := &geodata.GeoIPList{Entry: []*geodata.GeoIP{
		{Code: "RU", Cidr: []*geodata.CIDR{cidr("198.51.100.0/24")}},
		{Code: "TEST", Cidr: []*geodata.CIDR{cidr("203.0.113.0/24")}},
	}}
	sites := &geodata.GeoSiteList{Entry: []*geodata.GeoSite{
		{Code: "CATEGORY-RU", Domain: []*geodata.Domain{
			{Type: geodata.Domain_Domain, Value: "ru.example"},
			{Type: geodata.Domain_Full, Value: "exact.ru.example"},
		}},
		{Code: "CATEGORY-ADS-ALL", Domain: []*geodata.Domain{{Type: geodata.Domain_Domain, Value: "ads.example"}}},
	}}
	for name, m := range map[string]proto.Message{"geoip.dat": ips, "geosite.dat": sites} {
		b, err := proto.Marshal(m)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			return err
		}
	}
	return nil
}
