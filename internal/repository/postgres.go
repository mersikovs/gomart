package repository

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mersikovs/gomart/internal/config/db"
	"github.com/mersikovs/gomart/internal/model"
)

type PgStorage struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewPgStorage(ctx context.Context, dsn string, l *slog.Logger) (*PgStorage, error) {
	curPool, err := db.NewPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &PgStorage{pool: curPool, logger: l}, nil
}

func (s *PgStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PgStorage) CreateUser(ctx context.Context, login, password string) (model.User, error) {
	return model.User{}, nil
}

func (s *PgStorage) Close() error {
	if s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}
