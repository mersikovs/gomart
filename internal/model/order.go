package model

import "time"

type ActionType string

const (
	ActionEarn  ActionType = "earn"  // Начисление
	ActionSpend ActionType = "spend" // Списание
)

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
	OrderStatusInvalid    OrderStatus = "INVALID"
)

func (s OrderStatus) IsFinal() bool {
	return s == OrderStatusProcessed || s == OrderStatusInvalid
}

func (s OrderStatus) CanTransitionTo(newStatus OrderStatus) bool {
	if s.IsFinal() {
		return false
	}

	return true
}

type Order struct {
	ID        int64       `db:"id"`
	UserID    int64       `db:"user_id"`
	Number    string      `db:"number"`
	Status    OrderStatus `db:"status"`
	Action    ActionType  `db:"action"`
	Points    int         `db:"points"`
	ChangedAt time.Time   `db:"created_at"`
}
