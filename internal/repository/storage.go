package repository

import (
	"context"
	"log/slog"

	"github.com/mersikovs/gomart/internal/config"
)

type Storage interface {
	FindByLogin(ctx context.Context, login string) (bool, error)
	CreateUser(ctx context.Context, login, password string) (int64, error)
}

func NewStorage(ctx context.Context, cnf *config.AppConfig, logger *slog.Logger) (Storage, error) {
	return NewPgStorage(ctx, cnf.DatabaseURI, logger)
}
