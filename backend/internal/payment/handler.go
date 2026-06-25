package payment

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flutter-admin-go/internal/admin"
	"flutter-admin-go/internal/common"
	"flutter-admin-go/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type createOrderRequest struct {
	ProductCode string `json:"product_code"`
	Provider    string `json:"provider"`
}

type productPayload struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Kind         string `json:"kind"`
	PriceCents   int    `json:"price_cents"`
	Currency     string `json:"currency"`
	DurationDays int    `json:"duration_days"`
	VideoID      *int64 `json:"video_id"`
	Status       string `json:"status"`
}

var supportedProductCurrencies = map[string]bool{
	"CNY": true,
	"USD": true,
	"EUR": true,
	"JPY": true,
	"HKD": true,
	"TWD": true,
	"GBP": true,
	"AUD": true,
	"CAD": true,
	"SGD": true,
}

func ProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	var products []store.Product
	if err := store.DB().Where("status = ?", "active").Order("id asc").Find(&products).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: products})
}

func OrdersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		listMobileOrders(w, r)
		return
	}
	if r.Method != http.MethodPost {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	if req.ProductCode == "" || req.Provider == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "product_code and provider required"})
		return
	}

	var product store.Product
	if err := store.DB().Where("code = ? AND status = ?", req.ProductCode, "active").First(&product).Error; err != nil {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "product not found"})
		return
	}
	provider, err := providerFor(req.Provider, LoadConfig())
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}

	now := time.Now()
	expiresAt := now.Add(30 * time.Minute)
	order := store.Order{
		OrderNo:     newOrderNo(now),
		UserID:      userID,
		ProductID:   product.ID,
		Provider:    provider.Name(),
		Status:      "pending",
		AmountCents: product.PriceCents,
		Currency:    product.Currency,
		ExpiresAt:   &expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.DB().Create(&order).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}

	session, err := provider.CreateCheckout(r.Context(), order, product)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	updates := map[string]interface{}{
		"status":            "paying",
		"provider_order_id": session.ProviderOrderID,
		"checkout_url":      session.CheckoutURL,
		"updated_at":        time.Now(),
	}
	if err := store.DB().Model(&store.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	order.Status = "paying"
	order.ProviderOrderID = session.ProviderOrderID
	order.CheckoutURL = session.CheckoutURL
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: order})
}

func listMobileOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 50 {
		limit = 50
	}
	var orders []store.Order
	if err := withOrderProduct(store.DB()).
		Where("user_id = ?", userID).
		Order("id desc").
		Limit(limit).
		Find(&orders).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: orders})
}

func OrderByNoHandler(w http.ResponseWriter, r *http.Request) {
	orderNo, action := parseOrderPath(r.URL.Path)
	switch action {
	case "":
		if r.Method != http.MethodGet {
			common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
			return
		}
		showMobileOrder(w, r, orderNo)
	case "mock-complete":
		completeMockOrder(w, r, orderNo)
	default:
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
	}
}

func AdminProductsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var products []store.Product
		if err := store.DB().Order("id asc").Find(&products).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: products})
	case http.MethodPost:
		saveProduct(w, r, 0)
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func AdminProductByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/admin/products/"))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		saveProduct(w, r, id)
	case http.MethodDelete:
		result := store.DB().Delete(&store.Product{}, id)
		if result.Error != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "product not found"})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
	default:
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
	}
}

func AdminOrdersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		return
	}
	query := withOrderProduct(store.DB()).Preload("User").Order("id desc")
	countQuery := store.DB().Model(&store.Order{})
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		query = query.Where("status = ?", status)
		countQuery = countQuery.Where("status = ?", status)
	}

	if !common.HasPagination(r) {
		var orders []store.Order
		if err := query.Limit(200).Find(&orders).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: orders})
		return
	}
	p := common.ParsePagination(r, 20, 100)
	var total int64
	countQuery.Count(&total)
	var orders []store.Order
	if err := query.Offset(p.Offset).Limit(p.PerPage).Find(&orders).Error; err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: common.PageResponse(orders, total, p)})
}

func AdminOrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, action := parseAdminOrderPath(r.URL.Path)
	if id <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid id"})
		return
	}
	switch action {
	case "refund":
		if r.Method != http.MethodPost {
			common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
			return
		}
		refundOrder(w, r, id)
	case "":
		switch r.Method {
		case http.MethodDelete:
			result := store.DB().Delete(&store.Order{}, id)
			if result.Error != nil {
				common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
				return
			}
			if result.RowsAffected == 0 {
				common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "order not found"})
				return
			}
			common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
		default:
			common.WriteJSON(w, http.StatusMethodNotAllowed, common.APIResponse{Code: 405, Msg: "method not allowed"})
		}
	default:
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
	}
}

// refundOrder issues a full refund for a paid order: it asks the payment
// provider to settle the refund, then atomically marks the order refunded and
// reverses any VIP time the original payment had granted. Only orders in the
// "paid" state can be refunded.
func refundOrder(w http.ResponseWriter, r *http.Request, id int) {
	var order store.Order
	if err := store.DB().First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "order not found"})
			return
		}
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	if order.Status == "refunded" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "order already refunded"})
		return
	}
	if order.Status != "paid" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "only paid orders can be refunded"})
		return
	}

	provider, err := providerFor(order.Provider, LoadConfig())
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	result, err := provider.Refund(r.Context(), order)
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}

	if err := store.DB().Transaction(func(tx *gorm.DB) error {
		return applyRefundedOrder(tx, order.ID, result.RefundID, time.Now())
	}); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}

	var updated store.Order
	if err := withOrderProduct(store.DB()).Preload("User").First(&updated, id).Error; err != nil {
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: updated})
}

// applyRefundedOrder marks the order refunded and, for VIP products, claws back
// the granted days from the user's membership (clearing it entirely if that
// pulls the expiry to now or earlier). It re-reads the order under a row lock so
// concurrent refunds are idempotent.
func applyRefundedOrder(tx *gorm.DB, orderID int, refundID string, now time.Time) error {
	var order store.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
		return err
	}
	if order.Status == "refunded" {
		return nil
	}
	if order.Status != "paid" {
		return fmt.Errorf("order cannot be refunded from status %s", order.Status)
	}
	if err := tx.Model(&store.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
		"status":      "refunded",
		"refunded_at": now,
		"refund_id":   strings.TrimSpace(refundID),
		"updated_at":  now,
	}).Error; err != nil {
		return err
	}

	var product store.Product
	if err := tx.Unscoped().First(&product, order.ProductID).Error; err != nil {
		return err
	}
	if product.Kind != "vip" || product.DurationDays <= 0 {
		return nil
	}

	var user store.MobileUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, order.UserID).Error; err != nil {
		return err
	}
	if user.VIPUntil == nil {
		return nil
	}
	newUntil := user.VIPUntil.AddDate(0, 0, -product.DurationDays)
	updates := map[string]interface{}{"updated_at": now}
	if newUntil.After(now) {
		updates["vip_until"] = newUntil
	} else {
		updates["vip_until"] = nil
	}
	return tx.Model(&store.MobileUser{}).Where("id = ?", user.ID).Updates(updates).Error
}

// parseAdminOrderPath splits "/api/admin/orders/{id}[/action]" into its numeric
// id and optional action. id is 0 when the segment is missing or non-numeric.
func parseAdminOrderPath(path string) (int, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/admin/orders/"), "/")
	parts := strings.Split(trimmed, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, ""
	}
	if len(parts) > 1 {
		return id, parts[1]
	}
	return id, ""
}

func StripeWebhookHandler(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "read body failed"})
		return
	}
	if LoadConfig().StripeWebhookKey == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "STRIPE_WEBHOOK_SECRET is not configured"})
		return
	}
	_ = raw
	common.WriteJSON(w, http.StatusNotImplemented, common.APIResponse{Code: 501, Msg: "stripe webhook verification is not implemented yet"})
}

func PayPalWebhookHandler(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "read body failed"})
		return
	}
	if LoadConfig().PayPalWebhookID == "" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "PAYPAL_WEBHOOK_ID is not configured"})
		return
	}
	_ = raw
	common.WriteJSON(w, http.StatusNotImplemented, common.APIResponse{Code: 501, Msg: "paypal webhook verification is not implemented yet"})
}

func saveProduct(w http.ResponseWriter, r *http.Request, id int) {
	var req productPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid body"})
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Code == "" || req.Name == "" || req.PriceCents <= 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "code, name and positive price required"})
		return
	}
	if req.DurationDays < 0 {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "duration_days cannot be negative"})
		return
	}
	if req.Kind == "" {
		req.Kind = "vip"
	}
	if req.Kind != "vip" && req.Kind != "video" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid kind"})
		return
	}
	if req.Currency == "" {
		req.Currency = LoadConfig().DefaultCurrency
	}
	if !supportedProductCurrencies[req.Currency] {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid currency"})
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Status != "active" && req.Status != "inactive" {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "invalid status"})
		return
	}
	if req.Kind == "video" {
		if req.VideoID == nil || *req.VideoID <= 0 {
			common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: "video_id required for video product"})
			return
		}
		if err := store.DB().First(&store.Video{}, *req.VideoID).Error; err != nil {
			common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "video not found"})
			return
		}
	} else {
		req.VideoID = nil
	}
	product := store.Product{
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		Kind:         req.Kind,
		PriceCents:   req.PriceCents,
		Currency:     req.Currency,
		DurationDays: req.DurationDays,
		VideoID:      req.VideoID,
		Status:       req.Status,
		UpdatedAt:    time.Now(),
	}
	if id == 0 {
		product.CreatedAt = time.Now()
		if err := store.DB().Create(&product).Error; err != nil {
			common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
			return
		}
		common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: product})
		return
	}
	result := store.DB().Model(&store.Product{}).Where("id = ?", id).Updates(map[string]interface{}{
		"code":          product.Code,
		"name":          product.Name,
		"description":   product.Description,
		"kind":          product.Kind,
		"price_cents":   product.PriceCents,
		"currency":      product.Currency,
		"duration_days": product.DurationDays,
		"video_id":      product.VideoID,
		"status":        product.Status,
		"updated_at":    product.UpdatedAt,
	})
	if result.Error != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "product not found"})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok"})
}

func showMobileOrder(w http.ResponseWriter, r *http.Request, orderNo string) {
	userID, ok := currentMobileUserID(r)
	if !ok {
		common.WriteJSON(w, http.StatusUnauthorized, common.APIResponse{Code: 401, Msg: "unauthorized"})
		return
	}
	var order store.Order
	err := withOrderProduct(store.DB()).Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	if err != nil {
		common.WriteJSON(w, http.StatusInternalServerError, common.APIResponse{Code: 500, Msg: err.Error()})
		return
	}
	common.WriteJSON(w, http.StatusOK, common.APIResponse{Code: 0, Msg: "ok", Data: order})
}

func completeMockOrder(w http.ResponseWriter, r *http.Request, orderNo string) {
	if !LoadConfig().MockEnabled {
		common.WriteJSON(w, http.StatusNotFound, common.APIResponse{Code: 404, Msg: "not found"})
		return
	}
	if err := markOrderPaid(orderNo, "mock_payment_"+orderNo); err != nil {
		common.WriteJSON(w, http.StatusBadRequest, common.APIResponse{Code: 400, Msg: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<!doctype html><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>Payment Complete</title><body><h1>Payment Complete</h1><p>You can return to the app.</p></body>"))
}

func markOrderPaid(orderNo string, paymentID string) error {
	return store.DB().Transaction(func(tx *gorm.DB) error {
		var order store.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return err
		}
		if order.Status == "paid" {
			return nil
		}
		if order.Status != "paying" && order.Status != "pending" {
			return fmt.Errorf("order cannot be paid from status %s", order.Status)
		}
		var product store.Product
		if err := tx.First(&product, order.ProductID).Error; err != nil {
			return err
		}
		now := time.Now()
		return applyPaidOrder(tx, order, product, paymentID, now)
	})
}

func applyPaidOrder(tx *gorm.DB, order store.Order, product store.Product, paymentID string, paidAt time.Time) error {
	updates := map[string]interface{}{
		"status":     "paid",
		"paid_at":    paidAt,
		"updated_at": time.Now(),
	}
	if strings.TrimSpace(paymentID) != "" {
		updates["provider_payment_id"] = strings.TrimSpace(paymentID)
	}
	if err := tx.Model(&store.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		return err
	}
	if product.Kind != "vip" || product.DurationDays <= 0 {
		return nil
	}

	var user store.MobileUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, order.UserID).Error; err != nil {
		return err
	}
	base := paidAt
	if user.VIPUntil != nil && user.VIPUntil.After(base) {
		base = *user.VIPUntil
	}
	vipUntil := base.AddDate(0, 0, product.DurationDays)
	return tx.Model(&store.MobileUser{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"vip_until":  vipUntil,
		"updated_at": time.Now(),
	}).Error
}

func withOrderProduct(db *gorm.DB) *gorm.DB {
	return db.Preload("Product", func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped()
	})
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

func parseOrderPath(path string) (string, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/orders/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func newOrderNo(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "ORD" + now.Format("20060102150405") + strings.ToUpper(hex.EncodeToString(b[:]))
}
