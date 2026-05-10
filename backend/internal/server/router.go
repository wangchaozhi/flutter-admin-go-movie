package server

import (
	"net/http"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/auth"
	"flutter-admin-go/internal/common"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/admin/login", auth.AdminLoginHandler)
	mux.HandleFunc("/api/mobile/login", auth.MobileLoginHandler)

	mux.HandleFunc("/api/admin/profile", admin.ProfileHandler)
	mux.HandleFunc("/api/admin/profile/theme", admin.ProfileThemeHandler)
	mux.HandleFunc("/api/admin/profile/avatar", admin.ProfileAvatarHandler)
	mux.HandleFunc("/api/admin/profile/assets/", admin.ProfileAssetHandler)
	mux.HandleFunc("/api/admin/users", admin.UsersHandler)
	mux.HandleFunc("/api/admin/users/", admin.UserByIDHandler)
	mux.HandleFunc("/api/admin/roles", admin.RolesHandler)
	mux.HandleFunc("/api/admin/roles/", admin.RoleByIDHandler)
	mux.HandleFunc("/api/admin/menus", admin.MenusHandler)
	mux.HandleFunc("/api/admin/menus/", admin.MenuByIDHandler)

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]string{"status": "up"}})
	})

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
