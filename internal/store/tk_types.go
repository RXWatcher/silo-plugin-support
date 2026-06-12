package store

import "time"

// Category taxonomy --------------------------------------------------

type TKCategory struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TKSubcategory struct {
	ID         int64     `json:"id"`
	CategoryID int64     `json:"categoryId"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	SortOrder  int       `json:"sortOrder"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type TKCategoryField struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"categoryId"`
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required"`
	SortOrder  int    `json:"sortOrder"`
}

// Ticket -------------------------------------------------------------

type TKTicket struct {
	ID              int64      `json:"id"`
	TrackingNumber  string     `json:"trackingNumber"`
	CustomerID      string     `json:"customerId"`
	CustomerEmail   string     `json:"customerEmail"`
	CategoryID      int64      `json:"categoryId"`
	SubcategoryID   *int64     `json:"subcategoryId,omitempty"`
	Subject         string     `json:"subject"`
	Status          string     `json:"status"`
	AssignedAdminID *string    `json:"assignedAdminId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	WaitingSince    *time.Time `json:"waitingSince,omitempty"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	Category        *TKCategory    `json:"category,omitempty"`
	Subcategory     *TKSubcategory `json:"subcategory,omitempty"`
	FieldValues     []TKFieldValue `json:"fieldValues,omitempty"`
}

type TKEntry struct {
	ID          int64              `json:"id"`
	TicketID    int64              `json:"ticketId"`
	Kind        string             `json:"kind"`
	AuthorID    string             `json:"authorId"`
	AuthorRole  string             `json:"authorRole"`
	Body        string             `json:"body"`
	CreatedAt   time.Time          `json:"createdAt"`
	Attachments []TKAttachmentMeta `json:"attachments,omitempty"`
}

type TKFieldValue struct {
	FieldID    int64  `json:"fieldId"`
	FieldKey   string `json:"fieldKey"`
	FieldLabel string `json:"fieldLabel"`
	Value      string `json:"value"`
}

type TKAttachmentMeta struct {
	ID        int64     `json:"id"`
	Filename  string    `json:"filename"`
	MIME      string    `json:"mime"`
	Bytes     int64     `json:"bytes"`
	CreatedAt time.Time `json:"createdAt"`
}

type TKAttachment struct {
	ID        int64
	EntryID   int64
	Filename  string
	MIME      string
	Bytes     int64
	Content   []byte
	SHA256    []byte
	CreatedAt time.Time
}

// TKAuditEntry is an append-only record of an admin action that touches
// customer PII (replies, notes, status changes, assignments). Detail
// carries action-specific fields (e.g. from/to status) as JSON.
type TKAuditEntry struct {
	ID        int64          `json:"id"`
	TicketID  int64          `json:"ticketId"`
	ActorID   string         `json:"actorId"`
	ActorRole string         `json:"actorRole"`
	Action    string         `json:"action"`
	Detail    map[string]any `json:"detail,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type TKTicketListFilter struct {
	CustomerID    string
	Status        string
	StatusGroup   string
	CategoryID    int64
	AssigneeID    string
	CallerAdminID string
	Search        string
	Limit         int
	Offset        int
}
