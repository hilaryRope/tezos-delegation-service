package tzkt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

type Client interface {
	FetchDelegations(ctx context.Context, since time.Time, limit int) ([]Delegation, error)
}

type client struct {
	baseURL string
	http    *http.Client
	limiter *rate.Limiter
}

func NewClient(baseURL string, timeout time.Duration) Client {
	if baseURL == "" {
		baseURL = "https://api.tzkt.io/v1"
	}
	return &client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
		// Rate limit: 10 requests per second with burst of 5
		limiter: rate.NewLimiter(rate.Limit(10), 5),
	}
}

type Delegation struct {
	ID        int64     `json:"id"`
	Level     int64     `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Amount    int64     `json:"amount"`
	Sender    struct {
		Address string `json:"address"`
	} `json:"sender"`
}

func (c *client) FetchDelegations(ctx context.Context, since time.Time, limit int) ([]Delegation, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	u, err := url.Parse(c.baseURL + "/operations/delegations")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("timestamp.gt", since.UTC().Format(time.RFC3339))
	q.Set("sort.asc", "id")
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("status", "applied")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Retry with exponential backoff
	var resp *http.Response
	var lastErr error
	maxRetries := 3
	backoff := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, lastErr = c.http.Do(req)
		if lastErr != nil {
			continue // Retry on network error
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			err := resp.Body.Close()
			if err != nil {
				return nil, err
			}
			continue
		}

		break
	}

	if lastErr != nil {
		return nil, fmt.Errorf("http request failed after %d attempts: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tzkt: unexpected status %d", resp.StatusCode)
	}

	var out []Delegation
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}
