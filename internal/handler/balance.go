package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mersikovs/gomart/internal/middleware"
)

func (h *Api) GetBalance(w http.ResponseWriter, r *http.Request) {
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

	balance, err := h.userService.GetBalance(r.Context(), int64(userID))
	if err != nil {
		h.logger.Debug("error GetBalance", "error", err)
		http.Error(w, "error GetBalance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(balance)
}
