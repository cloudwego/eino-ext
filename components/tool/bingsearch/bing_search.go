package bingsearch

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch/bingcore"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Config struct {
	ToolName string `json:"tool_name"` // default: bing_search
	ToolDesc string `json:"tool_desc"` // default: "search web for information by bing"

	Region     bingcore.Region     `json:"region"`      // default: "wt-wt"
	MaxResults int                 `json:"max_results"` // default: 10
	SafeSearch bingcore.SafeSearch `json:"safe_search"` // default: bingcore.SafeSearchModerate
	TimeRange  bingcore.TimeRange  `json:"time_range"`  // default: nil

	BingConfig *bingcore.Config `json:"ddg_config"`
}

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

func (c *Config) validate() error {
	if c.ToolName == "" {
		c.ToolName = "bing_search"
	}

	if c.ToolDesc == "" {
		c.ToolDesc = "search web for information by bing"
	}

	if c.Region == "" {
		c.Region = bingcore.RegionUS
	}

	if c.MaxResults <= 0 {
		c.MaxResults = 10
	}

	if c.MaxResults > 50 {
		c.MaxResults = 50
	}

	if c.SafeSearch == "" {
		c.SafeSearch = bingcore.SafeSearchModerate
	}

	return nil
}

type bingSearch struct {
	config *Config
	client *bingcore.BingClient
}

func newBingSearch(config *Config) (*bingSearch, error) {
	if config == nil {
		config = &Config{}
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	client, err := bingcore.New(config.BingConfig)
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
	Offset int    `json:"offset" jsonschema_description:"Subtract 1 from the page number of the search results, default: 0"`
}

type SearchResult struct {
	Title       string `json:"title" jsonschema_description:"The title of the search result"`
	URL         string `json:"url" jsonschema_description:"The link of the search result"`
	Description string `json:"description" jsonschema_description:"The description of the search result"`
}

type SearchResponse struct {
	Results []*SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

func (s *bingSearch) Search(ctx context.Context, request *SearchRequest) (response *SearchResponse, err error) {
	searchResults, err := s.client.Search(ctx, &bingcore.SearchParams{
		Query:      request.Query,
		Region:     s.config.Region,
		SafeSearch: s.config.SafeSearch,
		TimeRange:  s.config.TimeRange,
		Offset:     request.Offset,
		Count:      s.config.MaxResults,
	})
	if err != nil {
		return nil, err
	}
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
