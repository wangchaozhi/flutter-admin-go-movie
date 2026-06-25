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
		query := r.URL.Query()
		keyword := strings.TrimSpace(query.Get("q"))
		vipFilter := query.Get("vip") // "" | "vip" | "none"
		sortDir := query.Get("sort")  // "asc" | "desc" (by created_at)

		db := store.DB()
		if keyword != "" {
			like := "%" + strings.ToLower(keyword) + "%"
			db = db.Where(
				"CAST(id AS TEXT) LIKE ? OR LOWER(username) LIKE ? OR LOWER(nickname) LIKE ? OR LOWER(email) LIKE ?",
				like, like, like, like,
			)
		}
		now := time.Now()
		switch vipFilter {
		case "vip":
			db = db.Where("vip_until IS NOT NULL AND vip_until > ?", now)
		case "none":
			db = db.Where("vip_until IS NULL OR vip_until <= ?", now)
		}

		order := "created_at DESC, id DESC"
		if sortDir == "asc" {
			order = "created_at ASC, id ASC"
		}
		db = db.Order(order)

		if !common.HasPagination(r) {
			var users []store.MobileUser
			db.Find(&users)
			common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: users})
			return
		}
		p := common.ParsePagination(r, 20, 100)
		var total int64
		db.Model(&store.MobileUser{}).Count(&total)
		var users []store.MobileUser
		db.Offset(p.Offset).Limit(p.PerPage).Find(&users)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(users, total, p)})

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
// POST /api/admin/app-users/{id}/vip
func AppUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/app-users/"), "/")
	parts := strings.Split(rest, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	if len(parts) > 1 && parts[1] == "vip" {
		appUserVIPHandler(w, r, id)
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

// appUserVIPHandler grants or revokes VIP time by a signed delta (days/minutes).
// Adding extends from the later of now and the current expiry; subtracting past
// now clears VIP membership entirely.
func appUserVIPHandler(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var req struct {
		Days    int `json:"days"`
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	delta := time.Duration(req.Days)*24*time.Hour + time.Duration(req.Minutes)*time.Minute
	if delta == 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "no time delta"})
		return
	}

	var user store.MobileUser
	if err := store.DB().First(&user, id).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "user not found"})
		return
	}

	now := time.Now()
	base := now
	if user.VIPUntil != nil && user.VIPUntil.After(now) {
		base = *user.VIPUntil
	}
	newUntil := base.Add(delta)

	updates := map[string]interface{}{"updated_at": now}
	var resultUntil *time.Time
	if newUntil.After(now) {
		t := newUntil
		updates["vip_until"] = t
		resultUntil = &t
	} else {
		// Subtracting below the current time revokes membership.
		updates["vip_until"] = nil
	}
	if err := store.DB().Model(&store.MobileUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]interface{}{
		"id":        id,
		"vip_until": resultUntil,
		"is_vip":    resultUntil != nil,
	}})
}
