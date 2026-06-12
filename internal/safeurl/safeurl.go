// Package safeurl provides a defensive guard against server-side
// request forgery (SSRF) for outbound HTTP fetches against
// admin-configured URLs. It blocks non-http(s) schemes and any URL
// whose host resolves to a loopback, private, link-local, or otherwise
// non-public IP address.
//
// These checks are best-effort: they close the obvious holes (fetching
// http://169.254.169.254/, http://localhost/, file://, etc.) without
// attempting to defeat a determined attacker who controls DNS and can
// win a TOCTOU race. For admin-only inputs that is an acceptable
// hardening layer rather than a complete sandbox.
package safeurl

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Validate parses raw and returns an error if it is not a safe outbound
// target. It enforces an http/https scheme and rejects any host that
// resolves only to non-public IP addresses.
func Validate(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (http/https only)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url has no host")
	}

	// If the host is a literal IP, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("host %q resolves to a non-public address", host)
		}
		return nil
	}

	// Otherwise resolve and ensure at least one address exists and all
	// resolved addresses are public.
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("host %q did not resolve", host)
	}
	for _, ip := range addrs {
		if !isPublicIP(ip) {
			return fmt.Errorf("host %q resolves to a non-public address", host)
		}
	}
	return nil
}

// isPublicIP reports whether ip is a globally routable unicast address
// (i.e. not loopback, private, link-local, multicast, or unspecified).
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return false
	}
	// IPv6 unique local addresses (fc00::/7) — IsPrivate covers these in
	// modern Go, but guard explicitly for clarity.
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil {
		if len(v6) == net.IPv6len && (v6[0]&0xfe) == 0xfc {
			return false
		}
	}
	return true
}
