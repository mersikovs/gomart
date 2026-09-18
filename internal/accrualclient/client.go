package accrualclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type OrderResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

type AccrualClient interface {
	GetOrder(ctx context.Context, orderNumber string) (*OrderResponse, error)
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

func (c *DynHTTPClient) GetOrder(ctx context.Context, orderNumber string) (*OrderResponse, error) {

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

	return &orderResp, nil
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

	waitDuration := t.Sub(time.Now().UTC())

	if waitDuration < 0 {
		waitDuration = 0
	}
	const maxServerWait = 10 * time.Minute
	if waitDuration > maxServerWait {
		waitDuration = maxServerWait
	}

	return waitDuration, nil
}
