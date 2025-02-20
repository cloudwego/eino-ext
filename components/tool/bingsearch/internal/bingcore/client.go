/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	cache   *Cache
	config  *Config
}

// Config represents the Bing search client configuration.
type Config struct {
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Default:
	//   Headers: map[string]string{
	//     "Ocp-Apim-Subscription-Key": "YOUR_API_KEY",
	//   }
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; rv:11.0) like Gecko",
	//     "Accept-Language": "en-US",
	//   }
	Headers map[string]string `json:"headers"`

	// Timeout specifies the maximum duration for a single request.
	// Default is 30 seconds if not specified.
	// Default: 30 seconds
	// Example: 5 * time.Second
	Timeout time.Duration `json:"timeout"`

	// ProxyURL specifies the proxy server URL for all requests.
	// Supports HTTP, HTTPS, and SOCKS5 proxies.
	// Default: ""
	// Example values:
	//   - "http://proxy.example.com:8080"
	//   - "socks5://localhost:1080"
	//   - "tb" (special alias for Tor Browser)
	ProxyURL string `json:"proxy_url"`

	// Cache enables in-memory caching of search results.
	// When enabled, identical search requests will return cached results
	// for improved performance. Cache entries expire after 5 minutes.
	// Default: 0 (disabled)
	// Example: 5 * time.Minute
	Cache time.Duration `json:"cache"`

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Default: 3
	MaxRetries int `json:"max_retries"`
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

	if config.Cache > 0 {
		c.cache = newCache(config.Cache)
	}

	return c, nil
}

// sendRequestWithRetry sends the request with retry logic.
func (b *BingClient) sendRequestWithRetry(ctx context.Context, req *http.Request) ([]*searchResult, error) {
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

	return response, nil
}

// Search sends a search request to Bing API and returns the search results.
func (b *BingClient) Search(ctx context.Context, params *SearchParams) ([]*searchResult, error) {
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

		if results, ok := b.cache.Get(params.cacheKey); ok {
			return results, nil
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
	results, err := b.sendRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache search results
	if b.cache != nil && params.cacheKey != "" {
		b.cache.Set(params.cacheKey, results)
	}

	return results, nil
}
