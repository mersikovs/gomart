package repository

import (
	"context"
	"log/slog"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/model"
)

type Storage interface {
	CreateUser(ctx context.Context, login, password string) (model.User, error)
}

func NewStorage(ctx context.Context, cnf *config.AppConfig, logger *slog.Logger) (Storage, error) {
	return NewPgStorage(ctx, cnf.DatabaseURI, logger)
}
