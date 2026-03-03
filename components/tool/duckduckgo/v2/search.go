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

package duckduckgo

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	// ToolName is the name of the tool
	// Default: see defaultTextSearchToolName
	ToolName string `json:"tool_name"`
	// ToolDesc is the description of the tool
	// Default: see defaultTextSearchToolDesc
	ToolDesc string `json:"tool_desc"`

	// Timeout specifies the maximum duration for a single request.
	// Default: 30 seconds
	Timeout time.Duration

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// MaxResults limits the number of results returned
	// Default: 10
	MaxResults int `json:"max_results"`

	// Region is the geographical region for results
	// Default: RegionWT, means all regions
	// Reference: https://duckduckgo.com/duckduckgo-help-pages/settings/params
	Region Region `json:"region"`
}

func NewTextSearchTool(ctx context.Context, config *Config) (tool.InvokableTool, error) {
	if config == nil {
		config = &Config{}
	}

	name := config.ToolName
	if name == "" {
		name = defaultTextSearchToolName
	}
	desc := config.ToolDesc
	if desc == "" {
		desc = defaultTextSearchToolDesc
	}

	cli, err := buildClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create duckduckgo client: %w", err)
	}

	searchTool := utils.NewTool(getTextSearchSchema(name, desc), cli.TextSearch)

	return searchTool, nil
}

func NewSearch(ctx context.Context, config *Config) (Search, error) {
	return buildClient(ctx, config)
}

func getTextSearchSchema(toolName, toolDesc string) *schema.ToolInfo {
	sc := &jsonschema.Schema{
		Type:     string(schema.Object),
		Required: []string{"query"},
		Properties: orderedmap.New[string, *jsonschema.Schema](
			orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "query",
					Value: &jsonschema.Schema{
						Type:        string(schema.String),
						Description: "The user's search query. The query is required.",
					},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "time_range",
					Value: &jsonschema.Schema{
						Type:        string(schema.String),
						Description: "The time range of search results",
						Default:     "",
						OneOf: []*jsonschema.Schema{
							{

								Type:        string(schema.String),
								Enum:        []any{"d"},
								Description: "Search information from the past day",
							},
							{
								Type:        string(schema.String),
								Enum:        []any{"w"},
								Description: "Search information from the past week",
							},
							{
								Type:        string(schema.String),
								Enum:        []any{"m"},
								Description: "Search information from the past month",
							},
							{
								Type:        string(schema.String),
								Enum:        []any{"y"},
								Description: "Search information from the past year",
							},
							{
								Type:        string(schema.String),
								Enum:        []any{""},
								Description: "Search information at any time",
							},
						},
					},
				},
			),
		),
	}

	info := &schema.ToolInfo{
		Name:        toolName,
		Desc:        toolDesc,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(sc),
	}

	return info
}

func buildClient(_ context.Context, config *Config) (Search, error) {
	if config == nil {
		config = &Config{}
	}

	region := config.Region
	if region == "" {
		region = RegionWT
	}

	maxResults := config.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var httpCli *http.Client
	if config.HTTPClient != nil {
		httpCli = config.HTTPClient
	} else {
		httpCli = &http.Client{
			Timeout: timeout,
		}
	}

	return &client{
		httpCli:    httpCli,
		maxResults: maxResults,
		region:     region,
	}, nil
}
