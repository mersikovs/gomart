package model

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestOrderStatusCanTransitionTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current OrderStatus
		next    OrderStatus
		want    bool
	}{
		{
			name:    "успех_разрешенный_пеерход",
			current: OrderStatusNew,
			next:    OrderStatusProcessed,
			want:    true,
		},
		{
			name:    "ошибка_переход_из_финального_статуса_processed",
			current: OrderStatusProcessed,
			next:    OrderStatusNew,
			want:    false,
		},
		{
			name:    "ошибка_переход_из_финального_статуса_invalid",
			current: OrderStatusInvalid,
			next:    OrderStatusNew,
			want:    false,
		},
		{
			name:    "ошибка_переход_из_финального_статуса_processed#2",
			current: OrderStatusProcessed,
			next:    OrderStatusProcessing,
			want:    false,
		},
		{
			name:    "ошибка_переход_из_финального_статуса_invalid#2",
			current: OrderStatusInvalid,
			next:    OrderStatusProcessing,
			want:    false,
		},
		{
			name:    "ошибка_запрещенный_переход",
			current: OrderStatusProcessing,
			next:    OrderStatusNew,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.current.CanTransitionTo(tt.next)
			assert.Equal(t, tt.want, got)
		})
	}
}
