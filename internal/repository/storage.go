package repository

import (
	"context"
	"log/slog"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/model"
)

type Storage interface {
	FindUserByLogin(ctx context.Context, login string) (*model.User, error)
	CreateUser(ctx context.Context, login, password string) (int64, error)
}

func NewStorage(ctx context.Context, cnf *config.AppConfig, logger *slog.Logger) (Storage, error) {
	return NewPgStorage(ctx, cnf.DatabaseURI, logger)
}
