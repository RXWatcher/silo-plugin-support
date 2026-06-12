package safeurl

import "testing"

func TestValidateRejectsUnsafeTargets(t *testing.T) {
	bad := []string{
		"",
		"file:///etc/passwd",
		"ftp://example.com/x",
		"http://localhost/",
		"http://127.0.0.1/",
		"http://127.0.0.1:8080/empty.php",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"https://0.0.0.0/",
		"http://",
	}
	for _, raw := range bad {
		if err := Validate(raw); err == nil {
			t.Errorf("Validate(%q) = nil, want error", raw)
		}
	}
}

func TestValidateAllowsPublicTargets(t *testing.T) {
	good := []string{
		"http://93.184.216.34/", // example.com literal (public)
		"https://8.8.8.8/",
	}
	for _, raw := range good {
		if err := Validate(raw); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", raw, err)
		}
	}
}
