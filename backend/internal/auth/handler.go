package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
	"github.com/minio/minio-go/v7"
)

// Login brute-force guards. Counts are per client IP and reset on success.
var (
	adminLoginLimiter  = common.NewLimiter("admin_login", 5, 5*time.Minute)
	mobileLoginLimiter = common.NewLimiter("mobile_login", 10, 5*time.Minute)
	// registerLimiter caps self-service sign-ups per IP to curb abuse.
	registerLimiter = common.NewLimiter("mobile_register", 5, time.Hour)
)

// tooManyLoginAttempts writes a 429 with Retry-After when the caller is blocked,
// returning true so the handler can stop. It checks before touching the body so
// a flood of attempts is rejected cheaply.
func tooManyLoginAttempts(w http.ResponseWriter, limiter *common.Limiter, key string) bool {
	if blocked, retry := limiter.Blocked(key); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		common.WriteJSON(w, http.StatusTooManyRequests, common.APIResponse{Code: 429, Msg: "尝试次数过多，请稍后再试"})
		return true
	}
	return false
}

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
	ID            int        `json:"id"`
	Username      string     `json:"username"`
	Nickname      string     `json:"nickname"`
	Email         string     `json:"email"`
	Status        string     `json:"status"`
	AvatarURL     string     `json:"avatar_url"`
	VIPUntil      *time.Time `json:"vip_until"`
	IsVIP         bool       `json:"is_vip"`
	DaysRemaining int        `json:"days_remaining"`
}

func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	ip := common.ClientIP(r)
	if tooManyLoginAttempts(w, adminLoginLimiter, ip) {
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
		adminLoginLimiter.Fail(ip)
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "invalid credentials"})
		return
	}
	adminLoginLimiter.Reset(ip)
	profile, err := admin.BuildProfile(req.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	token, err := admin.BuildAdminToken(req.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "token generation failed"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{
		Token:        token,
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
	ip := common.ClientIP(r)
	if tooManyLoginAttempts(w, mobileLoginLimiter, ip) {
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	user, err := admin.GetMobileUser(req.Username, req.Password)
	if errors.Is(err, admin.ErrMobileUserBanned) {
		common.WriteJSON(w, http.StatusForbidden, common.APIResponse{Code: 403, Msg: "账号已被封禁，请联系管理员"})
		return
	}
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if user == nil {
		mobileLoginLimiter.Fail(ip)
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "invalid credentials"})
		return
	}
	mobileLoginLimiter.Reset(ip)
	token, err := admin.BuildMobileToken(user.ID, user.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "token generation failed"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{Token: token, Username: user.Username, Client: "mobile"}})
}

func MobileProfileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		showMobileProfile(w, r)
	case http.MethodPut:
		updateMobileProfile(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func showMobileProfile(w http.ResponseWriter, r *http.Request) {
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
	isVIP := user.VIPUntil != nil && user.VIPUntil.After(now)
	daysRemaining := 0
	if isVIP {
		// Round up so the last partial day still counts as 1 remaining.
		daysRemaining = int(user.VIPUntil.Sub(now).Hours()/24) + 1
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: MobileProfileResponse{
		ID:            user.ID,
		Username:      user.Username,
		Nickname:      user.Nickname,
		Email:         user.Email,
		Status:        user.Status,
		AvatarURL:     mobileProfileAvatarURL(user.AvatarKey),
		VIPUntil:      user.VIPUntil,
		IsVIP:         isVIP,
		DaysRemaining: daysRemaining,
	}})
}

func updateMobileProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if n := len([]rune(nickname)); n < 1 || n > 32 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "昵称长度需为 1-32 个字符"})
		return
	}
	if err := store.DB().Model(&store.MobileUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"nickname":   nickname,
		"updated_at": time.Now(),
	}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	showMobileProfile(w, r)
}

func MobileProfileAvatarHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		serveMobileProfileAvatar(w, r)
	case http.MethodPost:
		uploadMobileProfileAvatar(w, r)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func uploadMobileProfileAvatar(w http.ResponseWriter, r *http.Request) {
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
	avatar, err := makeMobileAvatar(raw)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "decode avatar failed"})
		return
	}

	stamp := time.Now().UnixNano()
	safeName := path.Base(header.Filename)
	objectKey := fmt.Sprintf("mobile-avatars/%d/%d-%s.jpg", userID, stamp, strings.TrimSuffix(safeName, path.Ext(safeName)))
	client := store.ObjectClient()
	if _, err = client.PutObject(context.Background(), store.AvatarBucket(), objectKey, bytes.NewReader(avatar), int64(len(avatar)), minio.PutObjectOptions{ContentType: "image/jpeg"}); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if err := store.DB().Model(&store.MobileUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"avatar_key": objectKey,
		"updated_at": time.Now(),
	}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	showMobileProfile(w, r)
}

func serveMobileProfileAvatar(w http.ResponseWriter, r *http.Request) {
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
	if user.AvatarKey == "" {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	object, err := store.ObjectClient().GetObject(context.Background(), store.AvatarBucket(), user.AvatarKey, minio.GetObjectOptions{})
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

// MobileRegisterHandler creates a new mobile account and returns a token so the
// app can sign the user straight in. Rate-limited per IP. No email verification
// (the project has no mail infrastructure), so the email is informational only.
func MobileRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	ip := common.ClientIP(r)
	if allowed, retry := registerLimiter.Allow(ip); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		common.WriteJSON(w, http.StatusTooManyRequests, common.APIResponse{Code: 429, Msg: "注册过于频繁，请稍后再试"})
		return
	}

	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		Nickname   string `json:"nickname"`
		Email      string `json:"email"`
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(req.Email)
	req.InviteCode = strings.ToUpper(strings.TrimSpace(req.InviteCode))
	if n := len([]rune(req.Username)); n < 3 || n > 32 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "用户名长度需为 3-32 个字符"})
		return
	}
	if len(req.Password) < 6 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "密码至少 6 位"})
		return
	}
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "邮箱格式不正确"})
		return
	}
	if req.InviteCode == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "请输入邀请码"})
		return
	}

	var count int64
	if err := store.DB().Model(&store.MobileUser{}).Where("username = ?", req.Username).Count(&count).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if count > 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "用户名已被占用"})
		return
	}

	// Invite-only sign-up: atomically consume one use of the code. Done after the
	// username check so a taken username doesn't burn an invite.
	if err := store.ConsumeInviteCode(req.InviteCode); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: inviteCodeErrorMessage(err)})
		return
	}

	hashed, err := admin.HashPassword(req.Password)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "hash password failed"})
		return
	}
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}
	now := time.Now()
	user := store.MobileUser{
		Username:   req.Username,
		Password:   hashed,
		Nickname:   nickname,
		Email:      req.Email,
		Status:     "active",
		InviteCode: req.InviteCode,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := store.DB().Create(&user).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	token, err := admin.BuildMobileToken(user.ID, user.Username)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "token generation failed"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: LoginResponse{
		Token: token, Username: user.Username, Client: "mobile",
	}})
}

// MobileChangePasswordHandler updates the signed-in user's password after
// verifying the current one.
func MobileChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	if len(req.NewPassword) < 6 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "新密码至少 6 位"})
		return
	}
	var user store.MobileUser
	if err := store.DB().First(&user, userID).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	if !admin.CheckPasswordHash(user.Password, req.OldPassword) {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "当前密码不正确"})
		return
	}
	hashed, err := admin.HashPassword(req.NewPassword)
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "hash password failed"})
		return
	}
	if err := store.DB().Model(&store.MobileUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password":   hashed,
		"updated_at": time.Now(),
	}).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

// inviteCodeErrorMessage maps a store invite-code error to a user-facing message.
func inviteCodeErrorMessage(err error) string {
	switch {
	case errors.Is(err, store.ErrInviteRequired):
		return "请输入邀请码"
	case errors.Is(err, store.ErrInviteDisabled):
		return "邀请码已停用"
	case errors.Is(err, store.ErrInviteExpired):
		return "邀请码已过期"
	case errors.Is(err, store.ErrInviteExhausted):
		return "邀请码使用次数已达上限"
	default:
		return "邀请码无效"
	}
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

func mobileProfileAvatarURL(objectKey string) string {
	if objectKey == "" {
		return ""
	}
	return "/api/mobile/profile/avatar?v=" + url.QueryEscape(objectKey)
}

func makeMobileAvatar(raw []byte) ([]byte, error) {
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

	size := 192
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
