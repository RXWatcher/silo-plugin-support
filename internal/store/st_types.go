package store

import (
	"encoding/json"
	"time"
)

// STEndpoint mirrors a row in st_endpoints.
type STEndpoint struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	URL       string    `json:"url"`
	Country   string    `json:"country"`
	Region    string    `json:"region"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// STGeoIPSource mirrors a row in st_geoip_sources. Config is kept as
// raw JSON so the source-kind packages can unmarshal into their own
// typed config struct.
type STGeoIPSource struct {
	ID              int64           `json:"id"`
	Label           string          `json:"label"`
	Kind            string          `json:"kind"`
	Config          json.RawMessage `json:"config"`
	SortOrder       int             `json:"sortOrder"`
	Active          bool            `json:"active"`
	LastStatus      string          `json:"lastStatus"`
	LastUsedAt      *time.Time      `json:"lastUsedAt,omitempty"`
	LastRefreshedAt *time.Time      `json:"lastRefreshedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// STResult mirrors a row in st_results. ClientIP is rendered as the
// truncated CIDR string the truncation logic wrote.
type STResult struct {
	ID            int64     `json:"id"`
	CustomerID    string    `json:"customerId"`
	EndpointID    *int64    `json:"endpointId,omitempty"`
	EndpointLabel string    `json:"endpointLabel"`
	AutoStrategy  string    `json:"autoStrategy"`
	DownloadMbps  float64   `json:"downloadMbps"`
	UploadMbps    float64   `json:"uploadMbps"`
	PingMs        float64   `json:"pingMs"`
	JitterMs      float64   `json:"jitterMs"`
	ClientIP      string    `json:"clientIp,omitempty"`
	UserAgent     string    `json:"userAgent,omitempty"`
	RanAt         time.Time `json:"ranAt"`
}

// STResultFilter narrows what STListResults returns. Zero values are
// wildcards.
type STResultFilter struct {
	CustomerID   string
	EndpointID   int64
	AutoStrategy string
	SlowOnly     bool
	SlowThresh   float64
	Since        time.Time
	Limit        int
	Offset       int
}

// STDashboardAggregates is the per-endpoint rollup the admin
// dashboard page renders. Counts are over the last 30 days.
type STDashboardAggregates struct {
	PerEndpoint []STEndpointAggregate `json:"perEndpoint"`
	PerDay      []STDailyCount        `json:"perDay"`
	SlowTop10   []STResult            `json:"slowTop10"`
	CountryHits []STCountryCount      `json:"countryHits"`
}

type STEndpointAggregate struct {
	EndpointID     *int64  `json:"endpointId,omitempty"`
	Label          string  `json:"label"`
	MedianDownload float64 `json:"medianDownload"`
	MedianUpload   float64 `json:"medianUpload"`
	MedianPing     float64 `json:"medianPing"`
	ResultCount    int     `json:"resultCount"`
}

type STDailyCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type STCountryCount struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}
