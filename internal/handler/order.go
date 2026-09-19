package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
	"unicode"

	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/service"
)

func (h *Api) RegisterOrder(w http.ResponseWriter, r *http.Request) {

	orderNumber, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read order number", http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userId"].(float64)
	if !ok {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}

	if !luhnValid(string(orderNumber)) {
		http.Error(w, "неверный формат номера заказа", http.StatusUnprocessableEntity)
		return
	}

	orderStatus, err := h.orderService.RegisterOrder(r.Context(), int64(userID), string(orderNumber))
	if err != nil {
		if errors.Is(err, service.ErrOrderAlreadyProcessedByOther) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		h.logger.Debug("registerOrder", "error", err)
		http.Error(w, "RegisterOrder error", http.StatusInternalServerError)
		return
	}

	if orderStatus == service.OrderStatusAlreadyAdded {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(string(orderNumber))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(string(orderNumber))
}

type OrderResponse struct {
	Number    string            `json:"number"`
	Status    model.OrderStatus `json:"status"`
	Points    float64           `json:"accrual,omitempty"`
	CreatedAt time.Time         `json:"uploaded_at"`
}

func (h *Api) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userId"].(float64)
	if !ok {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
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
			CreatedAt: o.ChangedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(listOrdersDTO)
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

	for i := len(runes) - 1; i >= 0; i-- {
		digit := int(runes[i] - '0')
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
