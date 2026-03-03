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

package duckduckgo

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type Config struct {
	ToolName string `json:"tool_name"` // default: duckduckgo_search
	ToolDesc string `json:"tool_desc"` // default: "search web for information by duckduckgo"

	Region     ddgsearch.Region     `json:"region"`      // default: "wt-wt"
	MaxResults int                  `json:"max_results"` // default: 10
	SafeSearch ddgsearch.SafeSearch `json:"safe_search"` // default: ddgsearch.SafeSearchModerate
	TimeRange  ddgsearch.TimeRange  `json:"time_range"`  // default: ddgsearch.TimeRangeAll

	DDGConfig *ddgsearch.Config `json:"ddg_config"`
}

// Deprecated: use NewTextSearchTool in V2 instead.
func NewTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	ddgs, err := newDDGS(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ddg search tool: %w", err)
	}

	searchTool, err := utils.InferTool(config.ToolName, config.ToolDesc, ddgs.Search)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}

	return searchTool, nil
}

// validate validates the configuration and sets default values if not provided.
func (conf *Config) validate() error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}

	if conf.ToolName == "" {
		conf.ToolName = "duckduckgo_search"
	}

	if conf.ToolDesc == "" {
		conf.ToolDesc = "search web for information by duckduckgo"
	}

	if conf.Region == "" {
		conf.Region = ddgsearch.RegionWT
	}

	if conf.MaxResults <= 0 {
		conf.MaxResults = 10
	}

	if conf.SafeSearch == "" {
		conf.SafeSearch = ddgsearch.SafeSearchOff
	}

	if conf.TimeRange == "" {
		conf.TimeRange = ddgsearch.TimeRangeAll
	}

	if conf.DDGConfig == nil {
		conf.DDGConfig = &ddgsearch.Config{}
	}

	if conf.DDGConfig.Timeout == 0 {
		conf.DDGConfig.Timeout = 30 * time.Second
	}

	if conf.DDGConfig.MaxRetries == 0 {
		conf.DDGConfig.MaxRetries = 3
	}

	return nil
}

func newDDGS(_ context.Context, config *Config) (*ddgs, error) {
	if config == nil {
		config = &Config{}
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	ddg, err := ddgsearch.New(config.DDGConfig)
	if err != nil {
		return nil, err
	}

	return &ddgs{
		config: config,
		ddg:    ddg,
	}, nil
}

type ddgs struct {
	config *Config
	ddg    *ddgsearch.DDGS
}

type SearchRequest struct {
	Query string `json:"query" jsonschema_description:"The query to search the web for"`
	Page  int    `json:"page" jsonschema_description:"The page number to search for, default: 1"`
}

type SearchResult struct {
	Title       string `json:"title" jsonschema_description:"The title of the search result"`
	Description string `json:"description" jsonschema_description:"The description of the search result"`
	Link        string `json:"link" jsonschema_description:"The link of the search result"`
}

type SearchResponse struct {
	Results []*SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

func (d *ddgs) Search(ctx context.Context, request *SearchRequest) (*SearchResponse, error) {
	results, err := d.ddg.Search(ctx, &ddgsearch.SearchParams{
		Query:      request.Query,
		Region:     d.config.Region,
		MaxResults: d.config.MaxResults,
		Page:       request.Page,
		SafeSearch: d.config.SafeSearch,
		TimeRange:  d.config.TimeRange,
	})
	if err != nil {
		return nil, err
	}

	searchResponse := &SearchResponse{
		Results: make([]*SearchResult, len(results.Results)),
	}

	for i, result := range results.Results {
		searchResponse.Results[i] = &SearchResult{
			Title:       result.Title,
			Description: result.Description,
			Link:        result.URL,
		}
	}

	return searchResponse, nil
}
