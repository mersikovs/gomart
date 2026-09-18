package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mersikovs/gomart/internal/config/db"
	"github.com/mersikovs/gomart/internal/model"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")
var ErrOrderNotFound = errors.New("order not found")
var ErrInsufficientFunds = errors.New("insufficient funds")

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

func (s *PgStorage) CreateOrder(ctx context.Context, userId int64, orderNumber string) (*model.Order, error) {
	query := `
        INSERT INTO orders (user_id, number, action)
        VALUES ($1, $2, $3)
		RETURNING id`

	var orderId int64
	err := s.pool.QueryRow(ctx, query, userId, orderNumber, model.ActionEarn).Scan(&orderId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("save order %s: %w", orderNumber, err)
	}

	return &model.Order{
		ID:     orderId,
		UserID: userId,
		Number: orderNumber,
		Status: "NEW",
		Action: model.ActionEarn,
	}, nil
}

func (s *PgStorage) CreateWithdraw(ctx context.Context, userId int64, orderNumber string, sum int) (*model.Order, error) {
	query := `
        INSERT INTO orders (user_id, number, action, points)
        VALUES ($1, $2, $3, $4)
		RETURNING id`

	var orderId int64

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка начала транзакции CreateWithdraw: %w", err)
	}
	defer tx.Rollback(ctx)

	queryDeduct := "UPDATE users SET current_balance = current_balance - $1, total_spent = total_spent + $1 WHERE id = $2"
	_, err = tx.Exec(ctx, queryDeduct, sum, userId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23514" || pgErr.Code == "22003" {
				return nil, ErrInsufficientFunds
			}
		}

		return nil, fmt.Errorf("ошибка списания средств: %w", err)
	}

	err = tx.QueryRow(ctx, query, userId, orderNumber, model.ActionSpend, sum).Scan(&orderId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("save order %s: %w", orderNumber, err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка применения транзакции %s: %w", orderNumber, err)
	}

	return &model.Order{
		ID:     orderId,
		UserID: userId,
		Number: orderNumber,
		Status: "PROCESSED",
		Action: model.ActionEarn,
		Points: sum,
	}, nil
}

func (s *PgStorage) UpdateOrderStatusAndUserBalance(ctx context.Context, userId int64, orderNumber string, status string, sum int) error {

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции UpdateOrderStatusAndUserBalance: %w", err)
	}
	defer tx.Rollback(ctx)

	queryChangeStatus := "UPDATE orders SET status = $1, points = $2 WHERE number = $3"
	_, err = tx.Exec(ctx, queryChangeStatus, status, sum, orderNumber)
	if err != nil {
		return fmt.Errorf("ошибка изменения статуса заказа: %w", err)
	}

	if sum > 0 {
		query := `UPDATE users SET current_balance = current_balance + $1 WHERE id = $2`
		_, err = tx.Exec(ctx, query, sum, userId)
		if err != nil {
			return fmt.Errorf("ошибка изменения %s: %w", orderNumber, err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка применения транзакции %s: %w", orderNumber, err)
	}

	return nil
}

func (s *PgStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (*model.Order, error) {
	query := `SELECT id, user_id, status, action, points FROM orders o WHERE number = $1`
	row := s.pool.QueryRow(ctx, query, orderNumber)
	var id, userId, points int64
	var status, action string

	err := row.Scan(&id, &userId, &status, &action, &points)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("error scan orders: %w", err)
	}

	return &model.Order{
		ID:     id,
		UserID: userId,
		Status: status,
		Action: model.ActionType(action),
		Points: int(points),
	}, nil
}

func (s *PgStorage) GetOrdersByUser(ctx context.Context, userId int64, action model.ActionType) ([]model.Order, error) {
	query := `SELECT id, user_id, number, status, action, points, created_at FROM orders o WHERE user_id = $1 and action = $2`
	rows, err := s.pool.Query(ctx, query, userId, action)
	if err != nil {
		return nil, fmt.Errorf("error Query GetOrdersByUser: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])

	if err != nil {
		return nil, fmt.Errorf("error CollectRows GetOrdersByUser: %w", err)
	}

	return orders, nil
}

func (s *PgStorage) GetOrdersByStatus(ctx context.Context, status string, action model.ActionType) ([]model.Order, error) {
	query := `SELECT id, user_id, number, status, action, points, created_at FROM orders o WHERE status = $1 AND action = $2`
	rows, err := s.pool.Query(ctx, query, status, action)
	if err != nil {
		return nil, fmt.Errorf("error Query GetOrdersByStatus: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])

	if err != nil {
		return nil, fmt.Errorf("error CollectRows GetOrdersByUser: %w", err)
	}

	return orders, nil
}

func (s *PgStorage) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	query := `SELECT id, login,  current_balance, total_spent FROM users u WHERE id = $1`
	row := s.pool.QueryRow(ctx, query, id)
	var login string
	var currentBalance, totalSpent int

	err := row.Scan(&id, &login, &currentBalance, &totalSpent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error scan users: %w", err)
	}

	return &model.User{
		ID:         id,
		Login:      login,
		Balance:    currentBalance,
		TotalSpent: totalSpent,
	}, nil
}

func (s *PgStorage) FindUserByLogin(ctx context.Context, login string) (*model.User, error) {
	query := `SELECT id, password FROM users u WHERE login = $1`
	row := s.pool.QueryRow(ctx, query, login)
	var id int64
	var password string

	err := row.Scan(&id, &password)
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

func (s *PgStorage) Close() error {
	if s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}
