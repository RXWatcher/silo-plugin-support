package server

import "net/http"

func hCustomerHome(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeSPA(w, r, supportBootstrap{
			Mode:    "customer-home",
			Theme:   adminTheme(r),
			Modules: modules,
			UserID:  r.Header.Get("X-Silo-User-Id"),
			IsAdmin: r.Header.Get("X-Silo-User-Role") == "admin",
		}, http.StatusOK)
	}
}

func hCustomerBootstrap(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modules := currentModules(r.Context(), d)
		writeJSON(w, http.StatusOK, map[string]any{
			"modules": modules,
			"userId":  r.Header.Get("X-Silo-User-Id"),
			"isAdmin": r.Header.Get("X-Silo-User-Role") == "admin",
		})
	}
}
