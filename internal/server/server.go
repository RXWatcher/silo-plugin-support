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

	pluginrt "github.com/RXWatcher/silo-plugin-support/internal/runtime"
	"github.com/RXWatcher/silo-plugin-support/internal/speedtest"
)

type ConfigStore interface {
	GetConfig(ctx context.Context) (pluginrt.Config, error)
	UpdateConfig(ctx context.Context, cfg pluginrt.Config) error
}

// EventPublisher publishes plugin lifecycle events to the host bus.
// Tests pass nil (no-op); production wires the SDK's host client.
type EventPublisher interface {
	PublishEvent(ctx context.Context, name string, payload map[string]any) error
}

type Deps struct {
	DatabaseURL    string
	Logger         hclog.Logger
	ConfigStore    ConfigStore
	EventPublisher EventPublisher

	// Speedtest module wiring; resolver-nil → 503 from the handler.
	STAutoResolver      *speedtest.Resolver
	STClientIPStorage   string  // "truncated" (default) | "off"
	STSlowThresholdMbps float64
	STGeoIPCacheDir     string  // on-disk cache dir for mmdb_auto downloads

	// Tickets module config.
	TKAutoCloseEnabled       bool
	TKResolvedCloseAfterDays int
	TKWaitingCloseAfterDays  int

	// Tickets spam / abuse + quota controls. Resolved from config with
	// in-code defaults applied; see runtime.NormalizeAppConfig. A
	// non-positive value disables the corresponding limit.
	TKMaxOpenPerCustomer         int
	TKMinBodyChars               int
	TKMaxBodyChars               int
	TKMaxAttachmentsPerTicket    int
	TKMaxStorageBytesPerCustomer int64
}

// tkLimits returns the effective spam/quota limits for this Deps,
// substituting in-code defaults for any value left at zero (e.g. tests
// that build Deps directly without going through config normalization).
func (d Deps) tkLimits() (maxOpen, minBody, maxBody, maxAtt int, maxBytes int64) {
	maxOpen, minBody, maxBody, maxAtt, maxBytes =
		d.TKMaxOpenPerCustomer, d.TKMinBodyChars, d.TKMaxBodyChars,
		d.TKMaxAttachmentsPerTicket, d.TKMaxStorageBytesPerCustomer
	if maxOpen == 0 {
		maxOpen = 10
	}
	if minBody == 0 {
		minBody = 10
	}
	if maxBody == 0 {
		maxBody = 20000
	}
	if maxAtt == 0 {
		maxAtt = 20
	}
	if maxBytes == 0 {
		maxBytes = 50 << 20
	}
	return
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(limitBody(12 << 20))

	r.Get("/", requireUser(hCustomerHome(d)))
	r.Get("/api/customer/bootstrap", requireUser(hCustomerBootstrap(d)))
	r.Get("/admin", requireAdmin(hAdminPage(d)))
	r.Get("/api/admin/config", requireAdmin(hGetConfig(d)))
	r.Patch("/api/admin/config", requireAdmin(hPatchConfig(d)))
	r.Get("/assets/*", hPublicAsset())

	// KB module routes.
	r.Get("/kb",                              requireUser(hKBBrowsePage(d)))
	r.Get("/kb/{slug}",                       requireUser(hKBDetailPage(d)))
	r.Get("/api/customer/kb/articles",        requireUser(hKBCustomerList(d)))
	r.Get("/api/customer/kb/articles/{slug}", requireUser(hKBCustomerDetail(d)))
	r.Get("/api/customer/kb/related/{slug}",  requireUser(hKBCustomerRelated(d)))
	r.Get("/api/customer/kb/search",          requireUser(hKBCustomerSearch(d)))
	r.Post("/api/customer/kb/articles/{slug}/vote", requireUser(hKBCustomerVote(d)))
	r.Get("/api/kb/images/{id}",              requireUser(hKBImageServe(d)))

	r.Get("/admin/kb",                requireAdmin(hKBAdminListPage(d)))
	r.Get("/admin/kb/new",            requireAdmin(hKBAdminEditPage(d)))
	r.Get("/admin/kb/{id}",           requireAdmin(hKBAdminEditPage(d)))
	r.Get("/admin/kb/categories",     requireAdmin(hKBAdminCategoriesPage(d)))
	r.Get("/admin/kb/tags",           requireAdmin(hKBAdminTagsPage(d)))

	r.Get("/api/admin/kb/articles",                  requireAdmin(hKBAdminListArticles(d)))
	r.Post("/api/admin/kb/articles",                 requireAdmin(hKBAdminCreateArticle(d)))
	r.Get("/api/admin/kb/articles/{id}",             requireAdmin(hKBAdminGetArticle(d)))
	r.Put("/api/admin/kb/articles/{id}",             requireAdmin(hKBAdminUpdateArticle(d)))
	r.Delete("/api/admin/kb/articles/{id}",          requireAdmin(hKBAdminDeleteArticle(d)))
	r.Post("/api/admin/kb/articles/{id}/publish",    requireAdmin(hKBAdminPublishArticle(d)))
	r.Post("/api/admin/kb/articles/{id}/unpublish",  requireAdmin(hKBAdminUnpublishArticle(d)))
	r.Get("/api/admin/kb/articles/{id}/engagement",  requireAdmin(hKBAdminEngagement(d)))

	r.Get("/api/admin/kb/categories",      requireAdmin(hKBAdminListCategories(d)))
	r.Post("/api/admin/kb/categories",     requireAdmin(hKBAdminCreateCategory(d)))
	r.Put("/api/admin/kb/categories/{id}", requireAdmin(hKBAdminUpdateCategory(d)))
	r.Delete("/api/admin/kb/categories/{id}", requireAdmin(hKBAdminDeleteCategory(d)))

	r.Get("/api/admin/kb/tags",             requireAdmin(hKBAdminListTags(d)))
	r.Post("/api/admin/kb/tags",            requireAdmin(hKBAdminCreateTag(d)))
	r.Put("/api/admin/kb/tags/{id}",        requireAdmin(hKBAdminRenameTag(d)))
	r.Delete("/api/admin/kb/tags/{id}",     requireAdmin(hKBAdminDeleteTag(d)))
	r.Post("/api/admin/kb/tags/merge",      requireAdmin(hKBAdminMergeTags(d)))

	r.Post("/api/admin/kb/images",          requireAdmin(hKBAdminUploadImage(d)))
	r.Post("/api/admin/kb/cron/run",        requireAdmin(hKBAdminRunCron(d)))

	// Speedtest module routes.
	r.Get("/speedtest",                       requireUser(hSTSpeedtestPage(d)))
	r.Get("/api/customer/speedtest/endpoints",requireUser(hSTCustomerEndpoints(d)))
	r.Get("/api/customer/speedtest/auto",     requireUser(hSTCustomerAuto(d)))
	r.Post("/api/customer/speedtest/results", requireUser(hSTCustomerSaveResult(d)))
	r.Get("/api/customer/speedtest/results",  requireUser(hSTCustomerHistory(d)))

	r.Get("/admin/speedtest",            requireAdmin(hSTAdminEndpointsPage(d)))
	r.Get("/admin/speedtest/geoip",      requireAdmin(hSTAdminGeoIPPage(d)))
	r.Get("/admin/speedtest/results",    requireAdmin(hSTAdminResultsPage(d)))
	r.Get("/admin/speedtest/dashboards", requireAdmin(hSTAdminDashboardsPage(d)))

	r.Get   ("/api/admin/speedtest/endpoints",           requireAdmin(hSTAdminListEndpoints(d)))
	r.Post  ("/api/admin/speedtest/endpoints",           requireAdmin(hSTAdminCreateEndpoint(d)))
	r.Put   ("/api/admin/speedtest/endpoints/{id}",      requireAdmin(hSTAdminUpdateEndpoint(d)))
	r.Delete("/api/admin/speedtest/endpoints/{id}",      requireAdmin(hSTAdminDeleteEndpoint(d)))
	r.Post  ("/api/admin/speedtest/endpoints/{id}/ping", requireAdmin(hSTAdminPingEndpoint(d)))

	r.Get   ("/api/admin/speedtest/geoip",              requireAdmin(hSTAdminListGeoIPSources(d)))
	r.Post  ("/api/admin/speedtest/geoip",              requireAdmin(hSTAdminCreateGeoIPSource(d)))
	r.Put   ("/api/admin/speedtest/geoip/{id}",         requireAdmin(hSTAdminUpdateGeoIPSource(d)))
	r.Delete("/api/admin/speedtest/geoip/{id}",         requireAdmin(hSTAdminDeleteGeoIPSource(d)))
	r.Post  ("/api/admin/speedtest/geoip/{id}/refresh", requireAdmin(hSTAdminRefreshGeoIPSource(d)))
	r.Post  ("/api/admin/speedtest/geoip/{id}/test",    requireAdmin(hSTAdminTestGeoIPSource(d)))

	r.Get   ("/api/admin/speedtest/results",            requireAdmin(hSTAdminListResults(d)))
	r.Get   ("/api/admin/speedtest/dashboards",         requireAdmin(hSTAdminDashboards(d)))

	// Tickets module routes.
	r.Get ("/tickets",                            requireUser(hTKListPage(d)))
	r.Get ("/tickets/new",                        requireUser(hTKNewPage(d)))
	r.Get ("/tickets/{tracking_number}",          requireUser(hTKDetailPage(d)))
	r.Get ("/api/customer/categories",            requireUser(hTKCategoriesForCustomer(d)))
	r.Get ("/api/customer/tickets",               requireUser(hTKCustomerList(d)))
	r.Post("/api/customer/tickets",               requireUser(hTKCustomerCreate(d)))
	r.Get ("/api/customer/tickets/{tracking_number}",         requireUser(hTKCustomerDetail(d)))
	r.Post("/api/customer/tickets/{tracking_number}/reply",   requireUser(hTKCustomerReply(d)))
	r.Post("/api/customer/tickets/{tracking_number}/reopen",  requireUser(hTKCustomerReopen(d)))

	r.Get ("/admin/tickets",                      requireAdmin(hTKAdminQueuePage(d)))
	r.Get ("/admin/tickets/categories",           requireAdmin(hTKAdminCategoriesPage(d)))
	r.Get ("/admin/tickets/{tracking_number}",    requireAdmin(hTKAdminDetailPage(d)))
	r.Get ("/api/admin/tickets",                  requireAdmin(hTKAdminQueue(d)))
	r.Get ("/api/admin/tickets/{tracking_number}",            requireAdmin(hTKAdminDetail(d)))
	r.Post("/api/admin/tickets/{tracking_number}/reply",      requireAdmin(hTKAdminReply(d)))
	r.Post("/api/admin/tickets/{tracking_number}/note",       requireAdmin(hTKAdminNote(d)))
	r.Post("/api/admin/tickets/{tracking_number}/status",     requireAdmin(hTKAdminStatus(d)))
	r.Post("/api/admin/tickets/{tracking_number}/assign",     requireAdmin(hTKAdminAssign(d)))
	r.Post("/api/admin/tickets/cron/run",         requireAdmin(hTKAdminRunCron(d)))

	r.Get   ("/api/admin/categories",             requireAdmin(hTKAdminListCategoriesAdmin(d)))
	r.Post  ("/api/admin/categories",             requireAdmin(hTKAdminCreateCategory(d)))
	r.Put   ("/api/admin/categories/{id}",        requireAdmin(hTKAdminUpdateCategory(d)))
	r.Delete("/api/admin/categories/{id}",        requireAdmin(hTKAdminDeleteCategory(d)))

	r.Get   ("/api/admin/subcategories",          requireAdmin(hTKAdminListSubcategories(d)))
	r.Post  ("/api/admin/subcategories",          requireAdmin(hTKAdminCreateSubcategory(d)))
	r.Put   ("/api/admin/subcategories/{id}",     requireAdmin(hTKAdminUpdateSubcategory(d)))
	r.Delete("/api/admin/subcategories/{id}",     requireAdmin(hTKAdminDeleteSubcategory(d)))

	r.Get   ("/api/admin/category-fields",        requireAdmin(hTKAdminListFields(d)))
	r.Post  ("/api/admin/category-fields",        requireAdmin(hTKAdminCreateField(d)))
	r.Put   ("/api/admin/category-fields/{id}",   requireAdmin(hTKAdminUpdateField(d)))
	r.Delete("/api/admin/category-fields/{id}",   requireAdmin(hTKAdminDeleteField(d)))

	r.Post  ("/api/tickets/entries/{entry_id}/attachments", requireUser(hTKUploadAttachment(d)))
	r.Get   ("/api/attachments/{id}",             requireUser(hTKServeAttachment(d)))

	return r
}
