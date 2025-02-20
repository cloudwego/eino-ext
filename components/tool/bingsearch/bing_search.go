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

package bingsearch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/bingsearch/internal/bingcore"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Region string
type SafeSearch string
type TimeRange string

const (
	// Regions settings
	RegionUS Region = "en-US"
	RegionGB Region = "en-GB"
	RegionCA Region = "en-CA"
	RegionAU Region = "en-AU"
	RegionDE Region = "de-DE"
	RegionFR Region = "fr-FR"
	RegionCN Region = "zh-CN"
	RegionHK Region = "zh-HK"
	RegionTW Region = "zh-TW"
	RegionJP Region = "ja-JP"
	RegionKR Region = "ko-KR"

	// SafeSearch settings
	SafeSearchOff      SafeSearch = "Off"
	SafeSearchModerate SafeSearch = "Moderate"
	SafeSearchStrict   SafeSearch = "Strict"

	// TimeRange settings
	TimeRangeDay   TimeRange = "Day"
	TimeRangeWeek  TimeRange = "Week"
	TimeRangeMonth TimeRange = "Month"
)

// Config represents the Bing search tool configuration.
type Config struct {
	// Eino tool settings
	ToolName string `json:"tool_name"` // optional, default is "bing_search"
	ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by bing"

	// Bing search settings
	// APIKey The API key is required to access the Bing Web Search API.
	APIKey string `json:"api_key"`

	// Region specifies the Bing search region and is used to customize the search results for a specific country or language.
	// Optional, default: ""
	Region Region `json:"region"`

	// MaxResults specifies the maximum number of search results to return.
	// Optional, default: 10
	MaxResults int `json:"max_results"`

	// SafeSearch specifies the Bing search safe search setting.
	// Optional, default: SafeSearchModerate
	SafeSearch SafeSearch `json:"safe_search"`

	// TimeRange specifies the Bing search time range.
	// Optional, default: ""
	TimeRange TimeRange `json:"time_range"`

	// Bing client settings
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Optional, default: map[string]string{}
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; rv:11.0) like Gecko",
	//     "Accept-Language": "en-US",
	//   }
	Headers map[string]string `json:"headers"`

	// Timeout specifies the maximum duration for a single request.
	// Optional, default: 30 * time.Second
	// Example: 5 * time.Second
	Timeout time.Duration `json:"timeout"`

	// ProxyURL specifies the proxy server URL for all requests.
	// Supports HTTP, HTTPS, and SOCKS5 proxies.
	// Optional, default: ""
	// Example values:
	//   - "http://proxy.example.com:8080"
	//   - "socks5://localhost:1080"
	//   - "tb" (special alias for Tor Browser)
	ProxyURL string `json:"proxy_url"`

	// Cache enables in-memory caching of search results.
	// When enabled, identical search requests will return cached results
	// for improved performance. Cache entries expire after 5 minutes.
	// Optional, default: 0 (disabled)
	// Example: 5 * time.Minute
	Cache time.Duration `json:"cache"`

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Optional, default: 3
	MaxRetries int `json:"max_retries"`
}

// NewTool creates a new Bing search tool instance.
func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	bing, err := newBingSearch(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create bing search tool: %w", err)
	}

	searchTool, err := utils.InferTool(config.ToolName, config.ToolDesc, bing.Search)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}

	return searchTool, nil
}

// validate validates the Bing search tool configuration.
func (c *Config) validate() error {
	// Set default values
	if c.ToolName == "" {
		c.ToolName = "bing_search"
	}

	if c.ToolDesc == "" {
		c.ToolDesc = "search web for information by bing"
	}

	// Validate required fields
	if c.APIKey == "" {
		return errors.New("bing search tool config is missing API key")
	}

	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}

	c.Headers["Ocp-Apim-Subscription-Key"] = c.APIKey

	return nil
}

// bingSearch represents the Bing search tool.
type bingSearch struct {
	config *Config
	client *bingcore.BingClient
}

// newBingSearch creates a new Bing search client.
func newBingSearch(config *Config) (*bingSearch, error) {
	if config == nil {
		return nil, errors.New("bing search tool config is required")
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	bingConfig := &bingcore.Config{
		Headers:    config.Headers,
		Timeout:    config.Timeout,
		ProxyURL:   config.ProxyURL,
		Cache:      config.Cache,
		MaxRetries: config.MaxRetries,
	}

	client, err := bingcore.New(bingConfig)
	if err != nil {
		return nil, err
	}

	return &bingSearch{
		config: config,
		client: client,
	}, nil
}

type SearchRequest struct {
	Query  string `json:"query" jsonschema_description:"The query to search the web for"`
	Offset int    `json:"page" jsonschema_description:"The index of the first result to return, default is 0"`
}

type SearchResult struct {
	Title       string `json:"title" jsonschema_description:"The title of the search result"`
	URL         string `json:"url" jsonschema_description:"The link of the search result"`
	Description string `json:"description" jsonschema_description:"The description of the search result"`
}

type SearchResponse struct {
	Results []*SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

// Search searches the web for information.
func (s *bingSearch) Search(ctx context.Context, request *SearchRequest) (response *SearchResponse, err error) {
	// Search the web for information
	searchResults, err := s.client.Search(ctx, &bingcore.SearchParams{
		Query:      request.Query,
		Region:     bingcore.Region(s.config.Region),
		SafeSearch: bingcore.SafeSearch(s.config.SafeSearch),
		TimeRange:  bingcore.TimeRange(s.config.TimeRange),
		Offset:     request.Offset,
		Count:      s.config.MaxResults,
	})
	if err != nil {
		return nil, err
	}

	// Convert search results to search response
	results := make([]*SearchResult, 0, len(searchResults))
	for _, r := range searchResults {
		results = append(results, &SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
		})
	}

	return &SearchResponse{
		Results: results,
	}, nil
}
