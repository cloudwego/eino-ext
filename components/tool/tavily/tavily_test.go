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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				APIKey:      "test-api-key",
				ToolName:    "custom_tavily",
				ToolDesc:    "custom description",
				SearchDepth: SearchDepthAdvanced,
				Topic:       TopicNews,
				MaxResults:  5,
				Timeout:     10 * time.Second,
				MaxRetries:  5,
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			config: &Config{
				APIKey: "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &Config{
				ToolName: "tavily_search",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  &Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_validate_defaults(t *testing.T) {
	config := &Config{
		APIKey: "test-api-key",
	}

	err := config.validate()
	require.NoError(t, err)

	assert.Equal(t, defaultToolName, config.ToolName)
	assert.Equal(t, defaultToolDesc, config.ToolDesc)
	assert.Equal(t, defaultBaseURL, config.BaseURL)
	assert.Equal(t, SearchDepthBasic, config.SearchDepth)
	assert.Equal(t, TopicGeneral, config.Topic)
	assert.Equal(t, defaultMaxResults, config.MaxResults)
	assert.Equal(t, defaultTimeout, config.Timeout)
	assert.Equal(t, defaultMaxRetries, config.MaxRetries)
}

func TestNewTool(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey: "test-api-key",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "missing API key",
			config: &Config{
				ToolName: "tavily_search",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tool, err := NewTool(ctx, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tool)
			}
		})
	}
}

func TestTavilySearch_Search(t *testing.T) {
	// Create a mock server
	mockResp := `{
		"query": "test query",
		"answer": "This is the AI generated answer",
		"results": [
			{
				"title": "Test Title 1",
				"url": "https://example.com/1",
				"content": "Test content 1",
				"score": 0.95
			},
			{
				"title": "Test Title 2",
				"url": "https://example.com/2",
				"content": "Test content 2",
				"score": 0.85,
				"raw_content": "Raw content of the page"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var reqBody tavilyAPIRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "test-api-key", reqBody.APIKey)
		assert.Equal(t, "test query", reqBody.Query)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResp))
	}))
	defer server.Close()

	config := &Config{
		APIKey:        "test-api-key",
		BaseURL:       server.URL,
		IncludeAnswer: true,
		MaxResults:    10,
	}
	err := config.validate()
	require.NoError(t, err)

	ts, err := newTavilySearch(config)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := ts.Search(ctx, &SearchRequest{Query: "test query"})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test query", resp.Query)
	assert.Equal(t, "This is the AI generated answer", resp.Answer)
	assert.Len(t, resp.Results, 2)

	assert.Equal(t, "Test Title 1", resp.Results[0].Title)
	assert.Equal(t, "https://example.com/1", resp.Results[0].URL)
	assert.Equal(t, "Test content 1", resp.Results[0].Content)
	assert.Equal(t, 0.95, resp.Results[0].Score)

	assert.Equal(t, "Test Title 2", resp.Results[1].Title)
	assert.Equal(t, "Raw content of the page", resp.Results[1].RawContent)
}

func TestTavilySearch_Search_EmptyQuery(t *testing.T) {
	config := &Config{
		APIKey: "test-api-key",
	}
	err := config.validate()
	require.NoError(t, err)

	ts, err := newTavilySearch(config)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := ts.Search(ctx, &SearchRequest{Query: ""})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "search query is required")
}

func TestTavilySearch_Search_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	config := &Config{
		APIKey:     "invalid-api-key",
		BaseURL:    server.URL,
		MaxRetries: 1,
	}

	ts, err := newTavilySearch(config)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := ts.Search(ctx, &SearchRequest{Query: "test"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "tavily API error")
}

func TestTavilySearch_Search_WithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "server error"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"query": "test", "results": []}`))
	}))
	defer server.Close()

	config := &Config{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
	}

	require.NoError(t, config.validate())
	ts, err := newTavilySearch(config)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := ts.Search(ctx, &SearchRequest{Query: "test"})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 3, attempts)
}

func TestTavilySearch_Search_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		Timeout:    10 * time.Second,
		MaxRetries: 1,
	}

	err := config.validate()
	require.NoError(t, err)
	ts, err := newTavilySearch(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := ts.Search(ctx, &SearchRequest{Query: "test"})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestNewTavilySearch_WithProxy(t *testing.T) {
	config := &Config{
		APIKey:   "test-api-key",
		ProxyURL: "http://proxy.example.com:8080",
	}
	err := config.validate()
	require.NoError(t, err)

	ts, err := newTavilySearch(config)
	require.NoError(t, err)
	assert.NotNil(t, ts)
}

func TestNewTavilySearch_InvalidProxy(t *testing.T) {
	config := &Config{
		APIKey:   "test-api-key",
		ProxyURL: "://invalid-proxy-url",
	}
	err := config.validate()
	require.NoError(t, err)

	ts, err := newTavilySearch(config)
	assert.Error(t, err)
	assert.Nil(t, ts)
	assert.Contains(t, err.Error(), "failed to parse proxy URL")
}

func TestSearchDepthConstants(t *testing.T) {
	assert.Equal(t, SearchDepth("basic"), SearchDepthBasic)
	assert.Equal(t, SearchDepth("advanced"), SearchDepthAdvanced)
}

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, Topic("general"), TopicGeneral)
	assert.Equal(t, Topic("news"), TopicNews)
}
