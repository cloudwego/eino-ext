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
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "MyApp/1.0",
	//     "Accept-Language": "en-US",
	//   }
	// The "Ocp-Apim-Subscription-Key" header will automatic setting witch is required for Bing API.
	Headers map[string]string `json:"headers"`

	// Timeout specifies the maximum duration for a single request.
	// Default is 30 seconds if not specified.
	// Example: 5 * time.Second
	Timeout time.Duration `json:"timeout"` // default: 30 seconds

	// ProxyURL specifies the proxy server URL for all requests.
	// Supports HTTP, HTTPS, and SOCKS5 proxies.
	// Example values:
	//   - "http://proxy.example.com:8080"
	//   - "socks5://localhost:1080"
	//   - "tb" (special alias for Tor Browser)
	ProxyURL string `json:"proxy_url"`

	// Cache enables in-memory caching of search results.
	// When enabled, identical search requests will return cached results
	// for improved performance. Cache entries expire after 5 minutes.
	Cache bool `json:"cache"`

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	MaxRetries int `json:"max_retries"` // default: 3
}

// New creates a new BingClient instance.
func New(config *Config) (*BingClient, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
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

	// Validate search query
	if err := params.validate(); err != nil {
		return nil, err
	}

	// Set default SafeSearch if not provided
	query := params.build()

	// Check cache for existing results
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

	// Apply max results limit if specified
	if params.Count > 0 && len(results) > params.Count {
		results = results[:params.Count]
	}

	// Cache search results
	if b.cache != nil && params.cacheKey != "" {
		b.cache.set(params.cacheKey, results)
	}

	return results, nil
}
