package kb

import (
	"regexp"
	"strconv"
)

var imgRefRe = regexp.MustCompile(`/api/kb/images/(\d+)`)

// ExtractImageIDs scans body_html for `/api/kb/images/{id}` references
// and returns the distinct ids in first-seen order. Used by the
// article-save handler to adopt orphan images.
func ExtractImageIDs(htmlSrc string) []int64 {
	matches := imgRefRe.FindAllStringSubmatch(htmlSrc, -1)
	if matches == nil {
		return nil
	}
	seen := map[int64]bool{}
	out := []int64{}
	for _, m := range matches {
		id, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
