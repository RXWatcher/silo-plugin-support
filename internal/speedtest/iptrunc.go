// Package speedtest holds the speedtest module's non-store helpers:
// IP truncation, the auto-strategy resolver, and the GeoIP chain
// glue. The geoip subpackage holds source-kind implementations.
package speedtest

import "net/netip"

// TruncateIP reduces a client IP per the operator's privacy setting.
// "off" returns empty (caller persists NULL); anything else (default
// "truncated") returns the /24 (IPv4) or /48 (IPv6) CIDR string.
// Invalid input returns empty.
func TruncateIP(ip, storage string) string {
	if storage == "off" {
		return ""
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	var prefix netip.Prefix
	if addr.Is4() {
		prefix = netip.PrefixFrom(addr, 24).Masked()
	} else {
		prefix = netip.PrefixFrom(addr, 48).Masked()
	}
	return prefix.String()
}
