package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
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

func setupOrderService(t *testing.T) *mocks.OrderService {
	return mocks.NewOrderService(t)
}

func TestAPI_RegisterOrder(t *testing.T) {
	t.Parallel()

	const (
		statusNew   = service.OrderStatusAdded
		statusAdded = service.OrderStatusAlreadyAdded
	)

	tests := []struct {
		name      string
		body      string
		claims    jwt.MapClaims
		mockSetup func(*mocks.OrderService)
		wantCode  int
	}{
		{
			name:   "успех_новый_заказ_принят",
			body:   "79927398713",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterOrder", mock.Anything, int64(42), "79927398713").
					Return(statusNew, nil)
			},
			wantCode: http.StatusAccepted,
		},
		{
			name:   "успех_заказ_уже_добавлен",
			body:   "79927398713",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterOrder", mock.Anything, int64(42), "79927398713").
					Return(statusAdded, nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:   "ошибка_конфликт_уже_обрабатывается",
			body:   "79927398713",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterOrder", mock.Anything, int64(42), "79927398713").
					Return(statusNew, service.ErrOrderAlreadyProcessedByOther)
			},
			wantCode: http.StatusConflict,
		},
		{
			name:   "ошибка_сервиса",
			body:   "79927398713",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("RegisterOrder", mock.Anything, int64(42), "79927398713").
					Return(statusNew, errors.New("db error"))
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name:      "ошибка_невалидный_luhn",
			body:      "12345",
			claims:    jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusUnprocessableEntity,
		},
		{
			name:      "ошибка_нет_claims",
			body:      "79927398713",
			claims:    nil,
			mockSetup: func(m *mocks.OrderService) {},
			wantCode:  http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockSvc := setupOrderService(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				orderService: mockSvc,
				logger:       slog.Default(),
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString(tt.body))
			if tt.claims != nil {
				req = req.WithContext(middleware.WithTestClaims(req.Context(), tt.claims))
			}

			rr := httptest.NewRecorder()
			api.RegisterOrder(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)
		})
	}
}

func TestAPI_ListOrders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		claims       jwt.MapClaims
		mockSetup    func(*mocks.OrderService)
		wantCode     int
		wantResponse []OrderResponse
	}{
		{
			name:   "успех_список_заказов",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("OrderList", mock.Anything, int64(42)).Return([]model.Order{
					{
						Number:    "79927398713",
						Status:    model.OrderStatusProcessed,
						Points:    15000,
						CreatedAt: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
					},
				}, nil)
			},
			wantCode: http.StatusOK,
			wantResponse: []OrderResponse{
				{
					Number:    "79927398713",
					Status:    model.OrderStatusProcessed,
					Points:    150.0,
					CreatedAt: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name:   "успех_пустой_список",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("OrderList", mock.Anything, int64(42)).Return([]model.Order{}, nil)
			},
			wantCode:     http.StatusNoContent,
			wantResponse: nil,
		},
		{
			name:   "ошибка_сервиса",
			claims: jwt.MapClaims{"userID": float64(42)},
			mockSetup: func(m *mocks.OrderService) {
				m.On("OrderList", mock.Anything, int64(42)).Return([]model.Order(nil), errors.New("db error"))
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

			mockSvc := setupOrderService(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSvc)
			}

			api := &API{
				orderService: mockSvc,
				logger:       slog.Default(),
			}

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tt.claims != nil {
				req = req.WithContext(middleware.WithTestClaims(req.Context(), tt.claims))
			}

			rr := httptest.NewRecorder()
			api.ListOrders(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)

			if tt.wantResponse != nil {
				var got []OrderResponse
				require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
				assert.Equal(t, tt.wantResponse, got)
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			}
		})
	}
}

func Test_luhnValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orderNumber string
		want        bool
	}{
		{
			name:        "успех_валидный_номер",
			orderNumber: "79927398713",
			want:        true,
		},
		{
			name:        "успех_все_нули",
			orderNumber: "000000000000",
			want:        true,
		},
		{
			name:        "ошибка_пустая_строка",
			orderNumber: "",
			want:        false,
		},
		{
			name:        "ошибка_содержит_буквы",
			orderNumber: "7992739871a",
			want:        false,
		},
		{
			name:        "ошибка_содержит_пробел",
			orderNumber: "7992739871 ",
			want:        false,
		},
		{
			name:        "ошибка_невалидная_контрольная_сумма",
			orderNumber: "79927398714", // Последняя цифра изменена
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := luhnValid(tt.orderNumber)
			assert.Equal(t, tt.want, got)
		})
	}
}
