package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
)

type fakeConfigStore struct {
	mu  sync.Mutex
	cfg pluginrt.Config
}

func (s *fakeConfigStore) GetConfig(context.Context) (pluginrt.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, nil
}

func (s *fakeConfigStore) UpdateConfig(_ context.Context, cfg pluginrt.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	return nil
}

func newTestDeps() (Deps, *fakeConfigStore) {
	cs := &fakeConfigStore{}
	return Deps{ConfigStore: cs}, cs
}

func adminHeaders(r *http.Request) {
	r.Header.Set("X-Silo-User-Id", "1")
	r.Header.Set("X-Silo-User-Role", "admin")
}

func userHeaders(r *http.Request) {
	r.Header.Set("X-Silo-User-Id", "42")
}

func TestCustomerHomeRequiresUserIdentity(t *testing.T) {
	h := New(Deps{ConfigStore: &fakeConfigStore{}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminPageRequiresAdminRole(t *testing.T) {
	h := New(Deps{ConfigStore: &fakeConfigStore{}})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	userHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestGetConfigReturnsToggles(t *testing.T) {
	d, cs := newTestDeps()
	cs.cfg = pluginrt.Config{Modules: pluginrt.ModuleToggles{KB: true}}
	h := New(d)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	adminHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body pluginrt.Config
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Modules.KB {
		t.Fatalf("KB toggle = false, want true; body=%s", rec.Body.String())
	}
}

func TestPatchConfigUpdatesToggles(t *testing.T) {
	d, cs := newTestDeps()
	h := New(d)

	body := `{"modules":{"kb":true,"speedtest":true,"tickets":false,"ai":false}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/config", bytes.NewBufferString(body))
	adminHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !cs.cfg.Modules.KB || !cs.cfg.Modules.Speedtest {
		t.Fatalf("toggles not persisted: %+v", cs.cfg.Modules)
	}
}

func TestCustomerBootstrapReturnsModulesAndIdentity(t *testing.T) {
	d, cs := newTestDeps()
	cs.cfg = pluginrt.Config{Modules: pluginrt.ModuleToggles{Speedtest: true}}
	h := New(d)

	req := httptest.NewRequest(http.MethodGet, "/api/customer/bootstrap", nil)
	userHeaders(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Modules pluginrt.ModuleToggles `json:"modules"`
		UserID  string                 `json:"userId"`
		IsAdmin bool                   `json:"isAdmin"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Modules.Speedtest {
		t.Fatalf("Speedtest module should be exposed; got %+v", body.Modules)
	}
	if body.UserID != "42" {
		t.Fatalf("UserID = %q, want 42", body.UserID)
	}
	if body.IsAdmin {
		t.Fatal("user request should NOT be flagged isAdmin")
	}
}
