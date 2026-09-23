package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/repository"
)

// OrderProcessStatus Статус заказа для разделения ситуации заказ добавлен, заказ уже добавлен
type OrderProcessStatus int

const (
	// OrderStatusAlreadyAdded сигнализирует о том, что заказ с таким идентификатором
	// уже была зарегистрирована ранее в системе.
	OrderStatusAlreadyAdded OrderProcessStatus = iota

	// OrderStatusAdded означает успешную первичную регистрацию новой сущности.
	OrderStatusAdded
)

// ErrOrderAlreadyProcessedByOther возвращается при попытке изменить или добавить заказ,
// который уже был обработан другим пользователем (например, в условиях гонки).
var ErrOrderAlreadyProcessedByOther = errors.New("order already processed by another user")

// ErrWithdrawInsufficientFunds возникает при попытке списать сумму баллов,
// превышающую текущий доступный баланс пользователя.
var ErrWithdrawInsufficientFunds = errors.New("there are not enough funds")

// OrderService определяет контракт бизнес-логики для работы с заказами и списаниями пользователя.
// Все методы принимают context.Context для управления жизненным циклом запроса.
type OrderService interface {
	// RegisterOrder регистрирует новый заказ в системе от имени пользователя userID.
	// Возвращает статус обработки для разделения ситуации появления нового заказа или когда такой заказ зарегистрирован, так как не ошибка.
	RegisterOrder(ctx context.Context, userID int64, orderNumber string) (OrderProcessStatus, error)

	// OrderList возвращает список всех заказов указанного пользователя.
	OrderList(ctx context.Context, userID int64) ([]model.Order, error)

	// RegisterWithdraw создает заказ на списание суммы sum баллов у пользователя userID
	// в счет покупки для заказа с orderNumber.
	// Возвращает статус операции. Ошибка ErrWithdrawInsufficientFunds возникает при нехватке баланса.
	RegisterWithdraw(ctx context.Context, userID int64, orderNumber string, sum int) (OrderProcessStatus, error)

	// WithdrawList возвращает историю списаний баллов для указанного пользователя.
	WithdrawList(ctx context.Context, userID int64) ([]model.Order, error)
}

type orderService struct {
	repo    repository.Storage
	accrual accrualclient.AccrualClient
	logger  *slog.Logger
}

// NewOrderService конструктор orderService, который возвращает сконфигурированный указатель на структуру
func NewOrderService(repo repository.Storage, client accrualclient.AccrualClient, log *slog.Logger) OrderService {
	return &orderService{
		repo:    repo,
		accrual: client,
		logger:  log,
	}
}

func (s *orderService) RegisterOrder(ctx context.Context, userID int64, orderNumber string) (OrderProcessStatus, error) {
	var status OrderProcessStatus
	status = OrderStatusAdded
	order, err := s.repo.GetOrderByNumber(ctx, orderNumber)

	if err == nil {
		if order.UserID != userID {
			return status, ErrOrderAlreadyProcessedByOther
		}
		status = OrderStatusAlreadyAdded
		return status, nil
	}

	if !errors.Is(err, repository.ErrOrderNotFound) {
		return status, fmt.Errorf("error GetOrderByNumber %w", err)
	}

	_, err = s.repo.CreateOrder(ctx, userID, orderNumber)
	if err != nil {
		if errors.Is(err, repository.ErrOrderAlreadyExists) {
			existing, existingError := s.repo.GetOrderByNumber(ctx, orderNumber)
			if existingError != nil {
				return status, fmt.Errorf("get order after conflict: %w", existingError)
			}
			if existing.UserID == userID {
				return OrderStatusAlreadyAdded, nil
			}
			return status, ErrOrderAlreadyProcessedByOther
		}
		return status, fmt.Errorf("error CreateOrder: %w", err)
	}

	orderInfo, err := s.accrual.GetOrder(ctx, orderNumber)
	if err != nil {
		return status, nil
	}

	switch orderInfo.Status {
	case model.OrderStatusProcessed:
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userID, orderNumber, orderInfo.Status, orderInfo.Points)
		if err != nil {
			return status, nil
		}

	case model.OrderStatusInvalid:
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userID, orderNumber, orderInfo.Status, 0)
		if err != nil {
			return status, nil
		}

	}

	return status, nil
}

func (s *orderService) OrderList(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userID, model.ActionEarn)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return orders, nil
}

func (s *orderService) RegisterWithdraw(ctx context.Context, userID int64, orderNumber string, sum int) (OrderProcessStatus, error) {
	var status OrderProcessStatus
	status = OrderStatusAdded
	order, err := s.repo.GetOrderByNumber(ctx, orderNumber)

	if err == nil {
		if order.UserID != userID {
			return status, ErrOrderAlreadyProcessedByOther
		}
		status = OrderStatusAlreadyAdded
		return status, nil
	}

	if !errors.Is(err, repository.ErrOrderNotFound) {
		return status, fmt.Errorf("error GetOrderByNumber %w", err)
	}

	_, err = s.repo.CreateWithdraw(ctx, userID, orderNumber, sum)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientFunds) {
			return status, ErrWithdrawInsufficientFunds
		}
		return status, fmt.Errorf("error CreateWithdraw: %w", err)
	}

	return status, nil
}

func (s *orderService) WithdrawList(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userID, model.ActionSpend)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return orders, nil
}
