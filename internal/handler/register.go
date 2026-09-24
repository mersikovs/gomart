package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mersikovs/gomart/internal/service"
)

// RegisterRequest определяет структуру тела запроса для регистрации нового пользователя.
// Используется для декодирования JSON из HTTP-запроса в методе Api.Register.
type RegisterRequest struct {
	// Login — уникальный идентификатор пользователя (логин).
	// Обычно это строка, содержащая уникальное имя.
	Login string `json:"login"`

	// Password — пароль пользователя в открытом виде. В базе хешируется
	Password string `json:"password"`
}

// Register — HTTP-хендлер, обрабатывающий регистрацию новых пользователей.
func (h *API) Register(w http.ResponseWriter, r *http.Request) {
	var registerVars RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&registerVars); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if registerVars.Login == "" {
		http.Error(w, "login is required", http.StatusBadRequest)
		return
	}

	if len(registerVars.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Register(r.Context(), registerVars.Login, registerVars.Password)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			http.Error(w, "user already exists", http.StatusConflict) // 409
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}
