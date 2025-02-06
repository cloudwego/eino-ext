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

	Headers    map[string]string `json:"headers"`     // optional, default: map[string]string{}
	Timeout    time.Duration     `json:"timeout"`     // optional, default: 30 * time.Second
	ProxyURL   string            `json:"proxy_url"`   // optional, default: ""
	Cache      time.Duration     `json:"cache"`       // optional, default: 0 (disabled)
	MaxRetries int               `json:"max_retries"` // optional, default: 3
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
