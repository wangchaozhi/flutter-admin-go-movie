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
	mux.Handle("/api/mobile/profile", mobileBanGuard(http.HandlerFunc(auth.MobileProfileHandler)))
	mux.Handle("/api/mobile/watch-history", mobileBanGuard(http.HandlerFunc(video.AppWatchHistoryHandler)))
	mux.Handle("/api/mobile/favorites", mobileBanGuard(http.HandlerFunc(video.AppFavoritesHandler)))
	mux.Handle("/api/mobile/favorites/", mobileBanGuard(http.HandlerFunc(video.AppFavoriteByVideoHandler)))
	mux.Handle("/api/mobile/settings", mobileBanGuard(http.HandlerFunc(video.AppMobileSettingsHandler)))

	mux.Handle("/api/admin/stats", requireAdminAuth(http.HandlerFunc(admin.StatsHandler)))
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

	// app user management (button permissions enforced per method)
	mux.Handle("/api/admin/app-users", requirePerm(map[string]string{
		http.MethodPost: "app_user:create",
	}, http.HandlerFunc(admin.AppUsersHandler)))
	mux.Handle("/api/admin/app-users/", requirePerm(map[string]string{
		// PUT edits, POST .../vip grants time (both edits), DELETE removes.
		http.MethodPut:    "app_user:edit",
		http.MethodPost:   "app_user:edit",
		http.MethodDelete: "app_user:delete",
	}, http.HandlerFunc(admin.AppUserByIDHandler)))
	mux.Handle("/api/admin/products", requirePerm(map[string]string{
		http.MethodPost: "payment:product",
	}, http.HandlerFunc(payment.AdminProductsHandler)))
	mux.Handle("/api/admin/products/", requirePerm(map[string]string{
		http.MethodPut:    "payment:product",
		http.MethodDelete: "payment:product",
	}, http.HandlerFunc(payment.AdminProductByIDHandler)))
	mux.Handle("/api/admin/orders", requireAdminAuth(http.HandlerFunc(payment.AdminOrdersHandler)))
	mux.Handle("/api/admin/orders/", requirePerm(map[string]string{
		http.MethodDelete: "payment:order",
		http.MethodPost:   "payment:refund",
	}, http.HandlerFunc(payment.AdminOrderByIDHandler)))

	// admin video management (button permissions enforced per method)
	mux.Handle("/api/admin/video/transcode-tasks", requireAdminAuth(http.HandlerFunc(video.AdminTranscodeHistoryHandler)))
	mux.Handle("/api/admin/video/transcode-tasks/", requirePerm(map[string]string{
		http.MethodDelete: "video:transcode-history",
	}, http.HandlerFunc(video.AdminTranscodeHistoryByIDHandler)))
	mux.Handle("/api/admin/video/extract-tasks", requireAdminAuth(http.HandlerFunc(video.AdminExtractHistoryHandler)))
	mux.Handle("/api/admin/video/extract-tasks/", requirePerm(map[string]string{
		http.MethodDelete: "video:extract-history",
	}, http.HandlerFunc(video.AdminExtractHistoryByIDHandler)))
	mux.Handle("/api/admin/videos", requirePerm(map[string]string{
		http.MethodPost: "video:create",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			video.AdminCreateVideoHandler(w, r)
		} else {
			video.AdminListVideosHandler(w, r)
		}
	})))
	mux.Handle("/api/admin/videos/", requireAdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sub-actions of a single video. Uploading media/cover, AI metadata and
		// (re)transcoding are all "editing" the video; deleting tasks or the
		// video itself requires the delete permission. There is no dedicated
		// upload/cover/transcode button permission seeded, so they map to
		// video:edit. GET reads are open to any authenticated admin.
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/upload"):
			if !admin.EnsurePermission(w, r, "video:edit") {
				return
			}
			video.AdminUploadVideoHandler(w, r)
		case strings.HasSuffix(path, "/cover"):
			if !admin.EnsurePermission(w, r, "video:edit") {
				return
			}
			video.AdminUploadCoverHandler(w, r)
		case strings.HasSuffix(path, "/ai-metadata"):
			if !admin.EnsurePermission(w, r, "video:edit") {
				return
			}
			video.AdminGenerateAIMetadataHandler(w, r)
		case strings.Contains(path, "/tasks"):
			if r.Method == http.MethodDelete && !admin.EnsurePermission(w, r, "video:delete") {
				return
			}
			video.AdminVideoTasksHandler(w, r)
		case strings.HasSuffix(path, "/transcode"):
			if r.Method == "GET" {
				video.AdminTranscodeStatusHandler(w, r)
			} else if r.Method == "DELETE" {
				if !admin.EnsurePermission(w, r, "video:edit") {
					return
				}
				video.AdminCancelTranscodeHandler(w, r)
			} else {
				if !admin.EnsurePermission(w, r, "video:edit") {
					return
				}
				video.AdminTranscodeHandler(w, r)
			}
		default:
			if r.Method == "GET" {
				video.AdminGetVideoHandler(w, r)
			} else if r.Method == "PUT" {
				if !admin.EnsurePermission(w, r, "video:edit") {
					return
				}
				video.AdminUpdateVideoHandler(w, r)
			} else if r.Method == "DELETE" {
				if !admin.EnsurePermission(w, r, "video:delete") {
					return
				}
				video.AdminDeleteVideoHandler(w, r)
			} else {
				common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
			}
		}
	})))

	// admin categories (button permissions enforced per method)
	mux.Handle("/api/admin/categories", requirePerm(map[string]string{
		http.MethodPost: "category:create",
	}, http.HandlerFunc(video.AdminCategoriesHandler)))
	mux.Handle("/api/admin/categories/", requirePerm(map[string]string{
		http.MethodPut:    "category:edit",
		http.MethodDelete: "category:delete",
	}, http.HandlerFunc(video.AdminCategoryByIDHandler)))

	// admin comment moderation
	mux.Handle("/api/admin/comments", requireAdminAuth(http.HandlerFunc(video.AdminListCommentsHandler)))
	mux.Handle("/api/admin/comments/", requirePerm(map[string]string{
		http.MethodDelete: "comment:delete",
	}, http.HandlerFunc(video.AdminDeleteCommentHandler)))

	// app: categories + video list / detail / play / cover / progress
	mux.HandleFunc("/api/categories", video.AppListCategoriesHandler)
	mux.HandleFunc("/api/products", payment.ProductsHandler)
	mux.Handle("/api/orders", mobileBanGuard(http.HandlerFunc(payment.OrdersHandler)))
	mux.Handle("/api/orders/", mobileBanGuard(http.HandlerFunc(payment.OrderByNoHandler)))
	mux.HandleFunc("/api/videos", video.AppListVideosHandler)
	mux.Handle("/api/videos/", mobileBanGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/play"):
			video.AppPlayHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/cover"):
			video.AppCoverHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/progress"):
			video.AppProgressHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			video.AppVideoCommentsHandler(w, r)
		default:
			video.AppGetVideoHandler(w, r)
		}
	})))
	mux.Handle("/api/mobile/comments/", mobileBanGuard(http.HandlerFunc(video.AppCommentByIDHandler)))

	// hls m3u8 dynamic rewrite
	mux.HandleFunc("/api/hls/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "master.m3u8") {
			video.HLSMasterHandler(w, r)
		} else if strings.HasSuffix(r.URL.Path, "index.m3u8") {
			video.HLSIndexHandler(w, r)
		} else if strings.HasSuffix(r.URL.Path, ".vtt") {
			video.HLSAssetHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]string{"status": "up"}})
	})
	mux.HandleFunc("/api/webhooks/stripe", payment.StripeWebhookHandler)
	mux.HandleFunc("/api/webhooks/paypal", payment.PayPalWebhookHandler)

	// Observability wraps everything (including CORS) so it can trace and recover
	// from panics in any layer.
	return withObservability(withCORS(mux))
}

// mobileBanGuard revokes access for users banned after they logged in: if the
// request carries a valid mobile token whose account is now banned, it responds
// 403 with code 4030 so the app can force a logout. Requests without a token
// (anonymous browsing) pass through untouched.
func mobileBanGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
		if raw != "" {
			if claims, err := admin.ParseMobileToken(raw); err == nil && admin.IsMobileUserBanned(claims.UserID) {
				common.WriteJSON(w, http.StatusForbidden, common.APIResponse{Code: 4030, Msg: "账号已被封禁，请联系管理员"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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

// requirePerm guards a handler with a button permission that depends on the HTTP
// method. A method absent from byMethod (or mapped to "") only requires a valid
// admin session, which keeps read (GET) access open to any authenticated admin
// while gating writes behind the documented permissions.
func requirePerm(byMethod map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !admin.EnsurePermission(w, r, byMethod[r.Method]) {
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
