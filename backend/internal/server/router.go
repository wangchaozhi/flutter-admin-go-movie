package server

import (
	"net/http"
	"strings"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/auth"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/config"
	"flutter-admin-go/internal/payment"
	"flutter-admin-go/internal/video"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/admin/login", auth.AdminLoginHandler)
	mux.HandleFunc("/api/mobile/login", auth.MobileLoginHandler)
	mux.HandleFunc("/api/mobile/profile", auth.MobileProfileHandler)
	mux.HandleFunc("/api/mobile/watch-history", video.AppWatchHistoryHandler)
	mux.HandleFunc("/api/mobile/favorites", video.AppFavoritesHandler)
	mux.HandleFunc("/api/mobile/favorites/", video.AppFavoriteByVideoHandler)
	mux.HandleFunc("/api/mobile/settings", video.AppMobileSettingsHandler)

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

	// app user management (requires admin auth)
	mux.Handle("/api/admin/app-users", requireAdminAuth(http.HandlerFunc(admin.AppUsersHandler)))
	mux.Handle("/api/admin/app-users/", requireAdminAuth(http.HandlerFunc(admin.AppUserByIDHandler)))
	mux.Handle("/api/admin/products", requireAdminAuth(http.HandlerFunc(payment.AdminProductsHandler)))
	mux.Handle("/api/admin/products/", requireAdminAuth(http.HandlerFunc(payment.AdminProductByIDHandler)))
	mux.Handle("/api/admin/orders", requireAdminAuth(http.HandlerFunc(payment.AdminOrdersHandler)))

	// admin video management (requires admin auth)
	mux.Handle("/api/admin/videos", requireAdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			video.AdminCreateVideoHandler(w, r)
		} else {
			video.AdminListVideosHandler(w, r)
		}
	})))
	mux.Handle("/api/admin/videos/", requireAdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/upload"):
			video.AdminUploadVideoHandler(w, r)
		case strings.HasSuffix(path, "/cover"):
			video.AdminUploadCoverHandler(w, r)
		case strings.Contains(path, "/tasks"):
			video.AdminVideoTasksHandler(w, r)
		case strings.HasSuffix(path, "/transcode"):
			if r.Method == "GET" {
				video.AdminTranscodeStatusHandler(w, r)
			} else {
				video.AdminTranscodeHandler(w, r)
			}
		default:
			if r.Method == "GET" {
				video.AdminGetVideoHandler(w, r)
			} else if r.Method == "PUT" {
				video.AdminUpdateVideoHandler(w, r)
			} else if r.Method == "DELETE" {
				video.AdminDeleteVideoHandler(w, r)
			} else {
				common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
			}
		}
	})))

	// admin categories (requires admin auth)
	mux.Handle("/api/admin/categories", requireAdminAuth(http.HandlerFunc(video.AdminCategoriesHandler)))
	mux.Handle("/api/admin/categories/", requireAdminAuth(http.HandlerFunc(video.AdminCategoryByIDHandler)))

	// app: categories + video list / detail / play / cover / progress
	mux.HandleFunc("/api/categories", video.AppListCategoriesHandler)
	mux.HandleFunc("/api/products", payment.ProductsHandler)
	mux.HandleFunc("/api/orders", payment.OrdersHandler)
	mux.HandleFunc("/api/orders/", payment.OrderByNoHandler)
	mux.HandleFunc("/api/videos", video.AppListVideosHandler)
	mux.HandleFunc("/api/videos/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/play"):
			video.AppPlayHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/cover"):
			video.AppCoverHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/progress"):
			video.AppProgressHandler(w, r)
		default:
			video.AppGetVideoHandler(w, r)
		}
	})

	// hls m3u8 dynamic rewrite
	mux.HandleFunc("/api/hls/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "master.m3u8") {
			video.HLSMasterHandler(w, r)
		} else if strings.HasSuffix(r.URL.Path, "index.m3u8") {
			video.HLSIndexHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]string{"status": "up"}})
	})
	mux.HandleFunc("/api/webhooks/stripe", payment.StripeWebhookHandler)
	mux.HandleFunc("/api/webhooks/paypal", payment.PayPalWebhookHandler)

	return withCORS(mux)
}

func requireAdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := admin.CurrentAdminUsername(r); !ok {
			common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	allowed := config.Load().AllowedOrigins
	allowAll := false
	for _, o := range allowed {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && originAllowed(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}
