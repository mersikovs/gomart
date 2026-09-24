package handler

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/service"
)

// RegisterWithdrawRequest определяет структуру тела запроса для списания бонусных баллов.
// Валидируется при декодировании JSON из HTTP-запроса.
type RegisterWithdrawRequest struct {
	// Order — номер заказа который отправляет пользователь за получения скидки.
	Order string `json:"order"`

	// Sum — количество бонусных баллов к списанию.
	Sum float64 `json:"sum"`
}

// WithdrawResponse описывает успешный ответ API на создание заявки на списание.
type WithdrawResponse struct {
	// Number — номер заказа, по которому было произведено списание.
	Number string `json:"order"`

	// Points — сумма фактически списанных баллов. Тег omitempty позволяет скрыть поле,
	// если оно равно нулю (например, при ошибке или частичном возврате).
	Points float64 `json:"sum,omitempty"`

	// CreatedAt — временная метка UTC, когда транзакция была окончательно зарегистрированна системой.
	CreatedAt time.Time `json:"processed_at"`
}

// RegisterWithdraw — HTTP-хендлер, создающий заявку на списание бонусов.
// Ожидает тело в формате RegisterWithdrawRequest.
// Извлекает ID авторизованного пользователя из контекста (установленного middleware Auth).
func (h *API) RegisterWithdraw(w http.ResponseWriter, r *http.Request) {

	var withdrawVars RegisterWithdrawRequest

	if err := json.NewDecoder(r.Body).Decode(&withdrawVars); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userID"].(float64)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if !luhnValid(string(withdrawVars.Order)) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	sum, err := getValidateSum(withdrawVars.Sum)
	if err != nil {
		http.Error(w, "invalid withdrawal amount", http.StatusBadRequest)
		return
	}

	kopecks := int(math.Round(sum * 100))

	orderStatus, err := h.orderService.RegisterWithdraw(r.Context(), int64(userID), withdrawVars.Order, kopecks)
	if err != nil {
		if errors.Is(err, service.ErrWithdrawInsufficientFunds) {
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}
		h.logger.Error("registerWithdraw", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if orderStatus == service.OrderStatusAlreadyAdded {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// ListWithdraws — HTTP-хендлер, возвращающий историю списаний бонусных баллов
// для авторизованного пользователя.
// Извлекает ID пользователя из контекста запроса middleware Auth и запрашивает данные через OrderService.
func (h *API) ListWithdraws(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	userID, ok := claims["userID"].(float64)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	list, err := h.orderService.WithdrawList(r.Context(), int64(userID))
	if err != nil {
		h.logger.Error("error WithdrawList", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(list) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	listWithdrawDTO := make([]WithdrawResponse, 0)
	for _, o := range list {
		listWithdrawDTO = append(listWithdrawDTO, WithdrawResponse{
			Number:    o.Number,
			Points:    float64(o.Points) / 100,
			CreatedAt: o.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listWithdrawDTO); err != nil {
		h.logger.Error("failed to encode withdrawals response listWithdrawDTO", "error", err, "user_id", userID)
	}
}

func getValidateSum(val float64) (float64, error) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0, errors.New("amount must be finite")
	}

	if val <= 0 {
		return 0, errors.New("amount must be positive")
	}

	multiplied := val * 100
	if math.Abs(math.Round(multiplied)-multiplied) > 1e-9 {
		return 0, errors.New("amount has more than 2 decimal places")
	}

	return val, nil
}
