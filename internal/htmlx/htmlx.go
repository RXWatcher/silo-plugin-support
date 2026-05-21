// Package htmlx wraps the HTML sanitisation policy used by the KB
// module (and any future module that stores operator-authored HTML).
// One package so the policy lives in exactly one place and changes
// land for every caller atomically.
package htmlx

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// UGCPolicy() already calls AllowImages() which permits img with
	// `src` constrained to http/https schemes and `alt` allowed.
	// Only declare the extra non-URL attrs we additionally want;
	// re-declaring `src` here would APPEND a no-scheme-check policy
	// (bluemonday's `OnElements` is additive), opening `data:` URIs.
	p.AllowAttrs("title", "width", "height").OnElements("img")
	// Tiptap may produce these classes for code blocks / inline code.
	p.AllowAttrs("class").OnElements("code", "pre")
	return p
}()

// Sanitize returns dirty with disallowed tags / attrs / URL schemes
// removed. Safe to call on any string (including non-HTML).
func Sanitize(dirty string) string {
	return policy.Sanitize(dirty)
}

// ExtractText reduces HTML to whitespace-collapsed plaintext for the
// FTS body_text column. Tags drop out, text nodes survive, runs of
// whitespace become a single space. Empty input yields empty.
func ExtractText(htmlSrc string) string {
	if htmlSrc == "" {
		return ""
	}
	node, err := html.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return ""
	}
	var parts []string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				parts = append(parts, text)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)

	// Join parts, but don't add space before punctuation.
	var result strings.Builder
	for i, part := range parts {
		if i > 0 && !strings.HasPrefix(part, ".") && !strings.HasPrefix(part, ",") && !strings.HasPrefix(part, "!") && !strings.HasPrefix(part, "?") && !strings.HasPrefix(part, ";") && !strings.HasPrefix(part, ":") {
			result.WriteByte(' ')
		}
		result.WriteString(part)
	}
	return result.String()
}
