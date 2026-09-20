package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mersikovs/gomart/internal/service"
)

// LoginRequest определяет структуру тела запроса для аутентификации пользователя.
type LoginRequest struct {
	// Login — уникальный идентификатор пользователя (логин или email).
	Login string `json:"login"`

	// Password — пароль в открытом виде.
	Password string `json:"password"`
}

// Login — HTTP-хендлер, выполняющий аутентификацию пользователя.
func (h *API) Login(w http.ResponseWriter, r *http.Request) {
	var loginVars LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&loginVars); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if loginVars.Login == "" {
		http.Error(w, "login is required", http.StatusBadRequest)
		return
	}

	if loginVars.Password == "" {
		http.Error(w, "invalid password", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Login(r.Context(), loginVars.Login, loginVars.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized) // 401
			return
		}
		h.logger.Debug("login failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}
