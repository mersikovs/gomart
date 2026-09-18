// internal/worker/order_processor.go
package worker

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/repository"
)

type OrderProcessor struct {
	repo          repository.Storage
	accrualClient accrualclient.AccrualClient
	pool          *Pool
	interval      time.Duration
}

func NewOrderProcessor(
	repo repository.Storage,
	client accrualclient.AccrualClient,
	pool *Pool,
	interval time.Duration,
) *OrderProcessor {
	return &OrderProcessor{
		repo:          repo,
		accrualClient: client,
		pool:          pool,
		interval:      interval,
	}
}

func (p *OrderProcessor) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Order processor stopped")
			return
		case <-ticker.C:
			p.loadPendingOrders(ctx)
		}
	}
}

func (p *OrderProcessor) loadPendingOrders(ctx context.Context) {
	orders, err := p.repo.GetOrdersByStatus(ctx, "NEW", model.ActionEarn)
	if err != nil {
		slog.Error("Failed to get pending orders", "error", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	for _, order := range orders {
		p.pool.Submit(func(o model.Order) func(context.Context) error {
			return func(ctx context.Context) error {
				return p.processOrder(ctx, o)
			}
		}(order))
	}
}

func (p *OrderProcessor) processOrder(ctx context.Context, order model.Order) error {
	orderInfo, err := p.accrualClient.GetOrder(ctx, order.Number)
	if err != nil {
		return err
	}

	kopecks := int(math.Round(orderInfo.Accrual * 100))

	switch orderInfo.Status {
	case "PROCESSED":
		err := p.repo.UpdateOrderStatusAndUserBalance(ctx, order.UserID, order.Number, orderInfo.Status, kopecks)
		if err != nil {
			return err
		}

	case "PROCESSING":
	case "INVALID":
		err := p.repo.UpdateOrderStatusAndUserBalance(ctx, order.UserID, order.Number, orderInfo.Status, 0)
		if err != nil {
			return err
		}

	}
	return nil
}
