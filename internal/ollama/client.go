package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL      string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	client       *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return NewClientWithRetry(baseURL, timeout, 0, 0)
}

func NewClientWithRetry(baseURL string, timeout time.Duration, maxRetries int, retryBackoff time.Duration) *Client {
	if retryBackoff <= 0 {
		retryBackoff = 5 * time.Second
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Timeout:      timeout,
		MaxRetries:   maxRetries,
		RetryBackoff: retryBackoff,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	var out struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := c.doJSON(ctx, "/api/generate", body, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama generate error: %s", out.Error)
	}
	return out.Response, nil
}

func (c *Client) Embed(ctx context.Context, model, text string) ([]float32, error) {
	body := map[string]any{
		"model": model,
		"input": text,
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	if err := c.doJSON(ctx, "/api/embed", body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama embed error: %s", out.Error)
	}
	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embed returned no vectors")
	}
	return out.Embeddings[0], nil
}

func (c *Client) doJSON(ctx context.Context, path string, requestBody any, out any) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	attempts := c.MaxRetries + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if shouldRetryRequestError(ctx, err) && attempt < attempts {
				time.Sleep(c.RetryBackoff * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("ollama request failed after %d attempt(s): %w", attempt, err)
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < attempts {
				time.Sleep(c.RetryBackoff * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("read ollama response failed after %d attempt(s): %w", attempt, readErr)
		}
		if resp.StatusCode >= 300 {
			statusErr := fmt.Errorf("ollama request failed (%d): %s", resp.StatusCode, string(bodyBytes))
			lastErr = statusErr
			if shouldRetryStatus(resp.StatusCode) && attempt < attempts {
				time.Sleep(c.RetryBackoff * time.Duration(attempt))
				continue
			}
			return statusErr
		}
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("decode ollama response: %w; payload=%s", err, string(bodyBytes))
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("ollama request failed")
}

func shouldRetryStatus(code int) bool {
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500
}

func shouldRetryRequestError(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	// connection reset/refused and other transient transport errors.
	return true
}
