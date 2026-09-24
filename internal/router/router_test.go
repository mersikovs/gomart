package router

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/magiconair/properties/assert"
	"github.com/mersikovs/gomart/internal/handler"
	"github.com/mersikovs/gomart/internal/mocks"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/stretchr/testify/mock"
)

func generateTestToken(secret string) string {
	claims := jwt.MapClaims{
		"userID": float64(42),
		"exp":    time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString([]byte(secret))
	return signedToken
}

func TestSetup_Routes(t *testing.T) {
	t.Parallel()

	mockRepo := mocks.NewStorage(t)
	mockClient := mocks.NewAccrualClient(t)

	mockRepo.
		On("GetOrdersByUser", mock.Anything, int64(42), model.ActionEarn).
		Return([]model.Order{ /* ... */ }, nil).
		Once()

	mockRepo.
		On("FindUserByID", mock.Anything, int64(42)).
		Return(&model.User{ /* ... */ }, nil).
		Once()

	jwtSecret := "test-secret-key-123"
	logger := slog.Default()

	api := handler.New(mockRepo, mockClient, jwtSecret, 10, logger)

	router := Setup(api, slog.Default())

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		wantStatus int
	}{
		// --- Публичные маршруты (без Auth) ---
		{
			name:       "успех_регистрация_маршрут_существует",
			method:     http.MethodPost,
			path:       "/api/user/register",
			token:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "успех_логин_маршрут_существует",
			method:     http.MethodPost,
			path:       "/api/user/login",
			token:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ошибка_метод_not_allowed_для_регистрации",
			method:     http.MethodGet,
			path:       "/api/user/register",
			token:      "",
			wantStatus: http.StatusMethodNotAllowed,
		},

		// --- Защищенные маршруты (требуют Auth) ---
		{
			name:       "успех_баланс_с_валидным_токеном",
			method:     http.MethodGet,
			path:       "/api/user/balance",
			token:      generateTestToken(api.JWTSecret),
			wantStatus: http.StatusOK,
		},
		{
			name:       "ошибка_баланс_без_токена",
			method:     http.MethodGet,
			path:       "/api/user/balance",
			token:      "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "ошибка_метод_not_allowed_для_баланса",
			method:     http.MethodPost,
			path:       "/api/user/balance",
			token:      generateTestToken(api.JWTSecret),
			wantStatus: http.StatusMethodNotAllowed, // 405
		},
		{
			name:       "успех_список_заказов_с_токеном",
			method:     http.MethodGet,
			path:       "/api/user/orders",
			token:      generateTestToken(api.JWTSecret),
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "успех_неизвестный_маршрут",
			method:     http.MethodGet,
			path:       "/api/unknown/route",
			token:      "",
			wantStatus: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "неверный HTTP статус для маршрута %s %s", tt.method, tt.path)
		})
	}
}
