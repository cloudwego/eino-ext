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

// BingClient represents the Bing search client.
type BingClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
	timeout time.Duration
	cache   *cache
	config  *Config
}

// Config represents the Bing search client configuration.
type Config struct {
	Headers map[string]string `json:"headers"`

	Timeout time.Duration `json:"timeout"`

	ProxyURL string `json:"proxy_url"`

	Cache bool `json:"cache"`

	MaxRetries int `json:"max_retries"`
}

// New creates a new BingClient instance.
func New(config *Config) (*BingClient, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	c := &BingClient{
		client:  &http.Client{Timeout: config.Timeout},
		baseURL: searchURL,
		headers: config.Headers,
		timeout: config.Timeout,
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

	if config.Cache {
		c.cache = newCache(5 * time.Minute) // 5 minutes cache
	}

	return c, nil
}

// sendRequestWithRetry sends the request with retry logic.
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
			time.Sleep(time.Second) // Simple fixed one-second delay between retries
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

// Search sends a search request to Bing API and returns the search results.
func (b *BingClient) Search(ctx context.Context, params *SearchParams) ([]*SearchResult, error) {
	if params == nil {
		return nil, errors.New("params is nil")
	}

	err := params.validate()
	if err != nil {
		return nil, err
	}

	// Set default SafeSearch if not provided
	query := params.build()

	if b.cache != nil {
		params.cacheKey = params.getCacheKey()

		if results, ok := b.cache.get(params.cacheKey); ok {
			if response, ok := results.([]*SearchResult); ok {
				return response, nil
			}
		}
	}

	// Build query URL
	queryURL := fmt.Sprintf("%s?%s", b.baseURL, query.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	// Set default User-Agent if not provided
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	// Send request with retry
	results, err := b.sendRequestWithRetry(ctx, req, params)
	if err != nil {
		return nil, err
	}

	if params.Count > 0 && len(results) > params.Count {
		results = results[:params.Count]
	}

	if b.cache != nil && params.cacheKey != "" {
		b.cache.set(params.cacheKey, results)
	}

	return results, nil
}
