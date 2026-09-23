package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/model"
	"github.com/mersikovs/gomart/internal/repository"
)

// OrderProcessor обрабатывает заказы, взаимодействуя с репозиторием и системой начислений.
// Поля структуры содержат зависимости для выполнения бизнес-логики.
type OrderProcessor struct {
	repo          repository.Storage
	accrualClient accrualclient.AccrualClient
	pool          *Pool
	interval      time.Duration
}

// NewOrderProcessor создает OrderProcessor с заданными зависимостями и настройками.
// Возвращает указатель на сконфигурированный экземпляр.
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

// Run запускает основной цикл обработки заказов.
// Метод создает тикер с заданным интервалом, по сигналу которого происходит выборка ожидающих заказов (loadPendingOrders).
// При получении сигнала отмены через ctx.Done() останавливает тикер и завершает работу горутины.
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
	orders, err := p.repo.GetOrdersAwaitingUpdate(ctx)
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
	accruaInfo, err := p.accrualClient.GetOrder(ctx, order.Number)
	if err != nil {
		return err
	}

	if !order.Status.CanTransitionTo(accruaInfo.Status) {
		return fmt.Errorf("invalid status transition from %s to %s", order.Status, accruaInfo.Status)
	}

	return p.repo.UpdateOrderStatusAndUserBalance(ctx, order.UserID, order.Number, accruaInfo.Status, accruaInfo.Points)
}
