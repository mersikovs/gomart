package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mersikovs/gomart/internal/config/db"
	"github.com/mersikovs/gomart/internal/model"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")

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

func (s *PgStorage) FindUserByLogin(ctx context.Context, login string) (*model.User, error) {
	query := `SELECT id,  password, current_balance, total_spent FROM users u WHERE login = $1`
	row := s.pool.QueryRow(ctx, query, login)
	var id int64
	var password string
	var currentBalance, totalSpent int

	err := row.Scan(&id, &password, &currentBalance, &totalSpent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error scan metric: %w", err)
	}

	return &model.User{
		ID:       id,
		Login:    login,
		Password: password,
	}, nil
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
