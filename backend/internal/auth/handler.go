package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token        string   `json:"token"`
	Username     string   `json:"username"`
	Client       string   `json:"client"`
	MenuPaths    []string `json:"menuPaths,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	Theme        string   `json:"theme,omitempty"`
	AvatarURL    string   `json:"avatarUrl,omitempty"`
	ThumbnailURL string   `json:"thumbnailUrl,omitempty"`
}

type MobileProfileResponse struct {
	ID       int        `json:"id"`
	Username string     `json:"username"`
	Nickname string     `json:"nickname"`
	Email    string     `json:"email"`
	Status   string     `json:"status"`
	VIPUntil *time.Time `json:"vip_until"`
	IsVIP    bool       `json:"is_vip"`
}

func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	ok, err := admin.MustGetAdminUser(req.Username, req.Password)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "invalid credentials"})
		return
	}
	profile, err := admin.BuildProfile(req.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{
		Token:        admin.BuildAdminToken(req.Username),
		Username:     req.Username,
		Client:       "admin",
		MenuPaths:    profile.MenuPaths,
		Permissions:  profile.Permissions,
		Theme:        profile.Theme,
		AvatarURL:    profile.AvatarURL,
		ThumbnailURL: profile.ThumbnailURL,
	}})
}

func MobileLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	user, err := admin.GetMobileUser(req.Username, req.Password)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if user == nil {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "invalid credentials"})
		return
	}
	token, err := admin.BuildMobileToken(user.ID, user.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "token generation failed"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{Token: token, Username: user.Username, Client: "mobile"}})
}

func MobileProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var user store.MobileUser
	if err := store.DB().First(&user, userID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	now := time.Now()
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: MobileProfileResponse{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Email:    user.Email,
		Status:   user.Status,
		VIPUntil: user.VIPUntil,
		IsVIP:    user.VIPUntil != nil && user.VIPUntil.After(now),
	}})
}

func currentMobileUserID(r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	raw = strings.TrimPrefix(raw, "Bearer ")
	if raw == "" {
		return 0, false
	}
	claims, err := admin.ParseMobileToken(raw)
	if err != nil {
		return 0, false
	}
	return claims.UserID, true
}
