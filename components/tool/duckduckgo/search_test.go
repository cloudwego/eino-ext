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
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
)

func MockDDGS() *mockey.Mocker {
	return mockey.Mock((*ddgsearch.DDGS).Search).To(func(ctx context.Context, request *ddgsearch.SearchParams) (*ddgsearch.SearchResponse, error) {
		if request == nil {
			return nil, fmt.Errorf("request is nil")
		}
		if request.Query == "" {
			return nil, fmt.Errorf("query is empty")
		}
		fmt.Println("mocked ddgs.Search", request)
		return &ddgsearch.SearchResponse{Results: []ddgsearch.SearchResult{
			{
				Title:       "test title",
				Description: "test description",
				URL:         "test link",
			},
			{
				Title:       "test title 2",
				Description: "test description 2",
				URL:         "test link 2",
			},
		}}, nil
	}).Build()
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "valid config",
			config: &Config{
				ToolName:   "custom_ddg",
				ToolDesc:   "custom description",
				Region:     "us-en",
				MaxResults: 20,
				DDGConfig: &ddgsearch.Config{
					Timeout:    20,
					Proxy:      "http://proxy.example.com",
					Cache:      true,
					MaxRetries: 5,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			if tt.config.ToolName == "" {
				assert.Equal(t, "duckduckgo_search", tt.config.ToolName)
			}
			if tt.config.ToolDesc == "" {
				assert.Equal(t, "search web for information by duckduckgo", tt.config.ToolDesc)
			}
			if tt.config.Region == "" {
				assert.Equal(t, "zh-CN", tt.config.Region)
			}
			if tt.config.MaxResults <= 0 {
				assert.Equal(t, 10, tt.config.MaxResults)
			}
		})
	}
}

func TestNewDDGS(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &Config{
				DDGConfig: &ddgsearch.Config{
					Timeout:    20,
					Proxy:      "http://proxy.example.com",
					Cache:      true,
					MaxRetries: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid timeout",
			config: &Config{
				DDGConfig: &ddgsearch.Config{
					Timeout: -1,
				},
			},
			wantErr: false, // won't error because Validate() fixes it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ddgs, err := newDDGS(ctx, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			mocker := MockDDGS()
			assert.NoError(t, err)
			assert.NotNil(t, ddgs)
			assert.NotNil(t, ddgs.config)
			assert.NotNil(t, ddgs.ddg)
			mocker.UnPatch()
		})
	}
}

func TestDDGS_Search(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		Region:     "zh-CN",
		MaxResults: 5,
	}

	ddgs, err := newDDGS(ctx, config)
	assert.NoError(t, err)
	mocker := MockDDGS()
	defer mocker.UnPatch()

	tests := []struct {
		name    string
		request *SearchRequest
		wantErr bool
	}{
		{
			name: "basic search",
			request: &SearchRequest{
				Query: "golang testing",
				Page:  1,
			},
			wantErr: false,
		},
		{
			name: "empty query",
			request: &SearchRequest{
				Query: "",
				Page:  1,
			},
			wantErr: true,
		},
		{
			name: "page number handling",
			request: &SearchRequest{
				Query: "test query",
				Page:  0,
			},
			wantErr: false,
		},
		{
			name: "search with max results",
			request: &SearchRequest{
				Query: "popular technology news",
				Page:  1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := ddgs.Search(ctx, tt.request)

			if err != nil {
				if err.Error() == "no results found" {
					// This is an acceptable case for any search
					t.Logf("no results found for query: %s", tt.request.Query)
					return
				}
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantErr {
				t.Error("expected error but got none")
				return
			}

			assert.NotNil(t, resp)
			assert.NotNil(t, resp.Results)
			if len(resp.Results) > 0 {
				result := resp.Results[0]
				assert.NotEmpty(t, result.Title)
				assert.NotEmpty(t, result.Link)
			}

			for _, result := range resp.Results {
				fmt.Printf("title: %s, description: %s, link: %s\n", result.Title, result.Description, result.Link)
			}
		})
	}
}

func TestNewTool(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		DDGConfig: &ddgsearch.Config{
			Timeout:    10,
			MaxRetries: 5,
		},
	}
	tool, err := NewTool(ctx, config)
	assert.NoError(t, err)
	assert.NotNil(t, tool)

	info, err := tool.Info(ctx)
	assert.Nil(t, err)

	doc, err := info.ParamsOneOf.ToJSONSchema()
	assert.Nil(t, err)
	assert.Equal(t, 2, doc.Properties.Len())
	for pair := doc.Properties.Oldest(); pair != nil; pair = pair.Next() {
		assert.NotEqual(t, "", pair.Value.Description)
	}
}
