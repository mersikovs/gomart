package model

import (
	"slices"
	"time"
)

// ActionType определяет тип финансовой операции для фильтрации и агрегации данных.
type ActionType string

const (
	// ActionEarn обозначает номер заказа пришел и записан в режиме начисления
	ActionEarn ActionType = "earn"

	// ActionSpend обозначает номер заказа пришел и записан в режиме списания
	ActionSpend ActionType = "spend"
)

// OrderStatus представляет собой жизненный цикл заказа в системе начислений.
type OrderStatus string

const (
	// OrderStatusNew заказ загружен в систему, но не попал в обработку.
	OrderStatusNew OrderStatus = "NEW"

	// OrderStatusProcessing вознаграждение за заказ рассчитывается.
	OrderStatusProcessing OrderStatus = "PROCESSING"

	// OrderStatusProcessed данные по заказу проверены и информация о расчёте успешно получена.
	OrderStatusProcessed OrderStatus = "PROCESSED"

	// OrderStatusInvalid система расчёта вознаграждений отказала в расчёте.
	OrderStatusInvalid OrderStatus = "INVALID"
)

var allowedTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusNew:        {OrderStatusProcessing, OrderStatusProcessed, OrderStatusInvalid},
	OrderStatusProcessing: {OrderStatusProcessed, OrderStatusInvalid},
}

func (s OrderStatus) isFinal() bool {
	return s == OrderStatusProcessed || s == OrderStatusInvalid
}

// CanTransitionTo проверяет допустимость перехода заказа из текущего статуса (s) в новый статус newStatus.
// Возвращает true, если переход разрешен бизнес-логикой.
func (s OrderStatus) CanTransitionTo(newStatus OrderStatus) bool {
	if s.isFinal() {
		return false
	}
	return slices.Contains(allowedTransitions[s], newStatus)
}

// Order представляет собой доменную сущность заказа в системе лояльности.
// Агрегирует данные о заказе, его текущем состоянии и начисленных бонусах или списаниях в зависимости от Action.
type Order struct {
	// ID — внутренний первичный ключ записи в базе данных.
	ID int64 `db:"id"`

	// UserID — идентификатор пользователя, которому принадлежит данный заказ.
	UserID int64 `db:"user_id"`

	// Number — уникальный номер заказа, в том числе во внешней системе
	Number string `db:"number"`

	// Status — текущий статус заказа в системе.
	Status OrderStatus `db:"status"`

	// Action — тип операции, которую запросил пользователь:
	// начисление баллов пользователю или списание их в счет оплаты.
	Action ActionType `db:"action"`

	// Points — количество бонусных баллов. При ActionEarn это сумма к зачислению,
	// при ActionSpend — сумма к списанию со счета пользователя.
	Points int `db:"points"`

	// CreatedAt — временная метка последнего изменения статуса заказа или параметров начисления.
	CreatedAt time.Time `db:"created_at"`
}
