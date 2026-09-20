// Package repository предоставляет слой доступа к данным (Data Access Layer) для приложения.
package repository

import (
	"context"
	"log/slog"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/model"
)

// Storage определяет контракт для работы с слоем БД.
// Все методы принимают context.Context для управления таймаутами и отмены запросов.
type Storage interface {
	// CreateUser регистрирует нового пользователя в системе.
	// Возвращает внутренний идентификатор созданной записи или ошибку.
	CreateUser(ctx context.Context, login, password string) (int64, error)

	// CreateOrder создает новый заказ от имени пользователя userID.
	// orderNumber должен быть уникальным. Возвращает объект заказа со статусом NEW.
	CreateOrder(ctx context.Context, userID int64, orderNumber string) (*model.Order, error)

	// CreateWithdraw списывает сумму sum баллов у пользователя userID в счет покупки номер orderNumber.
	// Возвращает созданный объект списания. Ошибка возникает при недостаточном балансе.
	CreateWithdraw(ctx context.Context, userID int64, orderNumber string, sum int) (*model.Order, error)

	// UpdateOrderStatusAndUserBalance атомарно обновляет статус заказа и баланс пользователя.
	// Используется для подтверждения начисления бонусов за заказ или их списаний.
	UpdateOrderStatusAndUserBalance(ctx context.Context, userID int64, orderNumber string, status model.OrderStatus, sum int) error

	// GetOrderByNumber возвращает данные заказа по его номеру.
	// Если заказ не найден, возвращается ошибка.
	GetOrderByNumber(ctx context.Context, orderNumber string) (*model.Order, error)

	// GetOrdersByUser возвращает список заказов конкретного пользователя id.
	// action фильтрует выборку: начисление, списание.
	GetOrdersByUser(ctx context.Context, id int64, action model.ActionType) ([]model.Order, error)

	// GetOrdersByStatus возвращает список всех заказов системы с заданным статусом.
	// status фильтрует выборку по статусу.
	// action фильтрует выборку: начисление, списание.
	GetOrdersByStatus(ctx context.Context, status string, action model.ActionType) ([]model.Order, error)

	// FindUserByID ищет пользователя по его внутреннему числовому идентификатору.
	FindUserByID(ctx context.Context, id int64) (*model.User, error)

	// FindUserByLogin ищет пользователя по строковому логину.
	FindUserByLogin(ctx context.Context, login string) (*model.User, error)
}

// NewStorage конструктор Storage, который возвращает сконфигурированный указатель на структуру соотвествующую интерфейсу
func NewStorage(ctx context.Context, cnf *config.AppConfig, logger *slog.Logger) (Storage, error) {
	return NewPgStorage(ctx, cnf.DatabaseURI, logger)
}
