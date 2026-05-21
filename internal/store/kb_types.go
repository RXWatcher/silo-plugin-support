package store

import "time"

// KBCategory mirrors a row in kb_categories.
type KBCategory struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// KBTag mirrors a row in kb_tags.
type KBTag struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

// KBArticle mirrors a row in kb_articles. Body is the sanitised
// HTML (source of truth); BodyText is the derived plaintext used by
// the FTS index — kept out of the JSON shape.
type KBArticle struct {
	ID            int64       `json:"id"`
	Slug          string      `json:"slug"`
	Title         string      `json:"title"`
	Summary       string      `json:"summary"`
	BodyHTML      string      `json:"bodyHtml"`
	BodyText      string      `json:"-"`
	CategoryID    int64       `json:"categoryId"`
	Status        string      `json:"status"`
	PublishAt     *time.Time  `json:"publishAt,omitempty"`
	PublishedAt   *time.Time  `json:"publishedAt,omitempty"`
	LastEditedBy  string      `json:"lastEditedBy"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	Tags          []KBTag     `json:"tags"`
	Category      *KBCategory `json:"category,omitempty"`
}

// KBArticleSummary is the lightweight projection returned by list
// queries (no body, no full category).
type KBArticleSummary struct {
	ID           int64      `json:"id"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Summary      string     `json:"summary"`
	CategoryID   int64      `json:"categoryId"`
	CategoryName string     `json:"categoryName"`
	Status       string     `json:"status"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	Tags         []string   `json:"tags"`
}

// KBImage mirrors a row in kb_images.
type KBImage struct {
	ID        int64
	ArticleID *int64
	Filename  string
	MIME      string
	Bytes     int64
	Content   []byte
	SHA256    []byte
	CreatedAt time.Time
}

// KBVoteAggregate is the per-article rollup the admin engagement
// view consumes.
type KBVoteAggregate struct {
	HelpfulCount    int `json:"helpfulCount"`
	NotHelpfulCount int `json:"notHelpfulCount"`
}

// KBViewAggregate is the per-article rollup over the last 30 days.
type KBViewAggregate struct {
	TotalViews    int `json:"totalViews"`
	UniqueViewers int `json:"uniqueViewers"`
}
