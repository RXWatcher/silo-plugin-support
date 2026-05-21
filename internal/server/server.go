// Package server is the chi-mounted HTTP handler for the support
// plugin shell. It serves the customer + admin SPA shells and the
// minimal admin JSON API.
package server

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/go-hclog"

	pluginrt "github.com/ContinuumApp/continuum-plugin-support/internal/runtime"
)

type ConfigStore interface {
	GetConfig(ctx context.Context) (pluginrt.Config, error)
	UpdateConfig(ctx context.Context, cfg pluginrt.Config) error
}

type Deps struct {
	DatabaseURL string
	Logger      hclog.Logger
	ConfigStore ConfigStore
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Get("/", requireUser(hCustomerHome(d)))
	r.Get("/api/customer/bootstrap", requireUser(hCustomerBootstrap(d)))
	r.Get("/admin", requireAdmin(hAdminPage(d)))
	r.Get("/api/admin/config", requireAdmin(hGetConfig(d)))
	r.Patch("/api/admin/config", requireAdmin(hPatchConfig(d)))
	r.Get("/assets/*", hPublicAsset())

	return r
}
