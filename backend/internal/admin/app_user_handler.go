package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// GET /api/admin/app-users
// POST /api/admin/app-users
func AppUsersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var users []store.MobileUser
		store.DB().Order("id asc").Find(&users)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: users})

	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
			Email    string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "username and password required"})
			return
		}
		hashed, err := HashPassword(req.Password)
		if err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "hash password failed"})
			return
		}
		u := &store.MobileUser{
			Username: req.Username,
			Password: hashed,
			Nickname: req.Nickname,
			Email:    req.Email,
			Status:   "active",
		}
		if err := store.DB().Create(u).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: u})

	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

// PUT /api/admin/app-users/{id}
// DELETE /api/admin/app-users/{id}
func AppUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/app-users/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			Nickname *string `json:"nickname"`
			Email    *string `json:"email"`
			Password *string `json:"password"`
			Status   *string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		updates := map[string]interface{}{"updated_at": time.Now()}
		if req.Nickname != nil {
			updates["nickname"] = *req.Nickname
		}
		if req.Email != nil {
			updates["email"] = *req.Email
		}
		if req.Password != nil && *req.Password != "" {
			hashed, err := HashPassword(*req.Password)
			if err != nil {
				common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "hash password failed"})
				return
			}
			updates["password"] = hashed
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		store.DB().Model(&store.MobileUser{}).Where("id = ?", id).Updates(updates)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})

	case http.MethodDelete:
		store.DB().Delete(&store.MobileUser{}, id)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})

	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}
