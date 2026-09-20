package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/mersikovs/gomart/internal/mocks"
	"github.com/mersikovs/gomart/internal/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockUserServiceRegister struct {
	mocks.UserService
}

func TestAPI_Register(t *testing.T) {
	t.Parallel()

	errUserExists := service.ErrUserAlreadyExists

	tests := []struct {
		name      string
		body      string
		mockSetup func(*MockUserServiceRegister)
		wantCode  int
		wantToken string
	}{
		{
			name: "успех_регистрация_пройдена",
			body: `{"login": "newuser", "password": "securepass123"}`, // 13 символов
			mockSetup: func(m *MockUserServiceRegister) {
				m.On("Register", mock.Anything, "newuser", "securepass123").
					Return("new-fake-jwt-token", nil)
			},
			wantCode:  http.StatusOK,
			wantToken: "Bearer new-fake-jwt-token",
		},
		{
			name:      "ошибка_невалидный_json",
			body:      `{invalid json}`,
			mockSetup: func(m *MockUserServiceRegister) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "ошибка_пустой_login",
			body:      `{"login": "", "password": "securepass123"}`,
			mockSetup: func(m *MockUserServiceRegister) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "ошибка_короткий_пароль_7_символов",
			body:      `{"login": "newuser", "password": "short"}`, // 5 символов
			mockSetup: func(m *MockUserServiceRegister) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name: "ошибка_пользователь_уже_существует",
			body: `{"login": "existinguser", "password": "securepass123"}`,
			mockSetup: func(m *MockUserServiceRegister) {
				m.On("Register", mock.Anything, "existinguser", "securepass123").
					Return("", errUserExists)
			},
			wantCode: http.StatusConflict,
		},
		{
			name: "ошибка_внутренняя_ошибка_сервиса",
			body: `{"login": "newuser", "password": "securepass123"}`,
			mockSetup: func(m *MockUserServiceRegister) {
				m.On("Register", mock.Anything, "newuser", "securepass123").
					Return("", errors.New("database connection failed"))
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSvc := new(MockUserServiceRegister)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				userService: mockSvc,
				logger:      slog.Default(),
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			api.Register(rr, req)

			require.Equal(t, tt.wantCode, rr.Code, "неверный HTTP статус код")

			if tt.wantToken != "" {
				assert.Equal(t, tt.wantToken, rr.Header().Get("Authorization"))
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			}

			mockSvc.AssertExpectations(t)
		})
	}
}
