// Package kb holds KB-module helpers that aren't store or server
// layers — slug derivation, cron loop, event payload assembly.
package kb

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Slugify converts a title to a URL-safe lowercase-kebab slug:
//   - Unicode-normalises to NFKD and drops combining marks (so "über"
//     becomes "uber").
//   - Lowercases.
//   - Replaces any run of non-[a-z0-9] with a single "-".
//   - Trims leading/trailing "-".
//
// Empty input yields empty output (caller decides what to do).
func Slugify(s string) string {
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalised, _, err := transform.String(t, s)
	if err != nil {
		normalised = s
	}
	lower := strings.ToLower(normalised)
	var b strings.Builder
	prevDash := true
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// UniqueSlug returns a slug derived from base that doesn't collide
// according to exists. On collision it appends "-2", "-3", … up to
// 100 attempts. The caller passes a closure that consults the right
// table.
func UniqueSlug(base string, exists func(slug string) (bool, error)) (string, error) {
	if base == "" {
		return "", fmt.Errorf("slug: empty base")
	}
	taken, err := exists(base)
	if err != nil {
		return "", err
	}
	if !taken {
		return base, nil
	}
	for n := 2; n < 100; n++ {
		cand := fmt.Sprintf("%s-%d", base, n)
		taken, err := exists(cand)
		if err != nil {
			return "", err
		}
		if !taken {
			return cand, nil
		}
	}
	return "", fmt.Errorf("slug: too many collisions on %q", base)
}
