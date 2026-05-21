package geoip

import (
	"fmt"

	"github.com/RXWatcher/continuum-plugin-support/internal/store"
)

// BuildSource constructs a concrete Source from a store row.
// Returns an error for unknown kinds or invalid config JSON.
// `cacheDir` is used only by mmdb_auto.
func BuildSource(row store.STGeoIPSource, cacheDir string) (Source, error) {
	switch row.Kind {
	case "mmdb_auto":
		return NewMMDBAutoSource(row.ID, row.Config, cacheDir)
	case "mmdb_file":
		return NewMMDBFileSource(row.ID, row.Config)
	case "http_api":
		return NewHTTPAPISource(row.ID, row.Config, nil)
	case "request_header":
		return NewRequestHeaderSource(row.ID, row.Config)
	default:
		return nil, fmt.Errorf("unknown geoip source kind: %q", row.Kind)
	}
}
