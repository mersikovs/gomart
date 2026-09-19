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

type OrderProcessStatus int

const (
	OrderStatusAlreadyAdded OrderProcessStatus = iota
	OrderStatusAdded
)

var ErrOrderAlreadyProcessedByOther = errors.New("order already processed by another user") // → 409
var ErrWithdrawInsufficientFunds = errors.New("there are not enough funds.")                // → 402

type OrderService interface {
	RegisterOrder(ctx context.Context, userId int64, orderNumber string) (OrderProcessStatus, error)
	OrderList(ctx context.Context, userId int64) ([]model.Order, error)
	RegisterWithdraw(ctx context.Context, userId int64, orderNumber string, sum int) (OrderProcessStatus, error)
	WithdrawList(ctx context.Context, userId int64) ([]model.Order, error)
}

type orderService struct {
	repo    repository.Storage
	accrual *accrualclient.DynHTTPClient
	logger  *slog.Logger
}

func NewOrderService(repo repository.Storage, client *accrualclient.DynHTTPClient, log *slog.Logger) OrderService {
	return &orderService{
		repo:    repo,
		accrual: client,
		logger:  log,
	}
}

func (s *orderService) RegisterOrder(ctx context.Context, userId int64, orderNumber string) (OrderProcessStatus, error) {
	var status OrderProcessStatus
	status = OrderStatusAdded
	order, err := s.repo.GetOrderByNumber(ctx, orderNumber)

	if err == nil {
		if order.UserID != userId {
			return status, ErrOrderAlreadyProcessedByOther
		} else {
			status = OrderStatusAlreadyAdded
			return status, nil
		}
	}

	if !errors.Is(err, repository.ErrOrderNotFound) {
		return status, fmt.Errorf("error GetOrderByNumber %w", err)
	}

	_, err = s.repo.CreateOrder(ctx, userId, orderNumber)
	if err != nil {
		return status, fmt.Errorf("error CreateOrder: %w", err)
	}

	orderInfo, err := s.accrual.GetOrder(ctx, orderNumber)
	if err != nil {
		return status, nil
	}

	switch orderInfo.Status {
	case model.OrderStatusProcessed:
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userId, orderNumber, orderInfo.Status, orderInfo.Points)
		if err != nil {
			return status, nil
		}

	case model.OrderStatusInvalid:
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userId, orderNumber, orderInfo.Status, 0)
		if err != nil {
			return status, nil
		}

	}

	return status, nil
}

func (s *orderService) OrderList(ctx context.Context, userId int64) ([]model.Order, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userId, model.ActionEarn)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return orders, nil
}

func (s *orderService) RegisterWithdraw(ctx context.Context, userId int64, orderNumber string, sum int) (OrderProcessStatus, error) {
	var status OrderProcessStatus
	status = OrderStatusAdded
	order, err := s.repo.GetOrderByNumber(ctx, orderNumber)

	if err == nil {
		if order.UserID != userId {
			return status, ErrOrderAlreadyProcessedByOther
		} else {
			status = OrderStatusAlreadyAdded
			return status, nil
		}
	}

	if !errors.Is(err, repository.ErrOrderNotFound) {
		return status, fmt.Errorf("error GetOrderByNumber %w", err)
	}

	_, err = s.repo.CreateWithdraw(ctx, userId, orderNumber, sum)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientFunds) {
			return status, ErrWithdrawInsufficientFunds
		}
		return status, fmt.Errorf("error CreateWithdraw: %w", err)
	}

	return status, nil
}

func (s *orderService) WithdrawList(ctx context.Context, userId int64) ([]model.Order, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userId, model.ActionSpend)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	return orders, nil
}
