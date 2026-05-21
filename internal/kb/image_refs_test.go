package kb

import (
	"reflect"
	"testing"
)

func TestExtractImageIDsFindsAllReferences(t *testing.T) {
	html := `<p>see <img src="/api/kb/images/42" alt="x"></p>
	         <img src="/api/kb/images/7">
	         <a href="https://example.com">no match</a>
	         <img src="https://example.com/cat.png">`
	got := ExtractImageIDs(html)
	want := []int64{42, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractImageIDs = %v, want %v", got, want)
	}
}

func TestExtractImageIDsDedupes(t *testing.T) {
	html := `<img src="/api/kb/images/3"><img src="/api/kb/images/3">`
	got := ExtractImageIDs(html)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("ExtractImageIDs = %v, want [3]", got)
	}
}
