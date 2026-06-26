package admin

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
)

// inviteCodeAlphabet excludes easily-confused characters (0/O, 1/I/L) so codes
// are readable when shared verbally or over chat.
const inviteCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// generateInviteCode returns a random readable code of the given length.
func generateInviteCode(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = inviteCodeAlphabet[int(b)%len(inviteCodeAlphabet)]
	}
	return string(out), nil
}

// InviteCodesHandler lists invite codes (GET) and generates one (POST).
// GET /api/admin/invite-codes
// POST /api/admin/invite-codes
func InviteCodesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		codes, err := store.ListInviteCodes()
		if err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: codes})

	case http.MethodPost:
		var req struct {
			Code      string `json:"code"`
			MaxUses   int    `json:"max_uses"`
			Note      string `json:"note"`
			ExpiresAt string `json:"expires_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
			return
		}

		code := strings.ToUpper(strings.TrimSpace(req.Code))
		if code == "" {
			generated, err := generateInviteCode(8)
			if err != nil {
				common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: "生成邀请码失败"})
				return
			}
			code = generated
		} else if n := len(code); n < 4 || n > 32 {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "邀请码长度需为 4-32 个字符"})
			return
		}
		if exists, err := store.InviteCodeExists(code); err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		} else if exists {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "邀请码已存在"})
			return
		}

		if req.MaxUses < 0 {
			req.MaxUses = 0
		}
		var expiresAt *time.Time
		if trimmed := strings.TrimSpace(req.ExpiresAt); trimmed != "" {
			parsed, err := time.Parse(time.RFC3339, trimmed)
			if err != nil {
				common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "过期时间格式不正确"})
				return
			}
			expiresAt = &parsed
		}

		creator, _ := CurrentAdminUsername(r)
		ic := &store.InviteCode{
			Code:      code,
			MaxUses:   req.MaxUses,
			Note:      strings.TrimSpace(req.Note),
			CreatedBy: creator,
			ExpiresAt: expiresAt,
			Status:    "active",
		}
		if err := store.CreateInviteCode(ic); err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: ic})

	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

// InviteCodeByIDHandler toggles a code's status.
// POST /api/admin/invite-codes/{id}/disable
// POST /api/admin/invite-codes/{id}/enable
func InviteCodeByIDHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/invite-codes/"), "/")
	parts := strings.Split(rest, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	if r.Method != http.MethodPost || len(parts) < 2 {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}

	status := ""
	switch parts[1] {
	case "disable":
		status = "disabled"
	case "enable":
		status = "active"
	default:
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "unknown action"})
		return
	}
	if err := store.SetInviteCodeStatus(id, status); err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: map[string]any{"id": id, "status": status}})
}
