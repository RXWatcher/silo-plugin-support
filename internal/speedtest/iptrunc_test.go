package speedtest

import "testing"

func TestTruncateIPv4To24(t *testing.T) {
	got := TruncateIP("192.0.2.123", "truncated")
	if got != "192.0.2.0/24" {
		t.Fatalf("TruncateIP = %q, want 192.0.2.0/24", got)
	}
}

func TestTruncateIPv6To48(t *testing.T) {
	got := TruncateIP("2001:db8:1234:5678::abcd", "truncated")
	if got != "2001:db8:1234::/48" {
		t.Fatalf("TruncateIP = %q, want 2001:db8:1234::/48", got)
	}
}

func TestTruncateOffReturnsEmpty(t *testing.T) {
	if got := TruncateIP("192.0.2.123", "off"); got != "" {
		t.Fatalf("TruncateIP(off) = %q, want empty", got)
	}
}

func TestTruncateUnknownStorageDefaultsToTruncated(t *testing.T) {
	if got := TruncateIP("192.0.2.123", "weird"); got != "192.0.2.0/24" {
		t.Fatalf("TruncateIP(weird) = %q, want truncated default", got)
	}
}

func TestTruncateInvalidIPReturnsEmpty(t *testing.T) {
	if got := TruncateIP("not-an-ip", "truncated"); got != "" {
		t.Fatalf("TruncateIP(bad) = %q, want empty", got)
	}
}
