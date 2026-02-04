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

package tavily

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// SearchDepth represents the depth of the search.
type SearchDepth string

const (
	// SearchDepthBasic is the basic search depth.
	SearchDepthBasic SearchDepth = "basic"
	// SearchDepthAdvanced is the advanced search depth, which may take longer but returns more comprehensive results.
	SearchDepthAdvanced SearchDepth = "advanced"
)

// Topic represents the category of the search.
type Topic string

const (
	// TopicGeneral is for general web search.
	TopicGeneral Topic = "general"
	// TopicNews is for news-specific search.
	TopicNews Topic = "news"
)

const (
	defaultBaseURL    = "https://api.tavily.com"
	defaultToolName   = "tavily_search"
	defaultToolDesc   = "search web for information by tavily"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
	defaultMaxResults = 10
)

// Config represents the Tavily search tool configuration.
type Config struct {
	// Eino tool settings
	ToolName string `json:"tool_name"` // optional, default is "tavily_search"
	ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by tavily"

	// Tavily API settings
	// APIKey is required to access the Tavily Search API.
	APIKey string `json:"api_key"`

	// BaseURL specifies the Tavily API base URL.
	// Optional, default: "https://api.tavily.com"
	BaseURL string `json:"base_url"`

	// SearchDepth specifies the depth of the search.
	// Optional, default: SearchDepthBasic
	SearchDepth SearchDepth `json:"search_depth"`

	// Topic specifies the category of the search.
	// Optional, default: TopicGeneral
	Topic Topic `json:"topic"`

	// MaxResults specifies the maximum number of search results to return.
	// Optional, default: 10
	MaxResults int `json:"max_results"`

	// IncludeAnswer specifies whether to include an AI-generated answer in the response.
	// Optional, default: false
	IncludeAnswer bool `json:"include_answer"`

	// IncludeRawContent specifies whether to include the raw content of the search results.
	// Optional, default: false
	IncludeRawContent bool `json:"include_raw_content"`

	// IncludeDomains specifies a list of domains to specifically include in the search results.
	// Optional, default: nil
	IncludeDomains []string `json:"include_domains"`

	// ExcludeDomains specifies a list of domains to specifically exclude from the search results.
	// Optional, default: nil
	ExcludeDomains []string `json:"exclude_domains"`

	// HTTP client settings
	// Timeout specifies the maximum duration for a single request.
	// Optional, default: 30 * time.Second
	Timeout time.Duration `json:"timeout"`

	// ProxyURL specifies the proxy server URL for all requests.
	// Optional, default: ""
	ProxyURL string `json:"proxy_url"`

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Optional, default: 3
	MaxRetries int `json:"max_retries"`
}

// validate validates the configuration and sets default values if not provided.
func (c *Config) validate() error {
	if c.APIKey == "" {
		return errors.New("tavily search tool config is missing API key")
	}

	if c.ToolName == "" {
		c.ToolName = defaultToolName
	}

	if c.ToolDesc == "" {
		c.ToolDesc = defaultToolDesc
	}

	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}

	if c.SearchDepth == "" {
		c.SearchDepth = SearchDepthBasic
	}

	if c.Topic == "" {
		c.Topic = TopicGeneral
	}

	if c.MaxResults <= 0 {
		c.MaxResults = defaultMaxResults
	}

	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}

	if c.MaxRetries <= 0 {
		c.MaxRetries = defaultMaxRetries
	}

	return nil
}

// NewTool creates a new Tavily search tool instance.
func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	if config == nil {
		return nil, errors.New("tavily search tool config is required")
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	ts, err := newTavilySearch(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tavily search: %w", err)
	}

	searchTool, err := utils.InferTool(config.ToolName, config.ToolDesc, ts.Search)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}

	return searchTool, nil
}

// tavilySearch represents the Tavily search client.
type tavilySearch struct {
	config *Config
	client *http.Client
}

// newTavilySearch creates a new Tavily search client.
func newTavilySearch(config *Config) (*tavilySearch, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &tavilySearch{
		config: config,
		client: client,
	}, nil
}

// SearchRequest represents the search request from the tool invocation.
type SearchRequest struct {
	Query string `json:"query" jsonschema_description:"The query to search the web for"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title      string  `json:"title" jsonschema_description:"The title of the search result"`
	URL        string  `json:"url" jsonschema_description:"The URL of the search result"`
	Content    string  `json:"content" jsonschema_description:"The content snippet of the search result"`
	Score      float64 `json:"score,omitempty" jsonschema_description:"The relevance score of the search result"`
	RawContent string  `json:"raw_content,omitempty" jsonschema_description:"The raw content of the page if include_raw_content is enabled"`
}

// SearchResponse represents the search response.
type SearchResponse struct {
	Query   string          `json:"query,omitempty" jsonschema_description:"The original search query"`
	Answer  string          `json:"answer,omitempty" jsonschema_description:"AI-generated answer if include_answer is enabled"`
	Results []*SearchResult `json:"results" jsonschema_description:"The list of search results"`
}

// tavilyAPIRequest represents the request body for Tavily API.
type tavilyAPIRequest struct {
	APIKey            string   `json:"api_key"`
	Query             string   `json:"query"`
	SearchDepth       string   `json:"search_depth,omitempty"`
	Topic             string   `json:"topic,omitempty"`
	MaxResults        int      `json:"max_results,omitempty"`
	IncludeAnswer     bool     `json:"include_answer,omitempty"`
	IncludeRawContent bool     `json:"include_raw_content,omitempty"`
	IncludeDomains    []string `json:"include_domains,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
}

// tavilyAPIResponse represents the response from Tavily API.
type tavilyAPIResponse struct {
	Query   string `json:"query"`
	Answer  string `json:"answer,omitempty"`
	Results []struct {
		Title      string  `json:"title"`
		URL        string  `json:"url"`
		Content    string  `json:"content"`
		Score      float64 `json:"score"`
		RawContent string  `json:"raw_content,omitempty"`
	} `json:"results"`
}

// Search performs a web search using the Tavily API.
func (t *tavilySearch) Search(ctx context.Context, request *SearchRequest) (*SearchResponse, error) {
	if request.Query == "" {
		return nil, errors.New("search query is required")
	}

	apiReq := &tavilyAPIRequest{
		APIKey:            t.config.APIKey,
		Query:             request.Query,
		SearchDepth:       string(t.config.SearchDepth),
		Topic:             string(t.config.Topic),
		MaxResults:        t.config.MaxResults,
		IncludeAnswer:     t.config.IncludeAnswer,
		IncludeRawContent: t.config.IncludeRawContent,
		IncludeDomains:    t.config.IncludeDomains,
		ExcludeDomains:    t.config.ExcludeDomains,
	}

	apiResp, err := t.doRequestWithRetry(ctx, apiReq)
	if err != nil {
		return nil, err
	}

	// Convert API response to SearchResponse
	results := make([]*SearchResult, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		results = append(results, &SearchResult{
			Title:      r.Title,
			URL:        r.URL,
			Content:    r.Content,
			Score:      r.Score,
			RawContent: r.RawContent,
		})
	}

	return &SearchResponse{
		Query:   apiResp.Query,
		Answer:  apiResp.Answer,
		Results: results,
	}, nil
}

// doRequestWithRetry performs the HTTP request with retry logic.
func (t *tavilySearch) doRequestWithRetry(ctx context.Context, apiReq *tavilyAPIRequest) (*tavilyAPIResponse, error) {
	var lastErr error

	for i := 0; i < t.config.MaxRetries; i++ {
		resp, err := t.doRequest(ctx, apiReq)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if context is cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Wait before retry with exponential backoff
		if i < t.config.MaxRetries-1 {
			backoff := time.Duration(1<<uint(i)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRequest performs a single HTTP request to the Tavily API.
func (t *tavilySearch) doRequest(ctx context.Context, apiReq *tavilyAPIRequest) (*tavilyAPIResponse, error) {
	reqBody, err := sonic.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := t.config.BaseURL + "/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var apiResp tavilyAPIResponse
	if err := sonic.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &apiResp, nil
}
