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

type MockUserServiceLogin struct {
	mocks.UserService
}

func TestAPI_Login(t *testing.T) {
	t.Parallel()

	errInvalidCreds := service.ErrInvalidCredentials

	tests := []struct {
		name      string
		body      string
		mockSetup func(*MockUserServiceLogin)
		wantCode  int
		wantToken string
	}{
		{
			name: "успех_аутентификация_пройдена",
			body: `{"login": "testlogin", "password": "secret123"}`,
			mockSetup: func(m *MockUserServiceLogin) {
				m.On("Login", mock.Anything, "testlogin", "secret123").
					Return("fake-jwt-token", nil)
			},
			wantCode:  http.StatusOK,
			wantToken: "Bearer fake-jwt-token",
		},
		{
			name:      "ошибка_невалидный_json",
			body:      `{invalid json}`,
			mockSetup: func(m *MockUserServiceLogin) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "ошибка_пустой_login",
			body:      `{"login": "", "password": "secret123"}`,
			mockSetup: func(m *MockUserServiceLogin) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "ошибка_пустой_password",
			body:      `{"login": "testlogin", "password": ""}`,
			mockSetup: func(m *MockUserServiceLogin) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name: "ошибка_неверные_учетные_данные",
			body: `{"login": "testlogin", "password": "wrongpass"}`,
			mockSetup: func(m *MockUserServiceLogin) {
				m.On("Login", mock.Anything, "testlogin", "wrongpass").
					Return("", errInvalidCreds)
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "ошибка_внутренняя_ошибка_сервиса",
			body: `{"login": "testlogin", "password": "secret123"}`,
			mockSetup: func(m *MockUserServiceLogin) {
				m.On("Login", mock.Anything, "testlogin", "secret123").
					Return("", errors.New("database connection failed"))
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSvc := new(MockUserServiceLogin)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				userService: mockSvc,
				logger:      slog.Default(),
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			api.Login(rr, req)

			require.Equal(t, tt.wantCode, rr.Code, "неверный HTTP статус код")

			if tt.wantToken != "" {
				assert.Equal(t, tt.wantToken, rr.Header().Get("Authorization"))
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			}

			mockSvc.AssertExpectations(t)
		})
	}
}
