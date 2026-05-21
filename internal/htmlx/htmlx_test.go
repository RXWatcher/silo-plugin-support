package htmlx

import (
	"strings"
	"testing"
)

func TestSanitizeStripsScriptAndOnHandlers(t *testing.T) {
	dirty := `<p>hello</p><script>alert(1)</script><a href="javascript:alert(2)" onclick="bad()">x</a>`
	clean := Sanitize(dirty)
	if strings.Contains(clean, "<script>") || strings.Contains(clean, "alert") || strings.Contains(clean, "onclick") || strings.Contains(clean, "javascript:") {
		t.Fatalf("sanitize failed; got %q", clean)
	}
	if !strings.Contains(clean, "<p>hello</p>") {
		t.Fatalf("sanitize stripped safe content; got %q", clean)
	}
}

func TestSanitizeAllowsImagesAndLinks(t *testing.T) {
	dirty := `<p>see <a href="/kb/x">x</a> and <img src="/api/kb/images/3" alt="diagram"></p>`
	clean := Sanitize(dirty)
	if !strings.Contains(clean, `href="/kb/x"`) {
		t.Fatalf("sanitize dropped link; got %q", clean)
	}
	if !strings.Contains(clean, `src="/api/kb/images/3"`) {
		t.Fatalf("sanitize dropped image; got %q", clean)
	}
}

func TestExtractTextStripsTagsAndCollapsesWhitespace(t *testing.T) {
	html := `<h1>Title</h1><p>Hello   <strong>world</strong>.</p><ul><li>one</li><li>two</li></ul>`
	got := ExtractText(html)
	want := "Title Hello world. one two"
	if got != want {
		t.Fatalf("ExtractText = %q, want %q", got, want)
	}
}

func TestExtractTextOnEmptyInput(t *testing.T) {
	if got := ExtractText(""); got != "" {
		t.Fatalf("ExtractText(\"\") = %q, want empty", got)
	}
}

func TestSanitizeRejectsDataURIInImgSrc(t *testing.T) {
	dirty := `<img src="data:text/html,<script>alert(1)</script>" alt="x">`
	clean := Sanitize(dirty)
	if strings.Contains(clean, "data:") {
		t.Fatalf("data: URI survived sanitization: %q", clean)
	}
}

func TestSanitizeKeepsHttpAndAbsolutePathImgSrc(t *testing.T) {
	// Make sure the fix didn't over-restrict — the legitimate paths
	// (/api/kb/images/N and https://...) must still pass.
	for _, src := range []string{`/api/kb/images/3`, `https://example.com/x.png`} {
		clean := Sanitize(`<img src="` + src + `" alt="x">`)
		if !strings.Contains(clean, `src="`+src+`"`) {
			t.Fatalf("safe src %q lost during sanitization: %q", src, clean)
		}
	}
}
