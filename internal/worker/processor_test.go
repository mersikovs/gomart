package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/mersikovs/gomart/internal/mocks"
	"github.com/mersikovs/gomart/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockStorage struct {
	mocks.Storage
}

type MockAccrualClient struct {
	mocks.AccrualClient
}

func TestOrderProcessor_processOrder(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		mockSetup func(*MockStorage, *MockAccrualClient)
		// Named input parameters for target function.
		order         *model.Order
		wantErr       bool
		wantErrString string
	}{
		{
			name: "успех_обновление_статуса_и_баланса",
			order: &model.Order{
				Number: "12345",
				UserID: 1,
				Status: model.OrderStatusNew,
			},
			mockSetup: func(repo *MockStorage, client *MockAccrualClient) {
				client.On("GetOrder", mock.Anything, "12345").Return(&model.Order{
					UserID: 1,
					Status: model.OrderStatusProcessed,
					Points: 100,
				}, nil)
				repo.On("UpdateOrderStatusAndUserBalance", mock.Anything, int64(1), "12345", model.OrderStatusProcessed, 100).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "ошибка_клиента_начислений",
			order: &model.Order{
				Number: "999",
				UserID: 2,
				Status: model.OrderStatusNew,
			},
			mockSetup: func(repo *MockStorage, client *MockAccrualClient) {
				client.On("GetOrder", mock.Anything, "999").Return(&model.Order{}, errors.New("network error"))
			},
			wantErr:       true,
			wantErrString: "network error",
		},
		{
			name: "ошибка_невалидный_переход_статуса",
			order: &model.Order{
				Number: "777",
				UserID: 3,
				Status: model.OrderStatusProcessed, // Уже финальный статус
			},
			mockSetup: func(repo *MockStorage, client *MockAccrualClient) {
				client.On("GetOrder", mock.Anything, "777").Return(&model.Order{
					Status: model.OrderStatusNew,
					Points: 0,
				}, nil)
				// UpdateOrderStatusAndUserBalance не должен быть вызван
			},
			wantErr:       true,
			wantErrString: "invalid status transition",
		},
		{
			name: "ошибка_обновления_в_бд",
			order: &model.Order{
				Number: "555",
				UserID: 4,
				Status: model.OrderStatusNew,
			},
			mockSetup: func(repo *MockStorage, client *MockAccrualClient) {
				client.On("GetOrder", mock.Anything, "555").Return(&model.Order{
					Status: model.OrderStatusProcessed,
					Points: 50.0,
				}, nil)
				repo.On("UpdateOrderStatusAndUserBalance", mock.Anything, int64(4), "555", model.OrderStatusProcessed, 50).Return(errors.New("db constraint"))
			},
			wantErr:       true,
			wantErrString: "db constraint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo := new(MockStorage)
			mockClient := new(MockAccrualClient)

			if tt.mockSetup != nil {
				tt.mockSetup(mockRepo, mockClient)
			}

			p := NewOrderProcessor(mockRepo, mockClient, &Pool{}, 1*time.Second)

			gotErr := p.processOrder(t.Context(), *tt.order)

			if tt.wantErr {
				require.Error(t, gotErr)
				assert.Contains(t, gotErr.Error(), tt.wantErrString)
				return
			}

			require.NoError(t, gotErr)
			mockRepo.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}
