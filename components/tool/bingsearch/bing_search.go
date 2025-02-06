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

const (
	// Regions settings
	RegionUS = bingcore.RegionUS
	RegionGB = bingcore.RegionGB
	RegionCA = bingcore.RegionCA
	RegionAU = bingcore.RegionAU
	RegionDE = bingcore.RegionDE
	RegionFR = bingcore.RegionFR
	RegionCN = bingcore.RegionCN
	RegionHK = bingcore.RegionHK
	RegionTW = bingcore.RegionTW
	RegionJP = bingcore.RegionJP
	RegionKR = bingcore.RegionKR

	// SafeSearch settings
	SafeSearchOff      = bingcore.SafeSearchOff
	SafeSearchModerate = bingcore.SafeSearchModerate
	SafeSearchStrict   = bingcore.SafeSearchStrict

	// TimeRange settings
	TimeRangeDay   = bingcore.TimeRangeDay
	TimeRangeWeek  = bingcore.TimeRangeWeek
	TimeRangeMonth = bingcore.TimeRangeMonth
)

// Config represents the Bing search tool configuration.
type Config struct {
	ToolName string `json:"tool_name"` // optional, default is "bing_search"
	ToolDesc string `json:"tool_desc"` // optional, default is "search web for information by bing"

	APIKey     string              `json:"api_key"`     // required
	Region     bingcore.Region     `json:"region"`      // optional, default: ""
	MaxResults int                 `json:"max_results"` // optional, default: 10
	SafeSearch bingcore.SafeSearch `json:"safe_search"` // optional, default: bingcore.SafeSearchModerate
	TimeRange  bingcore.TimeRange  `json:"time_range"`  // optional, default: nil

	BingConfig *BingConfig `json:"bing_config"`
}

type BingConfig struct {
	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Default:
	//   Headers: map[string]string{
	//     "Ocp-Apim-Subscription-Key": "YOUR_API_KEY",
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; rv:11.0) like Gecko",
	//     "Accept-Language": "en-US",
	//   }
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
	// Example: 5 * time.Minute
	Cache time.Duration `json:"cache"` // default: 0 (disabled)

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	MaxRetries int `json:"max_retries"` // default: 3
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

	if c.BingConfig == nil {
		c.BingConfig = &BingConfig{
			Headers: make(map[string]string),
		}
	}

	if c.BingConfig.Headers == nil {
		c.BingConfig.Headers = make(map[string]string)
	}

	c.BingConfig.Headers["Ocp-Apim-Subscription-Key"] = c.APIKey

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
		Headers:    config.BingConfig.Headers,
		Timeout:    config.BingConfig.Timeout,
		ProxyURL:   config.BingConfig.ProxyURL,
		Cache:      config.BingConfig.Cache,
		MaxRetries: config.BingConfig.MaxRetries,
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
	Query string `json:"query" jsonschema_description:"The query to search the web for"`
	Page  int    `json:"page" jsonschema_description:"The page number to search for, default: 1"`
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
		Region:     s.config.Region,
		SafeSearch: s.config.SafeSearch,
		TimeRange:  s.config.TimeRange,
		Offset:     request.Page - 1,
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
