package auth

import (
	"encoding/json"
	"net/http"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
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
	ok, err := admin.MustGetMobileUser(req.Username, req.Password)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "invalid credentials"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{Token: "mobile-token", Username: req.Username, Client: "mobile"}})
}
