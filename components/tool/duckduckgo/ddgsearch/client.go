/*
 * Copyright 2024 CloudWeGo Authors
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

package ddgsearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DDGS represents the DuckDuckGo search client.
// It handles all search-related operations including request configuration,
// caching, and result parsing.
//
// Use New() to create a new instance with proper configuration.
type DDGS struct {
	client  *http.Client
	headers map[string]string
	proxy   string
	timeout time.Duration
	cache   *cache
	config  *Config
}

// Config configures the DDGS client behavior.
// All fields are optional and will use sensible defaults if not provided.
type Config struct {
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "MyApp/1.0",
	//     "Accept-Language": "en-US",
	//   }
	Headers map[string]string

	// Proxy specifies the proxy server URL for all requests.
	// Supports HTTP, HTTPS, and SOCKS5 proxies.
	// Example values:
	//   - "http://proxy.example.com:8080"
	//   - "socks5://localhost:1080"
	//   - "tb" (special alias for Tor Browser)
	Proxy string

	// Timeout specifies the maximum duration for a single request.
	// Default is 30 seconds if not specified.
	// Example: 5 * time.Second
	Timeout time.Duration

	// Cache enables in-memory caching of search results.
	// When enabled, identical search requests will return cached results
	// for improved performance. Cache entries expire after 5 minutes.
	Cache bool

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Default is 3.
	MaxRetries int
}

// New creates a new DDGS client with the given configuration
func New(cfg *Config) (*DDGS, error) {
	if cfg == nil {
		cfg = &Config{
			Headers:    make(map[string]string),
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	d := &DDGS{
		client:  &http.Client{Timeout: cfg.Timeout},
		headers: cfg.Headers,
		proxy:   cfg.Proxy,
		timeout: cfg.Timeout,
		config:  cfg,
	}

	// Configure proxy if specified
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		// Validate proxy scheme
		switch proxyURL.Scheme {
		case "http", "https", "socks5":
			d.client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
		}
	}

	if cfg.Cache {
		d.cache = newCache(5 * time.Minute) // 5 minutes cache expiration
	}

	return d, nil
}

// sendRequestWithRetry sends the request with retry
func (d *DDGS) sendRequestWithRetry(ctx context.Context, req *http.Request, params *SearchParams) (*SearchResponse, error) {
	var resp *http.Response
	var err error
	var attempt int

	for attempt = 0; attempt <= d.config.MaxRetries; attempt++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err = d.client.Do(req)
		if err != nil {
			if attempt == d.config.MaxRetries {
				return nil, fmt.Errorf("failed to send request after retries: %w", err)
			}
			time.Sleep(time.Second) // Simple fixed 1 second delay between retries
			continue
		}

		// Check for rate limit response
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == d.config.MaxRetries {
				return nil, ErrRateLimit
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request duckduckgo failed with status code: %d, msg=%v",
			resp.StatusCode, string(body))
	}

	// Parse search response
	response, err := parseSearchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Check for no results
	if len(response.Results) == 0 {
		return nil, ErrNoResults
	}

	// Apply max results limit if specified
	if params.MaxResults > 0 && len(response.Results) > params.MaxResults {
		response.Results = response.Results[:params.MaxResults]
	}

	return response, nil
}

// getVQD retrieves the VQD token required for search requests
func (d *DDGS) getVQD(ctx context.Context, query string) (string, error) {
	endpoint := "https://duckduckgo.com"

	// Create request with query parameter
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameter
	q := req.URL.Query()
	q.Set("q", query)
	req.URL.RawQuery = q.Encode()

	// Set headers
	for k, v := range d.headers {
		req.Header.Set(k, v)
	}

	// Set default User-Agent if not provided
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	vqd := extractVQDToken(string(body))
	if vqd == "" {
		return "", fmt.Errorf("failed to extract VQD token")
	}

	return vqd, nil
}
