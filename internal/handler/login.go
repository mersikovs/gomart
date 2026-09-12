package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mersikovs/gomart/internal/service"
)

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Api) Login(w http.ResponseWriter, r *http.Request) {
	var loginVars LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&loginVars); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if loginVars.Login == "" {
		http.Error(w, "Некоректные данные login", http.StatusBadRequest)
		return
	}

	if loginVars.Password == "" {
		http.Error(w, "Некоректные данные password", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Login(r.Context(), loginVars.Login, loginVars.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			http.Error(w, "Неверная пара логин/пароль", http.StatusUnauthorized) // 401
			return
		}
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(token)
}
