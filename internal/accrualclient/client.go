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

	"github.com/hashicorp/go-retryablehttp"
	"github.com/mersikovs/gomart/internal/model"
	"golang.org/x/time/rate"
)

const (
	defaultRetryDelay            = 1 * time.Second
	maxRetryDelay                = 1 * time.Second
	maxLimit          rate.Limit = 10.0
	speedUpFactor     rate.Limit = 1.1
	slowDownFactor    rate.Limit = 2.0
	minLimit          rate.Limit = 1.0
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
	appCtx context.Context

	baseURL    string
	httpClient *retryablehttp.Client
	limiter    *rate.Limiter
	mu         sync.Mutex
	logger     *slog.Logger

	isLimit0      bool
	restoredLimit rate.Limit
}

// NewHTTPClient создает сконфигурированный экземпляр клиента для системы начислений.
func NewHTTPClient(ctx context.Context, baseURL string, timeout time.Duration, initialRPS int, log *slog.Logger) *DynHTTPClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3 // Максимум 3 повторные попытки
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.Logger = nil

	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		if err != nil {
			return true, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return false, nil
		}

		switch resp.StatusCode {
		case http.StatusServiceUnavailable: // 503
			return true, nil
		case http.StatusBadGateway: // 502
			return true, nil
		case http.StatusGatewayTimeout: // 504
			return true, nil

		default:
			return false, nil
		}
	}

	retryClient.HTTPClient = &http.Client{Timeout: timeout}

	return &DynHTTPClient{
		appCtx:     ctx,
		baseURL:    baseURL,
		httpClient: retryClient,
		limiter:    rate.NewLimiter(rate.Limit(initialRPS), initialRPS),
		logger:     log,
	}
}

// GetOrder реализует метод интерфейса AccrualClient.
func (c *DynHTTPClient) GetOrder(ctx context.Context, orderNumber string) (*model.Order, error) {

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.slowDown()
		return nil, fmt.Errorf("request failed after retries: %w", err)
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
		wait := defaultRetryDelay
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if parsedWait, err := parseRetryAfter(retryAfter); err == nil {
				wait = parsedWait
				if wait > maxRetryDelay {
					c.logger.Warn("429: Retry-After value is too large", "retry_after", retryAfter)
					wait = maxRetryDelay
				}
			} else {
				c.logger.Warn("429: invalid Retry-After, using default", "retry_after", retryAfter, "error", err)
			}
		}

		c.mu.Lock()
		if c.isLimit0 {
			c.mu.Unlock()
			return nil, fmt.Errorf("rate limited, already set limit 0, wait time")
		}

		c.restoredLimit = c.limiter.Limit()
		c.limiter.SetLimit(0)
		c.isLimit0 = true
		c.mu.Unlock()

		go func() {
			select {
			case <-time.After(wait):

				c.mu.Lock()
				c.limiter.SetLimit(c.restoredLimit)
				c.isLimit0 = false
				c.mu.Unlock()
				c.logger.Info("Linit 0 finished, limit restored", "limit", c.restoredLimit)

			case <-c.appCtx.Done():
				c.logger.Info("Cooldown interrupted by application shutdown")
				return
			}
		}()

		return nil, fmt.Errorf("server asked to retry after %v", wait)
	case http.StatusServiceUnavailable: // 503
		c.slowDown()
		return nil, fmt.Errorf("service unavailable")
	case http.StatusInternalServerError: // 500
		c.slowDown()
		return nil, fmt.Errorf("internal server error")
	case http.StatusNoContent:
		return nil, fmt.Errorf("order not registered")
	default:
		c.logger.Warn("unexpected HTTP status", "status", resp.StatusCode)
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
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

	if c.isLimit0 {
		return
	}

	currentLimit := c.limiter.Limit()
	newLimit := currentLimit * speedUpFactor

	if newLimit > maxLimit {
		newLimit = maxLimit
	}

	c.limiter.SetLimit(newLimit)
	c.limiter.SetBurst(max(1, int(newLimit)))
}

func (c *DynHTTPClient) slowDown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isLimit0 {
		return
	}

	currentLimit := c.limiter.Limit()
	newLimit := currentLimit / slowDownFactor

	if newLimit < minLimit {
		newLimit = minLimit
	}

	c.limiter.SetLimit(newLimit)
	c.limiter.SetBurst(max(1, int(newLimit)))
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
