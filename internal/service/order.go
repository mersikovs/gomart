package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/repository"
)

type OrderProcessStatus int

const (
	OrderStatusAlreadyAdded OrderProcessStatus = iota
	OrderStatusAdded
)

var ErrOrderAlreadyProcessedByOther = errors.New("order already processed by another user") // → 409

type OrderService interface {
	RegisterOrder(ctx context.Context, userId int64, orderNumber string) (OrderProcessStatus, error)
	OrderList(ctx context.Context, userId int64) ([]OrderDTO, error)
}

type orderService struct {
	repo   repository.Storage
	logger *slog.Logger
}

type OrderDTO struct {
	Number    string  `json:"number"`
	Status    string  `json:"status"`
	Points    float64 `json:"accrual,omitempty"`
	CreatedAt string  `json:"uploaded_at"`
}

func NewOrderService(repo repository.Storage, log *slog.Logger) OrderService {
	return &orderService{
		repo:   repo,
		logger: log,
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

	_, err = s.repo.CreateOrder(ctx, userId, orderNumber, model.ActionEarn)
	if err != nil {
		return status, fmt.Errorf("error CreateOrder: %w", err)
	}

	return status, nil
}

func (s *orderService) OrderList(ctx context.Context, userId int64) ([]OrderDTO, error) {
	orders, err := s.repo.GetOrdersByUser(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("error GetOrdersByUser: %w", err)
	}

	if len(orders) == 0 {
		return []OrderDTO{}, nil
	}

	listOrdersDTO := make([]OrderDTO, 0)
	for _, o := range orders {
		listOrdersDTO = append(listOrdersDTO, OrderDTO{
			Number:    o.Number,
			Status:    o.Status,
			Points:    float64(o.Points),
			CreatedAt: o.ChangedAt.Format(time.RFC3339),
		})
	}

	return listOrdersDTO, nil
}
