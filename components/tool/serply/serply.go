/*
 * Copyright 2026 CloudWeGo Authors
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

package serply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// SearchType selects which Serply endpoint the tool queries.
type SearchType string

const (
	// SearchTypeWeb queries Google web search results.
	SearchTypeWeb SearchType = "search"
	// SearchTypeNews queries Google News.
	SearchTypeNews SearchType = "news"
	// SearchTypeScholar queries Google Scholar.
	SearchTypeScholar SearchType = "scholar"
)

const (
	defaultBaseURL   = "https://api.serply.io"
	defaultUserAgent = "eino (https://github.com/cloudwego/eino)"
	maxResultsLimit  = 100
)

// Config is the configuration for the Serply search tool.
// Get an API key at https://serply.io and see https://serply.io/docs for the API reference.
type Config struct {
	// Eino tool settings
	ToolName string `json:"tool_name"` // optional, default is "serply_search"
	ToolDesc string `json:"tool_desc"` // optional, default depends on SearchType

	// APIKey is required to access the Serply API. It is sent as the X-Api-Key header.
	APIKey string `json:"api_key"`

	// BaseURL is the base URL of the Serply API.
	// Optional, default: "https://api.serply.io"
	BaseURL string `json:"base_url"`

	// SearchType selects the endpoint: SearchTypeWeb, SearchTypeNews or SearchTypeScholar.
	// Optional, default: SearchTypeWeb
	SearchType SearchType `json:"search_type"`

	// Num is the default number of results to return, between 1 and 100.
	// The request may override it.
	// Optional, default: 10
	Num int `json:"num"`

	// Headers specifies custom HTTP headers to be sent with each request,
	// e.g. "User-Agent" or "X-Proxy-Location" to localize results.
	// Optional, default: map[string]string{}
	Headers map[string]string `json:"headers"`

	// Timeout specifies the maximum duration for a single request.
	// Ignored when HttpClient is set.
	// Optional, default: 30 * time.Second
	Timeout time.Duration `json:"timeout"`

	// HttpClient specifies the custom HTTP client to be used.
	// Optional, default: &http.Client{Timeout: Timeout}
	HttpClient *http.Client `json:"http_client"`
}

// NewTool creates a new Serply search tool.
func NewTool(ctx context.Context, conf *Config) (tool.InvokableTool, error) {
	s, err := newSerplySearch(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create serply search tool: %w", err)
	}

	t, err := utils.InferTool(conf.ToolName, conf.ToolDesc, s.Search)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}

	return t, nil
}

// validate validates the configuration and sets default values if not provided.
func (conf *Config) validate() error {
	if conf == nil {
		return errors.New("serply search tool config is required")
	}

	if conf.APIKey == "" {
		return errors.New("serply search tool config is missing API key")
	}

	if conf.SearchType == "" {
		conf.SearchType = SearchTypeWeb
	}

	switch conf.SearchType {
	case SearchTypeWeb, SearchTypeNews, SearchTypeScholar:
	default:
		return fmt.Errorf("search_type must be one of: %v", []SearchType{SearchTypeWeb, SearchTypeNews, SearchTypeScholar})
	}

	if conf.ToolName == "" {
		conf.ToolName = "serply_search"
	}

	if conf.ToolDesc == "" {
		switch conf.SearchType {
		case SearchTypeNews:
			conf.ToolDesc = "search google news for recent articles by serply"
		case SearchTypeScholar:
			conf.ToolDesc = "search google scholar for academic papers by serply"
		default:
			conf.ToolDesc = "search the web for information by serply"
		}
	}

	if conf.BaseURL == "" {
		conf.BaseURL = defaultBaseURL
	}

	if conf.Num <= 0 {
		conf.Num = 10
	}
	if conf.Num > maxResultsLimit {
		conf.Num = maxResultsLimit
	}

	if conf.Headers == nil {
		conf.Headers = make(map[string]string)
	}

	if conf.Timeout <= 0 {
		conf.Timeout = 30 * time.Second
	}

	if conf.HttpClient == nil {
		conf.HttpClient = &http.Client{Timeout: conf.Timeout}
	}

	return nil
}

type serplySearch struct {
	conf *Config
}

func newSerplySearch(conf *Config) (*serplySearch, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}

	return &serplySearch{conf: conf}, nil
}

// SearchRequest is the search request.
type SearchRequest struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"The query to search for"`
	Num   int    `json:"num,omitempty" jsonschema_description:"Number of results to return, between 1 and 100. Defaults to the configured value"`
}

// SearchResult is a single search result.
type SearchResult struct {
	Title       string   `json:"title" jsonschema_description:"The title of the search result"`
	Link        string   `json:"link" jsonschema_description:"The url of the search result"`
	Description string   `json:"description,omitempty" jsonschema_description:"The snippet of the search result"`
	Source      string   `json:"source,omitempty" jsonschema_description:"The publisher of a news result"`
	Published   string   `json:"published,omitempty" jsonschema_description:"The publish time of a news result"`
	Authors     []string `json:"authors,omitempty" jsonschema_description:"The authors of a scholar result"`
	CitedBy     int      `json:"cited_by,omitempty" jsonschema_description:"The citation count of a scholar result"`
}

// SearchResponse is the search response.
type SearchResponse struct {
	Query   string          `json:"query" jsonschema_description:"The query of the search"`
	Results []*SearchResult `json:"results" jsonschema_description:"The results of the search"`
}

// Search sends the query to the Serply API and returns the simplified results.
func (s *serplySearch) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil || strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}

	num := s.conf.Num
	if req.Num > 0 {
		num = req.Num
	}
	if num > maxResultsLimit {
		num = maxResultsLimit
	}

	params := url.Values{}
	params.Set("q", req.Query)
	params.Set("num", strconv.Itoa(num))

	endpoint := fmt.Sprintf("%s/v1/%s/?%s", strings.TrimRight(s.conf.BaseURL, "/"), s.conf.SearchType, params.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", defaultUserAgent)
	for k, v := range s.conf.Headers {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("X-Api-Key", s.conf.APIKey)

	resp, err := s.conf.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("serply %s request failed: %w", s.conf.SearchType, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serply %s request failed: status=%d, body=%s", s.conf.SearchType, resp.StatusCode, truncate(body, 200))
	}

	var raw rawResponse
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	results := raw.simplify(s.conf.SearchType)
	if len(results) > num {
		results = results[:num]
	}

	return &SearchResponse{Query: req.Query, Results: results}, nil
}

// rawResponse covers the fields this tool reads from the three Serply endpoints:
// web search returns "results", news returns "entries" and scholar returns "articles".
type rawResponse struct {
	Results []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		Description string `json:"description"`
	} `json:"results"`
	Entries []struct {
		Title     string `json:"title"`
		Link      string `json:"link"`
		Published string `json:"published"`
		Source    struct {
			Title string `json:"title"`
		} `json:"source"`
	} `json:"entries"`
	Articles []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		Description string `json:"description"`
		Author      struct {
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"author"`
		Extras struct {
			Citations struct {
				Count int `json:"count"`
			} `json:"citations"`
		} `json:"extras"`
	} `json:"articles"`
}

func (r *rawResponse) simplify(searchType SearchType) []*SearchResult {
	switch searchType {
	case SearchTypeNews:
		results := make([]*SearchResult, 0, len(r.Entries))
		for _, e := range r.Entries {
			results = append(results, &SearchResult{
				Title:     e.Title,
				Link:      e.Link,
				Source:    e.Source.Title,
				Published: e.Published,
			})
		}
		return results
	case SearchTypeScholar:
		results := make([]*SearchResult, 0, len(r.Articles))
		for _, a := range r.Articles {
			authors := make([]string, 0, len(a.Author.Authors))
			for _, au := range a.Author.Authors {
				authors = append(authors, au.Name)
			}
			results = append(results, &SearchResult{
				Title:       a.Title,
				Link:        a.Link,
				Description: a.Description,
				Authors:     authors,
				CitedBy:     a.Extras.Citations.Count,
			})
		}
		return results
	default:
		results := make([]*SearchResult, 0, len(r.Results))
		for _, item := range r.Results {
			results = append(results, &SearchResult{
				Title:       item.Title,
				Link:        item.Link,
				Description: item.Description,
			})
		}
		return results
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
