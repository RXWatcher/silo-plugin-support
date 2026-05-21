package kb

import "testing"

func TestSlugifyHandlesCommonCases(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"Why is my video buffering?", "why-is-my-video-buffering"},
		{"  spaces   ", "spaces"},
		{"4K Streaming!!!", "4k-streaming"},
		{"über-cool", "uber-cool"},
		{"", ""},
		{"---", ""},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueSlugAppendsSuffixOnCollision(t *testing.T) {
	exists := map[string]bool{"hello": true, "hello-2": true}
	check := func(s string) (bool, error) { return exists[s], nil }
	got, err := UniqueSlug("hello", check)
	if err != nil {
		t.Fatalf("UniqueSlug: %v", err)
	}
	if got != "hello-3" {
		t.Fatalf("UniqueSlug = %q, want hello-3", got)
	}
}

func TestUniqueSlugReturnsBaseOnNoCollision(t *testing.T) {
	check := func(string) (bool, error) { return false, nil }
	got, err := UniqueSlug("fresh", check)
	if err != nil || got != "fresh" {
		t.Fatalf("UniqueSlug = %q, %v; want (fresh, nil)", got, err)
	}
}
