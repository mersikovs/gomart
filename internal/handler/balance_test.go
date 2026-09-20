package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/magiconair/properties/assert"
	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/mocks"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockUserService struct {
	mocks.UserService
}

func TestAPI_GetBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		string
		claims       jwt.MapClaims
		mockSetup    func(*MockUserService)
		wantCode     int
		wantResponse *BalanceResponse
	}{
		{
			name: "успех_корректный_баланс",
			claims: jwt.MapClaims{
				"userID": float64(42),
			},
			mockSetup: func(m *MockUserService) {
				m.On("GetBalance", mock.Anything, int64(42)).Return(&model.User{
					Balance:    15000, // В БД хранится в копейках/центах
					TotalSpent: 5000,
				}, nil)
			},
			wantCode: http.StatusOK,
			wantResponse: &BalanceResponse{
				CurrentBalance: 150.0, // Хендлер делит на 100
				TotalSpent:     50.0,
			},
		},
		{
			name:      "ошибка_нет_claims_в_контексте",
			claims:    nil,
			mockSetup: func(m *MockUserService) {},
			wantCode:  http.StatusUnauthorized,
		},
		{
			name: "ошибка_неверный_тип_userID",
			claims: jwt.MapClaims{
				"userID": "строка_вместо_числа",
			},
			mockSetup: func(m *MockUserService) {},
			wantCode:  http.StatusUnauthorized,
		},
		{
			name: "ошибка_сервиса",
			claims: jwt.MapClaims{
				"userID": float64(99),
			},
			mockSetup: func(m *MockUserService) {
				m.On("GetBalance", mock.Anything, int64(99)).Return(nil, errors.New("service error"))
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockService := new(MockUserService)
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			api := &API{
				userService: mockService,
				logger:      slog.Default(),
			}

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)

			if tt.claims != nil {
				req = req.WithContext(middleware.WithTestClaims(req.Context(), tt.claims))
			}

			rr := httptest.NewRecorder()

			api.GetBalance(rr, req)

			require.Equal(t, tt.wantCode, rr.Code, "неверный HTTP статус код")

			if tt.wantResponse != nil {
				var got BalanceResponse
				err := json.NewDecoder(rr.Body).Decode(&got)
				require.NoError(t, err, "ошибка декодирования JSON ответа")

				assert.Equal(t, tt.wantResponse.CurrentBalance, got.CurrentBalance)
				assert.Equal(t, tt.wantResponse.TotalSpent, got.TotalSpent)
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			}

			mockService.AssertExpectations(t)
		})
	}
}
