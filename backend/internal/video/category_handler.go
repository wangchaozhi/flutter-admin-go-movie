package video

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// GET /api/admin/categories
// POST /api/admin/categories
func AdminCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var cats []store.Category
		store.DB().Order("sort_order ASC, id ASC").Find(&cats)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: cats})
	case http.MethodPost:
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "name required"})
			return
		}
		cat := &store.Category{Name: req.Name, SortOrder: req.SortOrder}
		if err := store.DB().Create(cat).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: cat})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

// PUT /api/admin/categories/{id}
// DELETE /api/admin/categories/{id}
func AdminCategoryByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/categories/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}
		if err := store.DB().Model(&store.Category{}).Where("id = ?", id).Updates(map[string]interface{}{
			"name": req.Name, "sort_order": req.SortOrder,
		}).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	case http.MethodDelete:
		store.DB().Delete(&store.Category{}, id)
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

// GET /api/categories  (public, for app)
func AppListCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var cats []store.Category
	store.DB().Order("sort_order ASC, id ASC").Find(&cats)
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: cats})
}
