package repository

import (
	"context"
	"log/slog"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/model"
)

type Storage interface {
	CreateUser(ctx context.Context, login, password string) (int64, error)
	CreateOrder(ctx context.Context, userId int64, orderNumber string) (*model.Order, error)
	CreateWithdraw(ctx context.Context, userId int64, orderNumber string, sum int) (*model.Order, error)
	UpdateOrderStatusAndUserBalance(ctx context.Context, userId int64, orderNumber string, status string, sum int) error
	GetOrderByNumber(ctx context.Context, orderNumber string) (*model.Order, error)
	GetOrdersByUser(ctx context.Context, id int64, action model.ActionType) ([]model.Order, error)
	GetOrdersByStatus(ctx context.Context, status string, action model.ActionType) ([]model.Order, error)
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
	FindUserByLogin(ctx context.Context, login string) (*model.User, error)
}

func NewStorage(ctx context.Context, cnf *config.AppConfig, logger *slog.Logger) (Storage, error) {
	return NewPgStorage(ctx, cnf.DatabaseURI, logger)
}
