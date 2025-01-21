package bingcore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type BingClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
	timeout time.Duration
	config  *Config
}

type Config struct {
	Headers map[string]string `json:"headers"`

	Timeout time.Duration `json:"timeout"`

	ProxyURL string `json:"proxy_url"`

	MaxRetries int `json:"max_retries"`
}

func New(config *Config) (*BingClient, error) {
	if config == nil {
		config = &Config{
			Headers:    make(map[string]string),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	c := &BingClient{
		client:  &http.Client{Timeout: config.Timeout},
		baseURL: searchURL,
		headers: config.Headers,
		timeout: 0,
		config:  config,
	}

	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		// Validate proxy scheme
		switch proxyURL.Scheme {
		case "http", "https", "socks5":
			c.client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
		}
	}

	return c, nil
}

func (b *BingClient) sendRequestWithRetry(ctx context.Context, req *http.Request, params *SearchParams) ([]*SearchResult, error) {
	var resp *http.Response
	var err error
	var attempt int

	for attempt = 0; attempt <= b.config.MaxRetries; attempt++ {
		// Check context cancellation
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		resp, err = b.client.Do(req)
		if err != nil {
			if attempt == b.config.MaxRetries {
				return nil, fmt.Errorf("failed to send request after retries: %w", err)
			}
			time.Sleep(time.Second) // Simple fixed 1 second delay between retries
			continue
		}

		// Check for rate limit response
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == b.config.MaxRetries {
				return nil, errors.New("rate limit reached")
			}
			time.Sleep(time.Second)
			continue
		}

		break
	}

	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse search response
	response, err := parseSearchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Check for no results
	if len(response) == 0 {
		return nil, errors.New("no search results found")
	}

	// Apply max results limit if specified
	if params.Count > 0 && len(response) > params.Count {
		response = response[:params.Count]
	}

	return response, nil
}

func (b *BingClient) Search(ctx context.Context, params *SearchParams) ([]*SearchResult, error) {
	if params == nil {
		return nil, fmt.Errorf("search params cannot be nil")
	}

	if params.Query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	query := params.build()

	queryURL := fmt.Sprintf("%s?%s", b.baseURL, query)
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	results, err := b.sendRequestWithRetry(ctx, req, params)
	if err != nil {
		return nil, err
	}

	return results, nil
}
