package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/RXWatcher/silo-plugin-support/internal/store"
)

// uniqueCustomer returns a customer id unlikely to collide with other
// tests sharing the same database (these are integration tests against a
// live PG and the schema is not reset between runs).
func uniqueCustomer(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, randSeed())
}

var seed int64

func randSeed() int64 { seed++; return seed }

func createTicket(t *testing.T, h http.Handler, catID int64, customerID, body string) (int, store.TKTicket) {
	t.Helper()
	reqBody := fmt.Sprintf(`{"categoryId":%d,"subject":"s","body":%q,"customerEmail":"a@b.com"}`, catID, body)
	req := httptest.NewRequest(http.MethodPost, "/api/customer/tickets", bytes.NewBufferString(reqBody))
	req.Header.Set("X-Silo-User-Id", customerID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var tk store.TKTicket
	_ = json.Unmarshal(rec.Body.Bytes(), &tk)
	return rec.Code, tk
}

func TestTKCreateRejectsShortBody(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	d.TKMinBodyChars = 10
	h := New(d)
	cat, _ := st.TKCreateCategory(context.Background(),
		store.TKCategory{Slug: uniqueCustomer("short"), Name: "S", Active: true})

	code, _ := createTicket(t, h, cat.ID, uniqueCustomer("c"), "tiny")
	if code != http.StatusBadRequest {
		t.Fatalf("short body status = %d, want 400", code)
	}
}

func TestTKCreateRejectsOverlongBody(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	d.TKMaxBodyChars = 50
	h := New(d)
	cat, _ := st.TKCreateCategory(context.Background(),
		store.TKCategory{Slug: uniqueCustomer("long"), Name: "L", Active: true})

	code, _ := createTicket(t, h, cat.ID, uniqueCustomer("c"), strings.Repeat("x", 100))
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("overlong body status = %d, want 413", code)
	}
}

func TestTKOpenTicketCapRejectsNewTicket(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	d.TKMaxOpenPerCustomer = 2
	// Disable the per-action rate limit interference by leaving it; the
	// rate limit fires on the *second* request, so seed tickets directly.
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx,
		store.TKCategory{Slug: uniqueCustomer("cap"), Name: "C", Active: true})
	customer := uniqueCustomer("capcust")

	// Seed two open tickets directly via the store (bypasses rate limit).
	for i := 0; i < 2; i++ {
		tn, _ := st.TKNextTrackingNumber(ctx)
		tx, _ := st.TKBegin(ctx)
		saved, err := st.TKCreateTicket(ctx, tx, store.TKTicket{
			TrackingNumber: tn, CustomerID: customer, CustomerEmail: "a@b.com",
			CategoryID: cat.ID, Subject: "seed",
		})
		if err != nil {
			tx.Rollback(ctx)
			t.Fatal(err)
		}
		_, _ = st.TKInsertEntry(ctx, tx, store.TKEntry{
			TicketID: saved.ID, Kind: "initial", AuthorID: customer, AuthorRole: "customer", Body: "seed body here",
		})
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	count, err := st.TKCountOpenTicketsForCustomer(ctx, customer)
	if err != nil || count != 2 {
		t.Fatalf("open count = %d (err %v), want 2", count, err)
	}

	code, _ := createTicket(t, h, cat.ID, customer, "a valid body長い enough")
	if code != http.StatusTooManyRequests {
		t.Fatalf("over-cap create status = %d, want 429", code)
	}
}

func TestTKAdminActionsWriteAuditTrail(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx,
		store.TKCategory{Slug: uniqueCustomer("audit"), Name: "A", Active: true})

	customer := uniqueCustomer("auditcust")
	_, created := createTicket(t, h, cat.ID, customer, "a sufficiently long body")
	tn := created.TrackingNumber
	if tn == "" {
		t.Fatal("ticket not created")
	}

	adminReq := func(path, body string) {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("X-Silo-User-Id", "admin-7")
		req.Header.Set("X-Silo-User-Role", "admin")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", path, rec.Code, rec.Body.String())
		}
	}
	adminReq("/api/admin/tickets/"+tn+"/reply", `{"body":"admin reply here"}`)
	adminReq("/api/admin/tickets/"+tn+"/assign", `{"adminId":"admin-7"}`)
	adminReq("/api/admin/tickets/"+tn+"/status", `{"to":"resolved"}`)

	audit, err := st.TKListAudit(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, a := range audit {
		got[a.Action] = true
		if a.ActorID != "admin-7" {
			t.Errorf("audit actor = %q, want admin-7", a.ActorID)
		}
	}
	for _, want := range []string{"reply", "assign", "status_change"} {
		if !got[want] {
			t.Errorf("missing audit action %q (have %v)", want, got)
		}
	}
}

func TestTKAttachmentPerTicketCountCap(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	d.TKMaxAttachmentsPerTicket = 1
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx,
		store.TKCategory{Slug: uniqueCustomer("attcap"), Name: "AC", Active: true})

	customer := uniqueCustomer("attcust")
	_, created := createTicket(t, h, cat.ID, customer, "a sufficiently long body")
	entries, _ := st.TKListEntries(ctx, created.ID, false)
	if len(entries) == 0 {
		t.Fatal("no initial entry")
	}
	entryID := entries[0].ID

	upload := func() int {
		body, ct := textAttachmentBody(t)
		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/tickets/entries/%d/attachments", entryID), body)
		req.Header.Set("X-Silo-User-Id", customer)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := upload(); code != http.StatusOK {
		t.Fatalf("first upload status = %d, want 200", code)
	}
	if code := upload(); code != http.StatusConflict {
		t.Fatalf("second upload status = %d, want 409 (cap)", code)
	}
}

func TestTKAttachmentStorageQuota(t *testing.T) {
	d, st, cleanup := tkTestDeps(t)
	defer cleanup()
	d.TKMaxStorageBytesPerCustomer = 4 // bytes; "hello" is 5
	d.TKMaxAttachmentsPerTicket = 100
	h := New(d)
	ctx := context.Background()
	cat, _ := st.TKCreateCategory(ctx,
		store.TKCategory{Slug: uniqueCustomer("quota"), Name: "Q", Active: true})

	customer := uniqueCustomer("quotacust")
	_, created := createTicket(t, h, cat.ID, customer, "a sufficiently long body")
	entries, _ := st.TKListEntries(ctx, created.ID, false)
	entryID := entries[0].ID

	body, ct := textAttachmentBody(t)
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/tickets/entries/%d/attachments", entryID), body)
	req.Header.Set("X-Silo-User-Id", customer)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("over-quota upload status = %d, want 507", rec.Code)
	}
}

// textAttachmentBody builds a multipart body with a single 5-byte
// text/plain file field. The explicit Content-Type is required because
// the upload allowlist rejects the multipart writer's default
// application/octet-stream.
func textAttachmentBody(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="a.txt"`)
	h.Set("Content-Type", "text/plain")
	fw, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("hello"))
	_ = mw.Close()
	return body, mw.FormDataContentType()
}
