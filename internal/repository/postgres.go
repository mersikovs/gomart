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

// ErrUserAlreadyExists сигнализирует о конфликте создания пользователя.
var ErrUserAlreadyExists = errors.New("user already exists")

// ErrOrderAlreadyExists сигнализирует о конфликте создания заказа.
var ErrOrderAlreadyExists = errors.New("order already exists")

// ErrUserNotFound возвращается при поиске пользователя в методах поиска по ID или Login,
var ErrUserNotFound = errors.New("user not found")

// ErrOrderNotFound возвращается репозиторием заказов (Storage),
// если заказ с данным номером не найден.
var ErrOrderNotFound = errors.New("order not found")

// ErrInsufficientFunds возникает при попытке списать сумму баллов,
// превышающую текущий доступный баланс пользователя.
var ErrInsufficientFunds = errors.New("insufficient funds")

// PgStorage реализует интерфейс Storage с использованием PostgreSQL в качестве хранилища данных.
type PgStorage struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewPgStorage конструктор PgStorage, который возвращает сконфигурированный указатель на структуру соотвествующую интерфейсу Storage
func NewPgStorage(ctx context.Context, dsn string, l *slog.Logger) (*PgStorage, error) {
	curPool, err := db.NewPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &PgStorage{pool: curPool, logger: l}, nil
}

// Ping тестирование работоспособности базы данных.
func (s *PgStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// CreateUser регистрирует нового пользователя в системе.
// Возвращает внутренний идентификатор созданной записи или ошибку.
func (s *PgStorage) CreateUser(ctx context.Context, login, password string) (int64, error) {
	query := `
        INSERT INTO users (login, password)
        VALUES ($1, $2)
		ON CONFLICT (login) DO NOTHING
		RETURNING id`

	var userID int64
	err := s.pool.QueryRow(ctx, query, login, password).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUserAlreadyExists
		}
		return 0, fmt.Errorf("failed to create user %q: %w", login, err)
	}

	return userID, nil
}

// CreateOrder создает новый заказ от имени пользователя userID.
// orderNumber должен быть уникальным. Возвращает объект заказа со статусом NEW.
func (s *PgStorage) CreateOrder(ctx context.Context, userID int64, orderNumber string) (*model.Order, error) {
	query := `
        INSERT INTO orders (user_id, number, action)
        VALUES ($1, $2, $3)
		RETURNING id`

	var orderID int64
	err := s.pool.QueryRow(ctx, query, userID, orderNumber, model.ActionEarn).Scan(&orderID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrOrderAlreadyExists
		}
		return nil, fmt.Errorf("failed to create order %s: %w", orderNumber, err)
	}

	return &model.Order{
		ID:     orderID,
		UserID: userID,
		Number: orderNumber,
		Status: "NEW",
		Action: model.ActionEarn,
	}, nil
}

// CreateWithdraw списывает сумму sum баллов у пользователя userID в счет покупки номер orderNumber.
// Возвращает созданный объект списания. Ошибка возникает при недостаточном балансе.
func (s *PgStorage) CreateWithdraw(ctx context.Context, userID int64, orderNumber string, sum int) (*model.Order, error) {

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction CreateWithdraw: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			s.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	var balance int64
	queryGetCurrentBalance := `SELECT current_balance FROM users WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(ctx, queryGetCurrentBalance, userID).Scan(&balance)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if balance < int64(sum) {
		return nil, ErrInsufficientFunds
	}

	queryDeduct := "UPDATE users SET current_balance = current_balance - $1, total_spent = total_spent + $1 WHERE id = $2"
	queryDeductResult, err := tx.Exec(ctx, queryDeduct, sum, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct funds: %w", err)
	}

	if queryDeductResult.RowsAffected() == 0 {
		return nil, fmt.Errorf("user not found")
	}

	query := `
        INSERT INTO orders (user_id, number, action, points)
        VALUES ($1, $2, $3, $4)
		RETURNING id`

	var orderID int64

	err = tx.QueryRow(ctx, query, userID, orderNumber, model.ActionSpend, sum).Scan(&orderID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrOrderAlreadyExists
		}
		return nil, fmt.Errorf("insert order record %q: %w", orderNumber, err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction %s: %w", orderNumber, err)
	}

	return &model.Order{
		ID:     orderID,
		UserID: userID,
		Number: orderNumber,
		Status: "PROCESSED",
		Action: model.ActionSpend,
		Points: sum,
	}, nil
}

// UpdateOrderStatusAndUserBalance атомарно обновляет статус заказа и баланс пользователя.
// Используется для подтверждения начисления бонусов за заказ или их списаний.
func (s *PgStorage) UpdateOrderStatusAndUserBalance(ctx context.Context, userID int64, orderNumber string, status model.OrderStatus, sum int) error {

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction UpdateOrderStatusAndUserBalance: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			s.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	queryChangeStatus := "UPDATE orders SET status = $1, points = $2 WHERE number = $3 AND status IN ($4,$5)"
	result, err := tx.Exec(ctx, queryChangeStatus, status, sum, orderNumber, model.OrderStatusProcessing, model.OrderStatusNew)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("order %q not found or already processed", orderNumber)
	}

	if sum > 0 {
		query := `UPDATE users SET current_balance = current_balance + $1 WHERE id = $2`
		_, err = tx.Exec(ctx, query, sum, userID)
		if err != nil {
			return fmt.Errorf("refund user balance: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %q: %w", orderNumber, err)
	}

	return nil
}

// GetOrderByNumber возвращает данные заказа по его номеру.
// Если заказ не найден, возвращается ошибка.
func (s *PgStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (*model.Order, error) {
	query := `SELECT id, user_id, status, action, points FROM orders o WHERE number = $1`
	row := s.pool.QueryRow(ctx, query, orderNumber)
	var id, userID, points int64
	var action string
	var status model.OrderStatus

	err := row.Scan(&id, &userID, &status, &action, &points)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("error scan orders: %w", err)
	}

	return &model.Order{
		ID:     id,
		UserID: userID,
		Status: status,
		Action: model.ActionType(action),
		Points: int(points),
	}, nil
}

// GetOrdersByUser возвращает список заказов конкретного пользователя id.
// action фильтрует выборку: начисление, списание.
func (s *PgStorage) GetOrdersByUser(ctx context.Context, userID int64, action model.ActionType) ([]model.Order, error) {
	query := `SELECT id, user_id, number, status, action, points, created_at FROM orders o WHERE user_id = $1 and action = $2 ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, userID, action)
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

// GetOrdersByStatus возвращает список всех заказов системы доступных для обновления по статусу.
func (s *PgStorage) GetOrdersAwaitingUpdate(ctx context.Context) ([]model.Order, error) {
	query := `SELECT id, user_id, number, status, action, points, created_at FROM orders o WHERE status IN ($1, $2) AND action = $3 ORDER BY created_at ASC`
	rows, err := s.pool.Query(ctx, query, model.OrderStatusNew, model.OrderStatusProcessing, model.ActionEarn)
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

// FindUserByID ищет пользователя по его внутреннему числовому идентификатору.
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

// FindUserByLogin ищет пользователя по строковому логину.
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

// Close закрывает пулл соединений
func (s *PgStorage) Close() error {
	if s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}
