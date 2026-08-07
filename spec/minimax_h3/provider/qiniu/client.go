/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 */

package qiniu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL        = "https://api.qnaigc.com"
	DefaultHTTPTimeout    = 120 * time.Second
	DefaultBaseRetryDelay = time.Second
	maxDebugBodyLen       = 4096
)

// Client is the HTTP client for the MiniMax-H3 FAL Queue API.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	apiKeyMu       sync.RWMutex
	apiKey         string
	maxRetries     int
	baseRetryDelay time.Duration
	debugLog       bool
	logger         *log.Logger
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the API base URL, primarily for tests.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithRetry enables retries for transient HTTP and network errors.
func WithRetry(maxRetries int, baseDelay time.Duration) ClientOption {
	return func(c *Client) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
		if baseDelay > 0 {
			c.baseRetryDelay = baseDelay
		}
	}
}

// WithDebugLog toggles response logging.
func WithDebugLog(enabled bool) ClientOption { return func(c *Client) { c.debugLog = enabled } }

// WithLogger sets the debug logger.
func WithLogger(logger *log.Logger) ClientOption { return func(c *Client) { c.logger = logger } }

// NewClient creates a MiniMax-H3 client. Empty apiKey falls back to
// QINIU_API_KEY for local tooling.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = os.Getenv("QINIU_API_KEY")
	}
	client := &Client{
		httpClient:     &http.Client{Timeout: DefaultHTTPTimeout},
		baseURL:        DefaultBaseURL,
		apiKey:         apiKey,
		baseRetryDelay: DefaultBaseRetryDelay,
		debugLog:       true,
		logger:         log.Default(),
	}
	for _, option := range opts {
		option(client)
	}
	return client
}

// ApiKey returns the current API key.
func (c *Client) ApiKey() string {
	c.apiKeyMu.RLock()
	defer c.apiKeyMu.RUnlock()
	return c.apiKey
}

// SetApiKey updates the API key at runtime.
func (c *Client) SetApiKey(apiKey string) {
	c.apiKeyMu.Lock()
	c.apiKey = apiKey
	c.apiKeyMu.Unlock()
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) logDebug(format string, args ...any) {
	if c.debugLog && c.logger != nil {
		c.logger.Printf("[qiniu-minimax-h3] "+format, args...)
	}
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError ||
		status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	apiKey := strings.TrimSpace(c.ApiKey())
	if apiKey == "" {
		return nil, errors.New("qiniu-minimax-h3: missing API key")
	}
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("qiniu-minimax-h3: marshal request: %w", err)
		}
	}
	fullURL := strings.TrimSuffix(c.baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		var requestBody io.Reader
		if len(bodyBytes) > 0 {
			requestBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, requestBody)
		if err != nil {
			return nil, fmt.Errorf("qiniu-minimax-h3: create request: %w", err)
		}
		req.Header.Set("Authorization", "Key "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("qiniu-minimax-h3: request failed: %w", err)
			if attempt < c.maxRetries {
				continue
			}
			return nil, lastErr
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("qiniu-minimax-h3: read response: %w", readErr)
			if attempt < c.maxRetries {
				continue
			}
			return nil, lastErr
		}
		c.logDebug("%s %s status=%d request_id=%s", method, path, resp.StatusCode, resp.Header.Get("X-Request-Id"))
		if c.debugLog && len(responseBody) > 0 {
			debugBody := string(responseBody)
			if len(debugBody) > maxDebugBodyLen {
				debugBody = debugBody[:maxDebugBodyLen] + "..."
			}
			c.logDebug("response body=%s", debugBody)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return responseBody, nil
		}
		lastErr = fmt.Errorf("qiniu-minimax-h3: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
		if retryableStatus(resp.StatusCode) && attempt < c.maxRetries {
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

// PostJSON sends a JSON POST request.
func (c *Client) PostJSON(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

// GetJSON sends a GET request.
func (c *Client) GetJSON(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}
