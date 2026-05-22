package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	StatusRegistered = "REGISTERED"
	StatusInvalid    = "INVALID"
	StatusProcessing = "PROCESSING"
	StatusProcessed  = "PROCESSED"
)

var ErrNoContent = errors.New("order not found")

type Result struct {
	Accrual *float64 `json:"accrual,omitempty"`
	Order   string   `json:"order"`
	Status  string   `json:"status"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Result(ctx context.Context, number string) (Result, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+number, nil)
	if err != nil {
		return Result{}, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result Result
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return Result{}, 0, err
		}
		return result, 0, nil
	case http.StatusNoContent:
		return Result{}, 0, ErrNoContent
	case http.StatusTooManyRequests:
		delay := time.Second
		if value := resp.Header.Get("Retry-After"); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		return Result{}, delay, fmt.Errorf("rate limited")
	default:
		_, _ = io.Copy(io.Discard, resp.Body)
		return Result{}, 0, fmt.Errorf("unexpected accrual status: %d", resp.StatusCode)
	}
}
