package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/magiconair/properties/assert"
	"github.com/mersikovs/gomart/internal/middleware"
	"github.com/mersikovs/gomart/internal/mocks"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupOrderServiceMock(t *testing.T) *mocks.OrderService {
	return mocks.NewOrderService(t)
}

func TestAPI_RegisterWithdraw(t *testing.T) {
	t.Parallel()

	errInsufficientFunds := service.ErrWithdrawInsufficientFunds

	tests := []struct {
		name      string
		body      string
		claims    jwt.MapClaims
		mockSetup func(*mocks.OrderService)
		wantCode  int
	}{
		{
			name:   "успех_новая_заявка_на_списание",
			body:   `{"order": "79927398713", "sum": 10.50}`,
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterWithdraw", mock.Anything, int64(42), "79927398713", 1050).
					Return(service.OrderStatusAdded, nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:   "успех_заявка_уже_существовала",
			body:   `{"order": "79927398713", "sum": 10.00}`,
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterWithdraw", mock.Anything, int64(42), "79927398713", 1000).
					Return(service.OrderStatusAlreadyAdded, nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:   "ошибка_недостаточно_средств",
			body:   `{"order": "79927398713", "sum": 100.00}`,
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterWithdraw", mock.Anything, int64(42), "79927398713", 10000).
					Return(service.OrderStatusAdded, errInsufficientFunds)
			},
			wantCode: http.StatusPaymentRequired,
		},
		{
			name:   "ошибка_сервиса",
			body:   `{"order": "79927398713", "sum": 10.00}`,
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterWithdraw", mock.Anything, int64(42), "79927398713", 1000).
					Return(service.OrderStatusAdded, errors.New("db error"))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name:      "ошибка_невалидный_json",
			body:      `{bad json}`,
			claims:    jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "ошибка_нет_claims",
			body:      `{"order": "79927398713", "sum": 10.00}`,
			claims:    nil,
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusUnauthorized,
		},
		{
			name:      "ошибка_невалидный_luhn",
			body:      `{"order": "123", "sum": 10.00}`,
			claims:    jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusUnprocessableEntity,
		},
		{
			name:      "ошибка_невалидная_сумма_отрицательная",
			body:      `{"order": "79927398713", "sum": -5.00}`,
			claims:    jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSvc := setupOrderServiceMock(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				orderService: mockSvc,
				logger:       slog.Default(),
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.claims != nil {
				req = req.WithContext(middleware.WithTestClaims(req.Context(), tt.claims))
			}

			rr := httptest.NewRecorder()
			api.RegisterWithdraw(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)
		})
	}
}

func TestAPI_ListWithdraws(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		claims       jwt.MapClaims
		mockSetup    func(*mocks.OrderService)
		wantCode     int
		wantResponse []WithdrawResponse
	}{
		{
			name:   "успех_список_списаний",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("WithdrawList", mock.Anything, int64(42)).Return([]model.Order{
					{
						Number:    "79927398713",
						Points:    15000,
						CreatedAt: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
					},
				}, nil)
			},
			wantCode: http.StatusOK,
			wantResponse: []WithdrawResponse{
				{
					Number:    "79927398713",
					Points:    150.0,
					CreatedAt: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name:   "успех_пустой_список",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("WithdrawList", mock.Anything, int64(42)).Return([]model.Order{}, nil)
			},
			wantCode:     http.StatusNoContent,
			wantResponse: nil,
		},
		{
			name:   "ошибка_сервиса",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("WithdrawList", mock.Anything, int64(42)).Return([]model.Order(nil), errors.New("db error"))
			},
			wantCode:     http.StatusInternalServerError,
			wantResponse: nil,
		},
		{
			name:         "ошибка_нет_claims",
			claims:       nil,
			mockSetup:    func(m *mocks.OrderService) {},
			wantCode:     http.StatusUnauthorized,
			wantResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSvc := setupOrderServiceMock(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				orderService: mockSvc,
				logger:       slog.Default(),
			}

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance/withdraw", nil)
			if tt.claims != nil {
				req = req.WithContext(middleware.WithTestClaims(req.Context(), tt.claims))
			}

			rr := httptest.NewRecorder()
			api.ListWithdraws(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)

			if tt.wantResponse != nil {
				var got []WithdrawResponse
				require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
				assert.Equal(t, tt.wantResponse, got)
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			}
		})
	}
}

func TestGetValidateSum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		val     float64
		want    float64
		wantErr bool
	}{
		{
			name:    "успех_целое_число",
			val:     10.0,
			want:    10.0,
			wantErr: false,
		},
		{
			name:    "успех_два_знака",
			val:     10.55,
			want:    10.55,
			wantErr: false,
		},
		{
			name:    "ошибка_ноль",
			val:     0.0,
			want:    0,
			wantErr: true,
		},
		{
			name:    "ошибка_отрицательное",
			val:     -5.0,
			want:    0,
			wantErr: true,
		},
		{
			name:    "ошибка_nan",
			val:     math.NaN(),
			want:    0,
			wantErr: true,
		},
		{
			name:    "ошибка_inf",
			val:     math.Inf(1),
			want:    0,
			wantErr: true,
		},
		{
			name:    "ошибка_три_знака",
			val:     10.555,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt // фиксация переменной цикла для t.Parallel()
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := getValidateSum(tt.val)

			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
