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
	resp, err := c.generateOllama(ctx, model, prompt)
	if err == nil {
		return resp, nil
	}
	if !isUnsupportedEndpoint(err) {
		return "", err
	}
	return c.generateLlamaCPP(ctx, model, prompt)
}

func (c *Client) generateOllama(ctx context.Context, model, prompt string) (string, error) {
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
	vector, err := c.embedOllama(ctx, model, text)
	if err == nil {
		return vector, nil
	}
	if !isUnsupportedEndpoint(err) {
		return nil, err
	}
	return c.embedLlamaCPP(ctx, model, text)
}

func (c *Client) embedOllama(ctx context.Context, model, text string) ([]float32, error) {
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

func (c *Client) generateLlamaCPP(ctx context.Context, model, prompt string) (string, error) {
	body := map[string]any{
		"prompt":       prompt,
		"stream":       false,
		"cache_prompt": true,
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = model
	}
	var out struct {
		Content string `json:"content"`
		Error   any    `json:"error"`
	}
	if err := c.doJSON(ctx, "/completion", body, &out); err == nil {
		if out.Error != nil {
			return "", fmt.Errorf("llama.cpp completion error: %v", out.Error)
		}
		return out.Content, nil
	} else if !isUnsupportedEndpoint(err) {
		return "", err
	}

	body = map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	var chatOut struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	if err := c.doJSON(ctx, openAIPath(c.BaseURL, "/chat/completions"), body, &chatOut); err != nil {
		return "", err
	}
	if chatOut.Error != nil {
		return "", fmt.Errorf("llama.cpp chat completion error: %v", chatOut.Error)
	}
	if len(chatOut.Choices) == 0 {
		return "", fmt.Errorf("llama.cpp chat completion returned no choices")
	}
	if chatOut.Choices[0].Message.Content != "" {
		return chatOut.Choices[0].Message.Content, nil
	}
	return chatOut.Choices[0].Text, nil
}

func (c *Client) embedLlamaCPP(ctx context.Context, model, text string) ([]float32, error) {
	body := map[string]any{
		"content": text,
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
		Error     any       `json:"error"`
	}
	if err := c.doJSON(ctx, "/embedding", body, &out); err == nil {
		if out.Error != nil {
			return nil, fmt.Errorf("llama.cpp embedding error: %v", out.Error)
		}
		if len(out.Embedding) == 0 {
			return nil, fmt.Errorf("llama.cpp embedding returned no vector")
		}
		return out.Embedding, nil
	} else if !isUnsupportedEndpoint(err) {
		return nil, err
	}

	body = map[string]any{
		"model": model,
		"input": text,
	}
	var openAIOut struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := c.doJSON(ctx, openAIPath(c.BaseURL, "/embeddings"), body, &openAIOut); err != nil {
		return nil, err
	}
	if openAIOut.Error != nil {
		return nil, fmt.Errorf("llama.cpp embeddings error: %v", openAIOut.Error)
	}
	if len(openAIOut.Data) == 0 || len(openAIOut.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("llama.cpp embeddings returned no vectors")
	}
	return openAIOut.Data[0].Embedding, nil
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
			return fmt.Errorf("model request failed after %d attempt(s): %w", attempt, err)
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < attempts {
				time.Sleep(c.RetryBackoff * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("read model response failed after %d attempt(s): %w", attempt, readErr)
		}
		if resp.StatusCode >= 300 {
			statusErr := endpointError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
			lastErr = statusErr
			if shouldRetryStatus(resp.StatusCode) && attempt < attempts {
				time.Sleep(c.RetryBackoff * time.Duration(attempt))
				continue
			}
			return statusErr
		}
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("decode model response: %w; payload=%s", err, string(bodyBytes))
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("model request failed")
}

type endpointError struct {
	StatusCode int
	Body       string
}

func (e endpointError) Error() string {
	return fmt.Sprintf("model request failed (%d): %s", e.StatusCode, e.Body)
}

func isUnsupportedEndpoint(err error) bool {
	var endpointErr endpointError
	if !errors.As(err, &endpointErr) {
		return false
	}
	return endpointErr.StatusCode == http.StatusNotFound || endpointErr.StatusCode == http.StatusMethodNotAllowed
}

func openAIPath(baseURL, path string) string {
	if strings.HasSuffix(strings.TrimRight(baseURL, "/"), "/v1") {
		return path
	}
	return "/v1" + path
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
