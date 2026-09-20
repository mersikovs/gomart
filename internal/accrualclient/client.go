// Package accrualclient предоставляет клиент для взаимодействия со внешней системой расчета бонусов.
package accrualclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mersikovs/gomart/internal/model"
	"golang.org/x/time/rate"
)

type accrualStatus string

const (
	accrualStatusRegistered accrualStatus = "REGISTERED"
	accrualStatusInvalid    accrualStatus = "INVALID"
	accrualStatusProcessing accrualStatus = "PROCESSING"
	accrualStatusProcessed  accrualStatus = "PROCESSED"
)

// OrderResponse представляет структуру JSON-ответа от внешнего API системы начислений.
type OrderResponse struct {
	// Order — номер заказа во внешнем формате. Должен совпадать с запрошенным номером.
	Order string `json:"order"`

	// Status — текущий статус обработки заказа на стороне партнерской системы.
	Status accrualStatus `json:"status"`

	// Accrual — количество начисленных бонусных баллов. Может быть дробным числом.
	Accrual float64 `json:"accrual"`
}

// AccrualClient определяет контракт для получения информации о начислениях по заказу.
type AccrualClient interface {
	// GetOrder запрашивает актуальный статус и сумму начисления для указанного номера заказа.
	// Возвращает обновленную модель *model.Order или ошибку связи/парсинга.
	GetOrder(ctx context.Context, orderNumber string) (*model.Order, error)
}

// DynHTTPClient — реализация клиента для внешней системы начислений.
type DynHTTPClient struct {
	baseURL    string
	httpClient *http.Client
	limiter    *rate.Limiter
	mu         sync.Mutex
	logger     *slog.Logger
}

// NewHTTPClient создает сконфигурированный экземпляр клиента для системы начислений.
func NewHTTPClient(baseURL string, timeout time.Duration, initialRPS int, log *slog.Logger) *DynHTTPClient {
	return &DynHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		limiter: rate.NewLimiter(rate.Limit(initialRPS), initialRPS),
		logger:  log,
	}
}

// GetOrder реализует метод интерфейса AccrualClient.
func (c *DynHTTPClient) GetOrder(ctx context.Context, orderNumber string) (*model.Order, error) {

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.slowDown()
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error("failed to close response body", "error", err)
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		c.speedUp()
	case http.StatusTooManyRequests:
		c.slowDown()
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if wait, err := parseRetryAfter(retryAfter); err == nil {
				const safetyCap = 5 * time.Minute
				if wait > safetyCap {
					wait = safetyCap
				}

				c.limiter.SetLimit(0)
				c.mu.Lock()

				select {
				case <-ctx.Done():
				case <-time.After(wait):
				}
				c.mu.Unlock()
				return nil, fmt.Errorf("server asked to retry after %s", wait)
			}
		}
		return nil, fmt.Errorf("rate limited by external service")
	case http.StatusServiceUnavailable: // 503
		c.slowDown()
		return nil, fmt.Errorf("service unavailable")
	case http.StatusInternalServerError: // 500
		c.slowDown()
		return nil, fmt.Errorf("internal server error")
	case http.StatusNoContent:
		c.slowDown()
		return nil, fmt.Errorf("order not registrate")
	}

	var orderResp OrderResponse

	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var internalStatus model.OrderStatus
	switch orderResp.Status {
	case accrualStatusRegistered:
		internalStatus = model.OrderStatusProcessing
	case accrualStatusProcessing:
		internalStatus = model.OrderStatusProcessing
	case accrualStatusInvalid:
		internalStatus = model.OrderStatusInvalid
	case accrualStatusProcessed:
		internalStatus = model.OrderStatusProcessed
	default:
		return nil, fmt.Errorf("unknown external status: %s", orderResp.Status)
	}

	kopecks := int(math.Round(orderResp.Accrual * 100))

	return &model.Order{
		Number: orderResp.Order,
		Status: internalStatus,
		Points: kopecks,
	}, nil
}

func (c *DynHTTPClient) speedUp() {
	c.mu.Lock()
	defer c.mu.Unlock()

	currentLimit := c.limiter.Limit()
	initialLimit := rate.Limit(10)

	newLimit := currentLimit * 1.1
	if newLimit > initialLimit {
		newLimit = initialLimit
	}
	c.limiter.SetLimit(newLimit)
	c.limiter.SetBurst(int(newLimit))
}

func (c *DynHTTPClient) slowDown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	currentLimit := c.limiter.Limit()
	newLimit := currentLimit / 2
	if newLimit < 1 {
		newLimit = 1
	}
	c.limiter.SetLimit(newLimit)
	c.limiter.SetBurst(int(newLimit))
}

func parseRetryAfter(headerValue string) (time.Duration, error) {

	if seconds, err := strconv.Atoi(strings.TrimSpace(headerValue)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second, nil
	}

	t, err := http.ParseTime(headerValue)
	if err != nil {
		return 0, err
	}

	waitDuration := max(t.Sub(time.Now().UTC()), 0)
	const maxServerWait = 10 * time.Minute
	if waitDuration > maxServerWait {
		waitDuration = maxServerWait
	}

	return waitDuration, nil
}
