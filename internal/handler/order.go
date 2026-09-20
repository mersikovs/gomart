package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"time"
	"unicode"

	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/service"
)

// RegisterOrder — HTTP-хендлер, регистрирующий новый номер заказа для текущего пользователя.
// Ожидает тело запроса в виде простого текста (plain/text) с номером заказа.
func (h *API) RegisterOrder(w http.ResponseWriter, r *http.Request) {

	orderNumber, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read order number", http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userID"].(float64)
	if !ok {
		http.Error(w, "invalid token claims", http.StatusUnauthorized)
		return
	}

	if !luhnValid(string(orderNumber)) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	orderStatus, err := h.orderService.RegisterOrder(r.Context(), int64(userID), string(orderNumber))
	if err != nil {
		if errors.Is(err, service.ErrOrderAlreadyProcessedByOther) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		h.logger.Debug("registerOrder error", "error", err)
		http.Error(w, "registerOrder error", http.StatusInternalServerError)
		return
	}

	if orderStatus == service.OrderStatusAlreadyAdded {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}

// OrderResponse определяет структуру ответа API при регистрации нового заказа.
// Содержит номер заказа.
type OrderResponse struct {
	// Number — уникальный номер заказа.
	Number string `json:"number"`

	// Status — текущее состояние обработки заказа сервером лояльности
	Status model.OrderStatus `json:"status"`

	// Points — количество начисленных бонусных баллов за заказ.
	Points float64 `json:"accrual,omitempty"`

	// CreatedAt — время UTC, когда номер заказа был загружен пользователем в систему.
	CreatedAt time.Time `json:"uploaded_at"`
}

// ListOrders — HTTP-хендлер, возвращающий список всех заказов авторизованного пользователя.
func (h *API) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userID"].(float64)
	if !ok {
		http.Error(w, "invalid token claims", http.StatusUnauthorized)
		return
	}

	list, err := h.orderService.OrderList(r.Context(), int64(userID))
	if err != nil {
		h.logger.Debug("error OrderList", "error", err)
		http.Error(w, "error OrderList", http.StatusInternalServerError)
		return
	}

	if len(list) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	listOrdersDTO := make([]OrderResponse, 0)
	for _, o := range list {
		listOrdersDTO = append(listOrdersDTO, OrderResponse{
			Number:    o.Number,
			Status:    o.Status,
			Points:    float64(o.Points) / 100,
			CreatedAt: o.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listOrdersDTO); err != nil {
		h.logger.Error("failed to encode orders response listOrdersDTO", "error", err, "user_id", userID)
	}
}

func luhnValid(orderNumber string) bool {
	for _, r := range orderNumber {
		if !unicode.IsDigit(r) {
			return false
		}
	}

	if len(orderNumber) == 0 {
		return false
	}

	total := 0
	isSecondDigit := false

	runes := []rune(orderNumber)

	for _, rune := range slices.Backward(runes) {
		digit := int(rune - '0')
		if isSecondDigit {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		total += digit
		isSecondDigit = !isSecondDigit
	}
	return total%10 == 0
}
