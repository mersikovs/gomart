package model

import "time"

type ActionType string

const (
	ActionEarn  ActionType = "earn"  // Начисление
	ActionSpend ActionType = "spend" // Списание
)

type Order struct {
	ID        int64      `db:"id"`
	UserID    int64      `db:"user_id"`
	Number    string     `db:"number"`
	Status    string     `db:"status"`
	Action    ActionType `db:"action"`
	Points    int        `db:"points"`
	ChangedAt time.Time  `db:"created_at"`
}
