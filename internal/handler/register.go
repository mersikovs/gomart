package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mersikovs/gomart/internal/service"
)

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Api) Register(w http.ResponseWriter, r *http.Request) {
	var registerVars RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&registerVars); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if registerVars.Login == "" {
		http.Error(w, "Некоректные данные login", http.StatusBadRequest)
		return
	}

	if len(registerVars.Password) < 8 {
		http.Error(w, "Некоректные данные password", http.StatusBadRequest)
		return
	}

	token, err := h.userService.Register(r.Context(), registerVars.Login, registerVars.Password)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			http.Error(w, "Пользователь уже существует", http.StatusConflict) // 409
			return
		}
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(token)
}
