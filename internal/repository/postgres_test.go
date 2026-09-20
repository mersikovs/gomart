//go:build integration

package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mersikovs/gomart/internal/database"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),

		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "не удалось запустить контейнер Postgres")

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "не удалось получить строку подключения")

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("не удалось остановить контейнер: %v", err)
		}
	}

	return dsn, cleanup
}

func truncateAll(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `TRUNCATE users, orders RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func TestPgStorage_CreateUser(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name     string
		seed     func(t *testing.T, pool *pgxpool.Pool) // подготовка данных
		login    string
		password string
		wantID   int64
		wantErr  error
	}{
		{
			name:     "успех_создание_нового_пользователя",
			seed:     func(t *testing.T, pool *pgxpool.Pool) {}, // таблица пуста
			login:    "newuser",
			password: "securepassword",
			wantID:   1,
			wantErr:  nil,
		},
		{
			name: "ошибка_пользователь_уже_существует",
			seed: func(t *testing.T, pool *pgxpool.Pool) {

				_, err := pool.Exec(ctx, `INSERT INTO users(login, password) VALUES ($1, $2)`, "existinguser", "oldpass")
				require.NoError(t, err)
			},
			login:    "existinguser",
			password: "newpassword",
			wantID:   0,
			wantErr:  ErrUserAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			tt.seed(t, s.pool)

			gotID, err := s.CreateUser(ctx, tt.login, tt.password)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, int64(0), gotID, "при ошибке ID должен быть 0")
				return
			}

			require.NoError(t, err)
			require.Greater(t, gotID, int64(0), "ID должен быть больше 0")

			var dbLogin, dbPassword string
			err = s.pool.QueryRow(ctx, `SELECT login, password FROM users WHERE id = $1`, gotID).Scan(&dbLogin, &dbPassword)
			require.NoError(t, err)
			require.Equal(t, tt.login, dbLogin)
			require.Equal(t, tt.password, dbPassword)
		})
	}
}

func TestPgStorage_CreateOrder(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name        string
		seed        func(t *testing.T, pool *pgxpool.Pool) int64
		orderNumber string
		wantErr     error
		validate    func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string)
	}{
		{
			name: "успех_создание_нового_заказа",
			seed: func(t *testing.T, pool *pgxpool.Pool) int64 {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`,
					"alice", "hash").Scan(&userID)
				require.NoError(t, err)
				return userID
			},
			orderNumber: "001",
			wantErr:     nil,
			validate: func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string) {
				var status string
				var action model.ActionType
				err := pool.QueryRow(ctx,
					`SELECT status, action FROM orders WHERE number = $1`, orderNumber).
					Scan(&status, &action)
				require.NoError(t, err)
				require.Equal(t, "NEW", status)
				require.Equal(t, model.ActionEarn, action)
			},
		},
		{
			name: "ошибка_дубликат_номера_заказа",
			seed: func(t *testing.T, pool *pgxpool.Pool) int64 {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`,
					"bob", "hash").Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, action, status) VALUES ($1, $2, $3, $4)`,
					userID, "1111", model.ActionEarn, "NEW")
				require.NoError(t, err)

				return userID
			},
			orderNumber: "1111",
			wantErr:     ErrOrderAlreadyExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			userID := tt.seed(t, s.pool)

			gotOrder, err := s.CreateOrder(ctx, userID, tt.orderNumber)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, gotOrder)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, gotOrder)

			require.Equal(t, userID, gotOrder.UserID)
			require.Equal(t, tt.orderNumber, gotOrder.Number)
			require.Equal(t, "NEW", string(gotOrder.Status))
			require.Equal(t, model.ActionEarn, gotOrder.Action)

			if tt.validate != nil {
				tt.validate(t, s.pool, userID, tt.orderNumber)
			}
		})
	}
}

func TestPgStorage_CreateWithdraw(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name        string
		seed        func(t *testing.T, pool *pgxpool.Pool) (userID int64)
		orderNumber string
		sum         int
		wantErr     error
		validate    func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string)
	}{
		{
			name: "успех_списание_баллов_и_создание_заказа",
			seed: func(t *testing.T, pool *pgxpool.Pool) int64 {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"alice", "hash", 1000, 200).Scan(&userID)
				require.NoError(t, err)
				return userID
			},
			orderNumber: "111",
			sum:         300,
			wantErr:     nil,
			validate: func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string) {

				var balance, totalSpent int
				err := pool.QueryRow(ctx,
					`SELECT current_balance, total_spent FROM users WHERE id = $1`, userID).
					Scan(&balance, &totalSpent)
				require.NoError(t, err)
				require.Equal(t, 700, balance, "баланс должен уменьшиться на 300 (1000 - 300)")
				require.Equal(t, 500, totalSpent, "total_spent должен увеличиться на 300 (200 + 300)")

				var action model.ActionType
				var points int
				err = pool.QueryRow(ctx,
					`SELECT action, points FROM orders WHERE number = $1`, orderNumber).
					Scan(&action, &points)
				require.NoError(t, err)
				require.Equal(t, model.ActionSpend, action)
				require.Equal(t, 300, points)
			},
		},
		{
			name: "ошибка_недостаточно_средств",
			seed: func(t *testing.T, pool *pgxpool.Pool) int64 {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"bob", "hash", 100, 0).Scan(&userID)
				require.NoError(t, err)
				return userID
			},
			orderNumber: "111111",
			sum:         200,
			wantErr:     ErrInsufficientFunds,
		},
		{
			name: "ошибка_пользователь_не_найден",
			seed: func(t *testing.T, pool *pgxpool.Pool) int64 {
				return 999999 // Несуществующий ID
			},
			orderNumber: "222222",
			sum:         100,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)

			userID := tt.seed(t, s.pool)

			gotOrder, err := s.CreateWithdraw(ctx, userID, tt.orderNumber, tt.sum)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, gotOrder)
				return
			}

			if tt.name == "ошибка_пользователь_не_найден" {
				require.Error(t, err)
				require.Contains(t, err.Error(), "user not found")
				require.Nil(t, gotOrder)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, gotOrder)

			require.Equal(t, userID, gotOrder.UserID)
			require.Equal(t, tt.orderNumber, gotOrder.Number)
			require.Equal(t, tt.sum, gotOrder.Points)

			require.Equal(t, model.ActionSpend, gotOrder.Action)

			if tt.validate != nil {
				tt.validate(t, s.pool, userID, tt.orderNumber)
			}
		})
	}
}

func TestPgStorage_UpdateOrderStatusAndUserBalance(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name        string
		seed        func(t *testing.T, pool *pgxpool.Pool) (userID int64, orderNumber string)
		status      model.OrderStatus
		sum         int
		wantErr     bool
		wantErrText string // часть текста ошибки для проверки
		validate    func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string)
	}{
		{
			name: "успех_статус_обновлен_с_начислением_баллов",
			seed: func(t *testing.T, pool *pgxpool.Pool) (int64, string) {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"alice", "hash", 1000, 0).Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "79927398713", model.OrderStatusNew, "earn", 0)
				require.NoError(t, err)

				return userID, "79927398713"
			},
			status:  model.OrderStatusProcessed,
			sum:     500,
			wantErr: false,
			validate: func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string) {

				var balance int
				err := pool.QueryRow(ctx, `SELECT current_balance FROM users WHERE id = $1`, userID).Scan(&balance)
				require.NoError(t, err)
				require.Equal(t, 1500, balance, "баланс должен увеличиться на 500 (1000 + 500)")

				var status model.OrderStatus
				var points int
				err = pool.QueryRow(ctx, `SELECT status, points FROM orders WHERE number = $1`, orderNumber).Scan(&status, &points)
				require.NoError(t, err)
				require.Equal(t, model.OrderStatusProcessed, status)
				require.Equal(t, 500, points)
			},
		},
		{
			name: "успех_статус_processing_на_processed",
			seed: func(t *testing.T, pool *pgxpool.Pool) (int64, string) {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"bob", "hash", 0, 0).Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "5555555555555", model.OrderStatusProcessing, "earn", 0)
				require.NoError(t, err)

				return userID, "5555555555555"
			},
			status:  model.OrderStatusProcessed,
			sum:     200,
			wantErr: false,
			validate: func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string) {
				var balance int
				err := pool.QueryRow(ctx, `SELECT current_balance FROM users WHERE id = $1`, userID).Scan(&balance)
				require.NoError(t, err)
				require.Equal(t, 200, balance)
			},
		},
		{
			name: "успех_sum_равен_нулю_баланс_не_меняется",
			seed: func(t *testing.T, pool *pgxpool.Pool) (int64, string) {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"carol", "hash", 300, 0).Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "7777777777777", model.OrderStatusNew, "earn", 0)
				require.NoError(t, err)

				return userID, "7777777777777"
			},
			status:  model.OrderStatusInvalid,
			sum:     0,
			wantErr: false,
			validate: func(t *testing.T, pool *pgxpool.Pool, userID int64, orderNumber string) {

				var balance int
				err := pool.QueryRow(ctx, `SELECT current_balance FROM users WHERE id = $1`, userID).Scan(&balance)
				require.NoError(t, err)
				require.Equal(t, 300, balance, "баланс не должен измениться при sum=0")

				var status model.OrderStatus
				err = pool.QueryRow(ctx, `SELECT status FROM orders WHERE number = $1`, orderNumber).Scan(&status)
				require.NoError(t, err)
				require.Equal(t, model.OrderStatusInvalid, status)
			},
		},
		{
			name: "ошибка_заказ_уже_обработан_финальный_статус",
			seed: func(t *testing.T, pool *pgxpool.Pool) (int64, string) {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"dave", "hash", 0, 0).Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "8888888888888", model.OrderStatusProcessed, "earn", 100) // уже финальный
				require.NoError(t, err)

				return userID, "8888888888888"
			},
			status:      model.OrderStatusInvalid,
			sum:         0,
			wantErr:     true,
			wantErrText: "not found or already processed",
		},
		{
			name: "ошибка_заказ_не_существует",
			seed: func(t *testing.T, pool *pgxpool.Pool) (int64, string) {
				var userID int64
				err := pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent) VALUES ($1, $2, $3, $4) RETURNING id`,
					"eve", "hash", 0, 0).Scan(&userID)
				require.NoError(t, err)
				return userID, "9999999999999"
			},
			status:      model.OrderStatusProcessed,
			sum:         100,
			wantErr:     true,
			wantErrText: "not found or already processed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			userID, orderNumber := tt.seed(t, s.pool)

			err := s.UpdateOrderStatusAndUserBalance(ctx, userID, orderNumber, tt.status, tt.sum)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrText != "" {
					require.Contains(t, err.Error(), tt.wantErrText)
				}
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, s.pool, userID, orderNumber)
			}
		})
	}
}

func TestPgStorage_GetOrderByNumber(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name     string
		seed     func(t *testing.T, pool *pgxpool.Pool)
		orderNum string
		want     *model.Order
		wantErr  error
	}{
		{
			name: "успех_заказ_найден",
			seed: func(t *testing.T, pool *pgxpool.Pool) {
				var userID int64
				err := pool.QueryRow(ctx, `INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`, "alice", "hash").Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx, `INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "79927398713", "PROCESSED", "earn", 1500)
				require.NoError(t, err)
			},
			orderNum: "79927398713",
			want: &model.Order{

				Number: "",
				Status: "PROCESSED",
				Action: model.ActionType("earn"),
				Points: 1500,
			},
			wantErr: nil,
		},
		{
			name: "ошибка_заказ_не_найден",
			seed: func(t *testing.T, pool *pgxpool.Pool) {

				var userID int64
				err := pool.QueryRow(ctx, `INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`, "bob", "hash").Scan(&userID)
				require.NoError(t, err)

				_, err = pool.Exec(ctx, `INSERT INTO orders(user_id, number, status, action, points) VALUES ($1, $2, $3, $4, $5)`,
					userID, "1111111111111", "NEW", "earn", 100)
				require.NoError(t, err)
			},
			orderNum: "9999999999999", // Несуществующий номер
			want:     nil,
			wantErr:  ErrOrderNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			tt.seed(t, s.pool)

			got, err := s.GetOrderByNumber(ctx, tt.orderNum)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			got.ID = 0
			got.UserID = 0

			require.Equal(t, tt.want, got)
		})
	}
}

type orderSeed struct {
	Number string
	Status string
	Action string
	Points int
}

func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool,
	login, password string, balance, spent int) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO users(login, password, current_balance, total_spent)
         VALUES ($1, $2, $3, $4) RETURNING id`,
		login, password, balance, spent).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedOrders(t *testing.T, ctx context.Context, pool *pgxpool.Pool,
	userID int64, createdAt time.Time, seeds []orderSeed) []model.Order {
	t.Helper()
	if len(seeds) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO orders(user_id, number, status, action, points, created_at) VALUES `)
	args := []any{}
	for i, o := range seeds {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d)",
			len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5, len(args)+6)
		args = append(args, userID, o.Number, o.Status, o.Action, o.Points, createdAt)
	}
	sb.WriteString(` RETURNING id, user_id, number, status, action, points, created_at`)

	rows, err := pool.Query(ctx, sb.String(), args...)
	require.NoError(t, err)

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Order])
	require.NoError(t, err)
	return orders
}

func TestPgStorage_GetOrdersByUser(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		seed   func(t *testing.T) (userID int64, want []model.Order)
		action model.ActionType
	}{
		{
			name: "найдены только заказы этого пользователя с нужным action",
			seed: func(t *testing.T) (int64, []model.Order) {
				aliceID := seedUser(t, ctx, s.pool, "alice", "hash", 0, 0)
				bobID := seedUser(t, ctx, s.pool, "bob", "hash", 0, 0)

				aliceOrders := seedOrders(t, ctx, s.pool, aliceID, fixedTime, []orderSeed{
					{Number: "111", Status: "PROCESSED", Action: string(model.ActionEarn), Points: 100},
					{Number: "222", Status: "NEW", Action: string(model.ActionEarn), Points: 200},
					{Number: "333", Status: "PROCESSED", Action: string(model.ActionSpend), Points: 300}, // другой action — не ждём
				})

				_ = seedOrders(t, ctx, s.pool, bobID, fixedTime, []orderSeed{
					{Number: "444", Status: "PROCESSED", Action: string(model.ActionEarn), Points: 400},
				})

				want := []model.Order{}
				for _, o := range aliceOrders {
					if o.Action == model.ActionEarn {
						want = append(want, o)
					}
				}
				return aliceID, want
			},
			action: model.ActionType(string(model.ActionEarn)),
		},
		{
			name: "у пользователя нет заказов — пустой срез",
			seed: func(t *testing.T) (int64, []model.Order) {
				id := seedUser(t, ctx, s.pool, "alice", "hash", 0, 0)
				return id, []model.Order{}
			},
			action: model.ActionType(string(model.ActionEarn)),
		},
		{
			name: "у пользователя есть заказы, но с другим action",
			seed: func(t *testing.T) (int64, []model.Order) {
				id := seedUser(t, ctx, s.pool, "alice", "hash", 0, 0)
				_ = seedOrders(t, ctx, s.pool, id, fixedTime, []orderSeed{
					{Number: "111", Status: "PROCESSED", Action: "spend", Points: 100},
				})
				return id, []model.Order{}
			},
			action: model.ActionType(string(model.ActionEarn)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			userID, want := tt.seed(t)

			got, err := s.GetOrdersByUser(ctx, userID, tt.action)
			require.NoError(t, err)

			require.ElementsMatch(t, want, got)
		})
	}
}

func TestPgStorage_GetOrdersByStatus(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		seed    func(t *testing.T)
		status  string
		action  model.ActionType
		want    []model.Order
		wantErr bool
	}{
		{
			name: "найдены два заказа с нужным статусом и action",
			seed: func(t *testing.T) {
				var id int64
				err := s.pool.QueryRow(ctx,
					`INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`,
					"alice", "hash").Scan(&id)
				require.NoError(t, err)

				_, err = s.pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points, created_at) VALUES
                        ($2, '111', 'PROCESSED', 'earn', 100, $1),
                        ($2, '222', 'PROCESSED', 'earn', 200, $1),
                        ($2, '333', 'NEW',       'earn', 300, $1),
                        ($2, '444', 'PROCESSED', 'spend', 400, $1)`,
					fixedTime, id)
				require.NoError(t, err)

			},
			status: "PROCESSED",
			action: model.ActionType(string(model.ActionEarn)),
			want: []model.Order{
				{Number: "111", Status: "PROCESSED", Action: model.ActionType(string(model.ActionEarn)), Points: 100, CreatedAt: fixedTime},
				{Number: "222", Status: "PROCESSED", Action: model.ActionType(string(model.ActionEarn)), Points: 200, CreatedAt: fixedTime},
			},
		},
		{
			name: "пустой результат — нет подходящих заказов",
			seed: func(t *testing.T) {
				var id int64
				err := s.pool.QueryRow(ctx,
					`INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id`,
					"alice", "hash").Scan(&id)
				require.NoError(t, err)

				_, err = s.pool.Exec(ctx,
					`INSERT INTO orders(user_id, number, status, action, points, created_at) VALUES
                        ($2, '111', 'NEW', 'earn', 100, $1)`,
					fixedTime, id)
				require.NoError(t, err)
			},
			status: "PROCESSED",
			action: model.ActionType(string(model.ActionEarn)),
			want:   []model.Order{},
		},
		{
			name:   "пустая таблица — пустой срез без ошибки",
			seed:   func(t *testing.T) {},
			status: "PROCESSED",
			action: model.ActionType(string(model.ActionEarn)),
			want:   []model.Order{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			tt.seed(t)

			got, err := s.GetOrdersByStatus(ctx, tt.status, tt.action)
			require.NoError(t, err)

			for i := range got {
				got[i].ID = 0
				got[i].UserID = 0
			}

			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestPgStorage_FindUserByID(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name    string
		seed    func(t *testing.T) int64 // возвращает id созданного пользователя (0 если никого не создаём)
		want    *model.User              // nil, если ожидаем ошибку
		wantErr error                    // ErrUserNotFound или nil
	}{
		{
			name: "найден с балансом и тратами",
			seed: func(t *testing.T) int64 {
				var id int64
				err := s.pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent)
                     VALUES ($1, $2, $3, $4)
                     RETURNING id`,
					"alice", "hash", 1500, 300,
				).Scan(&id)
				require.NoError(t, err)
				return id
			},
			want: &model.User{
				Login:      "alice",
				Balance:    1500,
				TotalSpent: 300,
			},
		},
		{
			name: "найден с нулевым балансом",
			seed: func(t *testing.T) int64 {
				var id int64
				err := s.pool.QueryRow(ctx,
					`INSERT INTO users(login, password, current_balance, total_spent)
                     VALUES ($1, $2, $3, $4)
                     RETURNING id`,
					"bob", "hash", 0, 0,
				).Scan(&id)
				require.NoError(t, err)
				return id
			},
			want: &model.User{
				Login:      "bob",
				Balance:    0,
				TotalSpent: 0,
			},
		},
		{
			name:    "не найден",
			seed:    func(t *testing.T) int64 { return 999_999 }, // заведомо несуществующий id
			wantErr: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			id := tt.seed(t)

			got, err := s.FindUserByID(ctx, id)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, id, got.ID)
			require.Equal(t, tt.want.Login, got.Login)
			require.Equal(t, tt.want.Balance, got.Balance)
			require.Equal(t, tt.want.TotalSpent, got.TotalSpent)
		})
	}
}

func TestPgStorage_FindUserByLogin(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupTestDB(t)
	defer cleanup()

	s, err := NewPgStorage(ctx, dsn, slog.Default())
	require.NoError(t, err)
	defer s.Close()

	database.MigrateUp(dsn)

	tests := []struct {
		name    string
		seed    func(t *testing.T) // готовим данные под кейс
		login   string
		want    *model.User // nil, если ожидаем ошибку
		wantErr error       // ErrUserNotFound или nil
	}{
		{
			name: "найден",
			seed: func(t *testing.T) {
				_, err := s.pool.Exec(ctx,
					`INSERT INTO users(login, password) VALUES ($1, $2)`,
					"alice", "hash")
				require.NoError(t, err)
			},
			login: "alice",
			want:  &model.User{Login: "alice", Password: "hash"},
		},
		{
			name:    "не найден",
			seed:    func(t *testing.T) {}, // ничего не сеем
			login:   "bob",
			wantErr: ErrUserNotFound,
		},
		{
			name: "несколько пользователей — берём нужного",
			seed: func(t *testing.T) {
				_, err := s.pool.Exec(ctx,
					`INSERT INTO users(login, password) VALUES
                        ('alice', 'hash1'),
                        ('bob',   'hash2')`)
				require.NoError(t, err)
			},
			login: "bob",
			want:  &model.User{Login: "bob", Password: "hash2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateAll(t, ctx, s.pool)
			tt.seed(t)

			got, err := s.FindUserByLogin(ctx, tt.login)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want.Login, got.Login)
			require.Equal(t, tt.want.Password, got.Password)
		})
	}
}
