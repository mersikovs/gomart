package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mersikovs/gomart/internal/config/db"
)

var ErrUserAlreadyExists = errors.New("user already exists")

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

func (s *PgStorage) FindByLogin(ctx context.Context, login string) (bool, error) {
	query := `SELECT id FROM users u WHERE login = $1`
	row := s.pool.QueryRow(ctx, query, login)
	var id int64

	err := row.Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("scan metric: %w", err)
	}

	return id > 0, nil
}

func (s *PgStorage) CreateUser(ctx context.Context, login, password string) (int64, error) {
	query := `
        INSERT INTO users (login, password)
        VALUES ($1, $2)
		ON CONFLICT (login) DO NOTHING
		RETURNING id`

	var userId int64
	err := s.pool.QueryRow(ctx, query, login, password).Scan(&userId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserAlreadyExists
		}
		return 0, fmt.Errorf("save user %q: %w", login, err)
	}

	return userId, nil
}

func (s *PgStorage) Close() error {
	if s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}
