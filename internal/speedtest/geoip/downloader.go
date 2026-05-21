package geoip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// downloadMMDB fetches the .mmdb (or .mmdb.gz) at url, validates by
// opening it with geoip2, and atomically renames into dest. Falls
// back to the previous UTC month's URL once if the primary 404s
// (db-ip.com publishes around the 1st of the month).
func downloadMMDB(ctx context.Context, urlPattern, dest string) error {
	now := time.Now().UTC()
	primary := strings.ReplaceAll(urlPattern, "{YYYY-MM}", now.Format("2006-01"))
	prev := strings.ReplaceAll(urlPattern, "{YYYY-MM}", now.AddDate(0, -1, 0).Format("2006-01"))

	if err := tryDownload(ctx, primary, dest); err == nil {
		return nil
	} else {
		// Fall back once to last month.
		if err2 := tryDownload(ctx, prev, dest); err2 == nil {
			return nil
		} else {
			return fmt.Errorf("download mmdb: primary %v; fallback %v", err, err2)
		}
	}
}

func tryDownload(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	var reader io.Reader = io.LimitReader(resp.Body, 50<<20) // 50 MB cap
	if strings.HasSuffix(url, ".gz") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			out.Close()
			os.Remove(tmp)
			return fmt.Errorf("gunzip: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	if _, err := io.Copy(out, reader); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()

	// Validate by opening.
	r, err := geoip2.Open(tmp)
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("validate: %w", err)
	}
	r.Close()

	return os.Rename(tmp, dest)
}
