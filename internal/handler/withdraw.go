package handler

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"

	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/service"
)

type RegisterWithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func (h *Api) RegisterWithdraw(w http.ResponseWriter, r *http.Request) {

	var withdrawVars RegisterWithdrawRequest

	if err := json.NewDecoder(r.Body).Decode(&withdrawVars); err != nil {
		h.logger.Debug("registerWithdrawRequest format", "error", err)
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
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

	if !luhnValid(string(withdrawVars.Order)) {
		http.Error(w, "неверный формат номера заказа", http.StatusUnprocessableEntity)
		return
	}

	sum, err := getValidateSum(withdrawVars.Sum)
	if err != nil {
		http.Error(w, "неверный формат суммы списания", http.StatusBadRequest)
		return
	}

	kopecks := int(math.Round(sum * 100))

	orderStatus, err := h.orderService.RegisterWithdraw(r.Context(), int64(userID), withdrawVars.Order, kopecks)
	if err != nil {
		if errors.Is(err, service.ErrWithdrawInsufficientFunds) {
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}
		h.logger.Debug("registerWithdraw", "error", err)
		http.Error(w, "RegisterWithdraw error", http.StatusInternalServerError)
		return
	}

	if orderStatus == service.OrderStatusAlreadyAdded {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(orderStatus)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(orderStatus)
}

func (h *Api) ListWithdraws(w http.ResponseWriter, r *http.Request) {
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

	list, err := h.orderService.WithdrawList(r.Context(), int64(userID))
	if err != nil {
		h.logger.Debug("error OrderList", "error", err)
		http.Error(w, "error OrderList", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(list)
}

func getValidateSum(val float64) (float64, error) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0, errors.New("сумма не может быть NaN или Infinity")
	}

	if val <= 0 {
		return 0, errors.New("сумма должна быть больше нуля")
	}

	multiplied := val * 100
	if math.Abs(math.Round(multiplied)-multiplied) > 1e-9 {
		return 0, errors.New("допустимо не более 2 знаков после запятой")
	}

	return val, nil
}
