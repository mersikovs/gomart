package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

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
	OrderList(ctx context.Context, userId int64) ([]OrderResponse, error)
	RegisterWithdraw(ctx context.Context, userId int64, orderNumber string, sum int) (OrderProcessStatus, error)
	WithdrawList(ctx context.Context, userId int64) ([]WithdrawResponse, error)
}

type orderService struct {
	repo    repository.Storage
	accrual *accrualclient.DynHTTPClient
	logger  *slog.Logger
}

type OrderResponse struct {
	Number    string  `json:"number"`
	Status    string  `json:"status"`
	Points    float64 `json:"accrual,omitempty"`
	CreatedAt string  `json:"uploaded_at"`
}

type WithdrawResponse struct {
	Number    string  `json:"order"`
	Points    float64 `json:"sum,omitempty"`
	CreatedAt string  `json:"processed_at"`
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

	kopecks := int(math.Round(orderInfo.Accrual * 100))

	switch orderInfo.Status {
	case "PROCESSED":
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userId, orderNumber, orderInfo.Status, kopecks)
		if err != nil {
			return status, nil
		}

	//case "PROCESSING":
	case "INVALID":
		err := s.repo.UpdateOrderStatusAndUserBalance(ctx, userId, orderNumber, orderInfo.Status, 0)
		if err != nil {
			return status, nil
		}

	}

	return status, nil
}

func (s *orderService) OrderList(ctx context.Context, userId int64) ([]OrderResponse, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userId, model.ActionEarn)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	if len(orders) == 0 {
		return []OrderResponse{}, nil
	}

	listOrdersDTO := make([]OrderResponse, 0)
	for _, o := range orders {
		listOrdersDTO = append(listOrdersDTO, OrderResponse{
			Number:    o.Number,
			Status:    o.Status,
			Points:    float64(o.Points) / 100,
			CreatedAt: o.ChangedAt.Format(time.RFC3339),
		})
	}

	return listOrdersDTO, nil
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

func (s *orderService) WithdrawList(ctx context.Context, userId int64) ([]WithdrawResponse, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userId, model.ActionSpend)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	if len(orders) == 0 {
		return []WithdrawResponse{}, nil
	}

	listWithdrawDTO := make([]WithdrawResponse, 0)
	for _, o := range orders {
		listWithdrawDTO = append(listWithdrawDTO, WithdrawResponse{
			Number:    o.Number,
			Points:    float64(o.Points) / 100,
			CreatedAt: o.ChangedAt.Format(time.RFC3339),
		})
	}

	return listWithdrawDTO, nil
}
