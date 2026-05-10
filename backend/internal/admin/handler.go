package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
	_ "image/png"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	RoleIDs  []int  `json:"roleIds"`
}

type Role struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Key     string `json:"key"`
	MenuIDs []int  `json:"menuIds"`
}

type Menu struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	ParentID   int    `json:"parentId"`
	Type       string `json:"type"`
	Permission string `json:"permission"`
}

type userPayload struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Nickname string `json:"nickname"`
	RoleIDs  []int  `json:"roleIds"`
}

type rolePayload struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	MenuIDs []int  `json:"menuIds"`
}

type menuPayload struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	ParentID   int    `json:"parentId"`
	Type       string `json:"type"`
	Permission string `json:"permission"`
}

type Profile struct {
	Username     string   `json:"username"`
	MenuPaths    []string `json:"menuPaths"`
	Permissions  []string `json:"permissions"`
	Theme        string   `json:"theme"`
	AvatarURL    string   `json:"avatarUrl"`
	ThumbnailURL string   `json:"thumbnailUrl"`
}

type themePayload struct {
	Theme string `json:"theme"`
}

func UsersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !authorize(w, r, "") {
			return
		}
		listUsers(w)
	case http.MethodPost:
		if !authorize(w, r, "user:create") {
			return
		}
		createUser(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func UserByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.URL.Path, "/api/admin/users/")
	if !ok {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !authorize(w, r, "user:edit") {
			return
		}
		updateUser(w, r, id)
	case http.MethodDelete:
		if !authorize(w, r, "user:delete") {
			return
		}
		deleteByID(w, "admin_users", id)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func RolesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !authorize(w, r, "") {
			return
		}
		listRoles(w)
	case http.MethodPost:
		if !authorize(w, r, "role:create") {
			return
		}
		createRole(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func RoleByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.URL.Path, "/api/admin/roles/")
	if !ok {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !authorize(w, r, "role:edit") {
			return
		}
		updateRole(w, r, id)
	case http.MethodDelete:
		if !authorize(w, r, "role:delete") {
			return
		}
		deleteByID(w, "admin_roles", id)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func MenusHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !authorize(w, r, "") {
			return
		}
		listMenus(w)
	case http.MethodPost:
		if !authorize(w, r, "menu:create") {
			return
		}
		createMenu(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func MenuByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.URL.Path, "/api/admin/menus/")
	if !ok {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !authorize(w, r, "menu:edit") {
			return
		}
		updateMenu(w, r, id)
	case http.MethodDelete:
		if !authorize(w, r, "menu:delete") {
			return
		}
		deleteByID(w, "admin_menus", id)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		showProfile(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func ProfileThemeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	username, ok := CurrentAdminUsername(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var req themePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if !validTheme(req.Theme) {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid theme"})
		return
	}
	if err := store.DB().Model(&store.AdminUser{}).Where("username = ?", username).Update("theme", req.Theme).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func ProfileAvatarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	username, ok := CurrentAdminUsername(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid multipart form"})
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "avatar required"})
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, 6<<20))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "read avatar failed"})
		return
	}
	contentType := http.DetectContentType(raw)
	if contentType != "image/jpeg" && contentType != "image/png" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "only jpeg and png are supported"})
		return
	}
	thumbnail, err := makeThumbnail(raw)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "decode avatar failed"})
		return
	}

	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	stamp := time.Now().UnixNano()
	safeName := path.Base(header.Filename)
	avatarKey := fmt.Sprintf("avatars/%s/%d-%s%s", username, stamp, strings.TrimSuffix(safeName, path.Ext(safeName)), ext)
	thumbnailKey := fmt.Sprintf("thumbnails/%s/%d.jpg", username, stamp)

	client := store.ObjectClient()
	ctx := context.Background()
	if _, err = client.PutObject(ctx, store.AvatarBucket(), avatarKey, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: contentType}); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if _, err = client.PutObject(ctx, store.AvatarBucket(), thumbnailKey, bytes.NewReader(thumbnail), int64(len(thumbnail)), minio.PutObjectOptions{ContentType: "image/jpeg"}); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err = store.DB().Model(&store.AdminUser{}).Where("username = ?", username).Updates(map[string]interface{}{
		"avatar_key":    avatarKey,
		"thumbnail_key": thumbnailKey,
	}).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	profile, err := BuildProfile(username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: profile})
}

func ProfileAssetHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := CurrentAdminUsername(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	kind := strings.TrimPrefix(r.URL.Path, "/api/admin/profile/assets/")
	if kind != "avatar" && kind != "thumbnail" {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	var user store.AdminUser
	if err := store.DB().Where("username = ?", username).First(&user).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	objectKey := user.AvatarKey
	if kind == "thumbnail" {
		objectKey = user.ThumbnailKey
	}
	if objectKey == "" {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	object, err := store.ObjectClient().GetObject(context.Background(), store.AvatarBucket(), objectKey, minio.GetObjectOptions{})
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = io.Copy(w, object)
}

func showProfile(w http.ResponseWriter, r *http.Request) {
	username, ok := CurrentAdminUsername(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	profile, err := BuildProfile(username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: profile})
}

func listUsers(w http.ResponseWriter) {
	var records []store.AdminUser
	if err := store.DB().Order("id ASC").Find(&records).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	result := make([]User, 0, len(records))
	for _, record := range records {
		result = append(result, User{
			ID:       record.ID,
			Username: record.Username,
			Nickname: record.Nickname,
			RoleIDs:  intSlice(record.RoleIDs),
		})
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: result})
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var req userPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if req.Username == "" || req.Password == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "username and password required"})
		return
	}
	record := store.AdminUser{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
		RoleIDs:  store.IntArray(req.RoleIDs),
	}
	if err := store.DB().Create(&record).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func updateUser(w http.ResponseWriter, r *http.Request, id int) {
	var req userPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if req.Username == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "username required"})
		return
	}
	updates := map[string]interface{}{
		"username": req.Username,
		"nickname": req.Nickname,
		"role_ids": store.IntArray(req.RoleIDs),
	}
	if req.Password != "" {
		updates["password"] = req.Password
	}
	if err := store.DB().Model(&store.AdminUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func listRoles(w http.ResponseWriter) {
	var records []store.AdminRole
	if err := store.DB().Order("id ASC").Find(&records).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	result := make([]Role, 0, len(records))
	for _, record := range records {
		result = append(result, Role{
			ID:      record.ID,
			Name:    record.Name,
			Key:     record.Key,
			MenuIDs: intSlice(record.MenuIDs),
		})
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: result})
}

func createRole(w http.ResponseWriter, r *http.Request) {
	var req rolePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if req.Name == "" || req.Key == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "name and key required"})
		return
	}
	record := store.AdminRole{Name: req.Name, Key: req.Key, MenuIDs: store.IntArray(req.MenuIDs)}
	if err := store.DB().Create(&record).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func updateRole(w http.ResponseWriter, r *http.Request, id int) {
	var req rolePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Key) == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "name and key required"})
		return
	}
	updates := map[string]interface{}{
		"name":     req.Name,
		"role_key": req.Key,
		"menu_ids": store.IntArray(req.MenuIDs),
	}
	if err := store.DB().Model(&store.AdminRole{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func listMenus(w http.ResponseWriter) {
	var records []store.AdminMenu
	if err := store.DB().Order("id ASC").Find(&records).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	result := make([]Menu, 0, len(records))
	for _, record := range records {
		menu := Menu{
			ID:         record.ID,
			Name:       record.Name,
			Path:       record.Path,
			ParentID:   record.ParentID,
			Type:       record.Type,
			Permission: record.Permission,
		}
		if menu.Type == "" {
			menu.Type = "menu"
		}
		result = append(result, menu)
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: result})
}

func createMenu(w http.ResponseWriter, r *http.Request) {
	var req menuPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	normalizeMenuPayload(&req)
	if req.Name == "" || (req.Type == "menu" && req.Path == "") || (req.Type == "button" && req.Permission == "") {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid menu"})
		return
	}
	record := store.AdminMenu{
		Name:       req.Name,
		Path:       req.Path,
		ParentID:   req.ParentID,
		Type:       req.Type,
		Permission: req.Permission,
	}
	if err := store.DB().Create(&record).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func updateMenu(w http.ResponseWriter, r *http.Request, id int) {
	var req menuPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	normalizeMenuPayload(&req)
	if strings.TrimSpace(req.Name) == "" || (req.Type == "menu" && strings.TrimSpace(req.Path) == "") || (req.Type == "button" && strings.TrimSpace(req.Permission) == "") {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid menu"})
		return
	}
	if req.ParentID == id {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "parent can not be self"})
		return
	}
	updates := map[string]interface{}{
		"name":       req.Name,
		"path":       req.Path,
		"parent_id":  req.ParentID,
		"type":       req.Type,
		"permission": req.Permission,
	}
	if err := store.DB().Model(&store.AdminMenu{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func deleteByID(w http.ResponseWriter, table string, id int) {
	err := store.DB().Transaction(func(tx *gorm.DB) error {
		switch table {
		case "admin_users":
			if err := tx.Delete(&store.AdminUser{}, id).Error; err != nil {
				return err
			}
		case "admin_roles":
			if err := tx.Delete(&store.AdminRole{}, id).Error; err != nil {
				return err
			}
		case "admin_menus":
			if err := tx.Delete(&store.AdminMenu{}, id).Error; err != nil {
				return err
			}
		default:
			return errors.New("invalid table")
		}
		return cleanupDeletedReference(tx, table, id)
	})
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func cleanupDeletedReference(tx *gorm.DB, table string, id int) error {
	switch table {
	case "admin_roles":
		var users []store.AdminUser
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			next := store.IntArray(removeInt(intSlice(user.RoleIDs), id))
			if err := tx.Model(&store.AdminUser{}).Where("id = ?", user.ID).Update("role_ids", next).Error; err != nil {
				return err
			}
		}
		return nil
	case "admin_menus":
		var roles []store.AdminRole
		if err := tx.Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			next := store.IntArray(removeInt(intSlice(role.MenuIDs), id))
			if err := tx.Model(&store.AdminRole{}).Where("id = ?", role.ID).Update("menu_ids", next).Error; err != nil {
				return err
			}
		}
		return tx.Model(&store.AdminMenu{}).Where("parent_id = ?", id).Update("parent_id", 0).Error
	default:
		return nil
	}
}

func removeInt(values []int, target int) []int {
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func intSlice(values store.IntArray) []int {
	if values == nil {
		return []int{}
	}
	return []int(values)
}

func parseID(path, prefix string) (int, bool) {
	raw := strings.TrimPrefix(path, prefix)
	if raw == path || raw == "" {
		return 0, false
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func normalizeMenuPayload(req *menuPayload) {
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	req.Type = strings.TrimSpace(req.Type)
	req.Permission = strings.TrimSpace(req.Permission)
	if req.Type == "" {
		req.Type = "menu"
	}
	if req.Type != "button" {
		req.Type = "menu"
		req.Permission = ""
	}
	if req.Type == "button" {
		req.Path = ""
	}
}

func BuildAdminToken(username string) string {
	return "admin-token:" + username
}

func CurrentAdminUsername(r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	raw = strings.TrimPrefix(raw, "Bearer ")
	if strings.HasPrefix(raw, "admin-token:") {
		username := strings.TrimPrefix(raw, "admin-token:")
		return username, username != ""
	}
	return "", false
}

func BuildProfile(username string) (Profile, error) {
	var user store.AdminUser
	if err := store.DB().Where("username = ?", username).First(&user).Error; err != nil {
		return Profile{}, err
	}
	roleIDs := intSlice(user.RoleIDs)
	if len(roleIDs) == 0 {
		return Profile{Username: username, MenuPaths: []string{}, Permissions: []string{}}, nil
	}

	var roles []store.AdminRole
	if err := store.DB().Order("id ASC").Find(&roles).Error; err != nil {
		return Profile{}, err
	}

	menuIDs := map[int]bool{}
	for _, role := range roles {
		if !containsInt(roleIDs, role.ID) {
			continue
		}
		for _, menuID := range intSlice(role.MenuIDs) {
			menuIDs[menuID] = true
		}
	}

	var menus []store.AdminMenu
	if err := store.DB().Order("id ASC").Find(&menus).Error; err != nil {
		return Profile{}, err
	}

	menuSet := map[string]bool{}
	permSet := map[string]bool{}
	for _, menu := range menus {
		if !menuIDs[menu.ID] {
			continue
		}
		if menu.Type == "button" {
			if menu.Permission != "" {
				permSet[menu.Permission] = true
			}
			continue
		}
		if menu.Path != "" {
			menuSet[menu.Path] = true
			if code := menuPermissionCode(menu.Path); code != "" {
				permSet[code] = true
			}
		}
	}

	return Profile{
		Username:     username,
		MenuPaths:    sortedKeys(menuSet),
		Permissions:  sortedKeys(permSet),
		Theme:        normalizeTheme(user.Theme),
		AvatarURL:    profileAssetURL(user.AvatarKey, "avatar"),
		ThumbnailURL: profileAssetURL(user.ThumbnailKey, "thumbnail"),
	}, nil
}

func profileAssetURL(objectKey, kind string) string {
	if objectKey == "" {
		return ""
	}
	return "/api/admin/profile/assets/" + kind
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func authorize(w http.ResponseWriter, r *http.Request, permission string) bool {
	username, ok := CurrentAdminUsername(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return false
	}
	if permission == "" {
		return true
	}
	profile, err := BuildProfile(username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return false
	}
	for _, item := range profile.Permissions {
		if item == permission {
			return true
		}
	}
	common.WriteJSON(w, http.StatusForbidden, common.APIResponse{Code: 403, Msg: "forbidden"})
	return false
}

func menuPermissionCode(path string) string {
	switch path {
	case "/system/user":
		return "system:user"
	case "/system/role":
		return "system:role"
	case "/system/menu":
		return "system:menu"
	default:
		return ""
	}
}

func validTheme(theme string) bool {
	return theme == "system" || theme == "light" || theme == "dark"
}

func normalizeTheme(theme string) string {
	if validTheme(theme) {
		return theme
	}
	return "system"
}

func makeThumbnail(raw []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid image")
	}

	size := 128
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	shortSide := width
	if height < shortSide {
		shortSide = height
	}
	srcX := bounds.Min.X + (width-shortSide)/2
	srcY := bounds.Min.Y + (height-shortSide)/2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := srcX + x*shortSide/size
			py := srcY + y*shortSide/size
			dst.Set(x, y, src.At(px, py))
		}
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 86}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j] < result[i] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func MustGetAdminUser(username, password string) (bool, error) {
	var user store.AdminUser
	err := store.DB().Where("username = ? AND password = ?", username, password).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func MustGetMobileUser(username, password string) (bool, error) {
	var user store.MobileUser
	err := store.DB().Where("username = ? AND password = ?", username, password).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
