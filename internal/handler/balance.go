package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mersikovs/gomart/internal/middleware"
)

// BalanceResponse определяет структуру JSON-ответа для эндпоинта баланса пользователя.
type BalanceResponse struct {
	// CurrentBalance — текущее количество доступных бонусных баллов на счету пользователя.
	CurrentBalance float64 `json:"current"`

	// TotalSpent — общая сумма бонусов, когда-либо списанных пользователем за все время.
	TotalSpent float64 `json:"withdrawn"`
}

// GetBalance — HTTP-хендлер, возвращающий актуальное состояние счета.
func (h *API) GetBalance(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.userService.GetBalance(r.Context(), int64(userID))
	if err != nil {
		h.logger.Error("error GetBalance", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	balance := &BalanceResponse{
		CurrentBalance: float64(user.Balance) / 100,
		TotalSpent:     float64(user.TotalSpent) / 100,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(balance); err != nil {
		h.logger.Error("failed to encode balance response", "error", err, "user_id", userID)
	}
}
