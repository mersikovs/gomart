package accrualclient

import (
	"context"
	"encoding/json"
	"fmt"
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

type OrderResponse struct {
	Order   string        `json:"order"`
	Status  accrualStatus `json:"status"`
	Accrual float64       `json:"accrual"`
}

type AccrualClient interface {
	GetOrder(ctx context.Context, orderNumber string) (*model.Order, error)
}

type DynHTTPClient struct {
	baseURL    string
	httpClient *http.Client
	limiter    *rate.Limiter
	mu         sync.Mutex
}

func NewHTTPClient(baseURL string, timeout time.Duration, initialRPS int) *DynHTTPClient {
	return &DynHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		limiter: rate.NewLimiter(rate.Limit(initialRPS), initialRPS),
	}
}

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
	defer resp.Body.Close()

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
