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

package wikipedia

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/wikipedia/internal"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// Config is the configuration for the wikipedia search tool.
type Config struct {
	// BaseURL is the base url of the wikipedia api.
	// format: https://<language>.wikipedia.org/w/api.php
	// The URL language depends on the settings you have set for the Language field
	// Optional. Default: "https://en.wikipedia.org/w/api.php".
	BaseURL string

	// UserAgent is the user agent to use for the http client.
	// Optional but HIGHLY RECOMMENDED to override the default with your project's info.
	// It is recommended to follow Wikipedia's robot specification:
	// https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
	// Optional. Default: "eino (https://github.com/cloudwego/eino)"
	UserAgent string `json:"user_agent"`
	// DocMaxChars is the maximum number of characters as extract for returning in the page content.
	// If the content is longer than this, it will be truncated.
	// Optional. Default: 15s.
	DocMaxChars int `json:"doc_max_chars"`
	// Timeout is the maximum time to wait for the http client to return a response.
	// Optional. Default: 15s.
	Timeout time.Duration `json:"timeout"`
	// TopK is the number of search results to return.
	// Optional. Default: 3.
	TopK int `json:"top_k"`
	// MaxRedirect is the maximum number of redirects to follow.
	// Optional. Default: 3.
	MaxRedirect int `json:"max_redirect"`
	// Language is the language to use for the wikipedia search.
	// Optional. Default: "en".
	Language string `json:"language"`

	ToolName string `json:"tool_name"` // Optional. Default: "wikipedia_search".
	ToolDesc string `json:"tool_desc"` // Optional. Default: "this tool provides quick and efficient access to information from the Wikipedia"
}

// NewTool creates a new wikipedia search tool.
func NewTool(ctx context.Context, conf *Config) (tool.InvokableTool, error) {
	err := conf.validate()
	if err != nil {
		return nil, err
	}
	w, err := newWikipedia(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create wikipedia search tool: %w", err)
	}
	t, err := utils.InferTool(conf.ToolName, conf.ToolDesc, w.Search)
	if err != nil {
		return nil, fmt.Errorf("failed to infer tool: %w", err)
	}
	return t, nil
}

// validate validates the configuration and sets default values if not provided.
func (conf *Config) validate() error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}
	if conf.ToolName == "" {
		conf.ToolName = "wikipedia_search"
	}
	if conf.ToolDesc == "" {
		conf.ToolDesc = "this tool provides quick and efficient access to information from the Wikipedia"
	}
	if conf.UserAgent == "" {
		conf.UserAgent = "eino (https://github.com/cloudwego/eino)"
	}
	if conf.DocMaxChars <= 0 {
		conf.DocMaxChars = 2000
	}
	if conf.TopK <= 0 {
		conf.TopK = 3
	}
	if conf.Timeout <= 0 {
		conf.Timeout = 15 * time.Second
	}
	if conf.MaxRedirect <= 0 {
		conf.MaxRedirect = 3
	}
	if conf.Language == "" {
		conf.Language = "en"
	}
	if conf.BaseURL == "" {
		conf.BaseURL = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", conf.Language)
	}
	return nil
}

// newWikipedia creates a new wikipedia search tool.
func newWikipedia(_ context.Context, conf *Config) (*wikipedia, error) {
	c := internal.NewClient(
		internal.WithBaseURL(conf.BaseURL),
		internal.WithUserAgent(conf.UserAgent),
		internal.WithTopK(conf.TopK),
		internal.WithLanguage(conf.Language),
		internal.WithHTTPClient(
			&http.Client{
				Timeout: conf.Timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= conf.MaxRedirect {
						return internal.ErrTooManyRedirects
					}
					return nil
				}}),
	)
	return &wikipedia{
		conf:   conf,
		client: c,
	}, nil
}

// Search searches the web for the query and returns the search results.
func (w *wikipedia) Search(ctx context.Context, query SearchRequest) (*SearchResponse, error) {
	sr, err := w.client.Search(ctx, query.Query)
	if err != nil {
		return nil, err
	}
	if len(sr) == 0 {
		return nil, internal.ErrPageNotFound
	}
	res := make([]*Result, 0, len(sr))
	for _, search := range sr {
		pr, err := w.client.GetPage(ctx, search.Title)
		if err != nil {
			return nil, err
		}
		extract := ""
		if len(pr.Content) > w.conf.DocMaxChars {
			extract = pr.Content[:w.conf.DocMaxChars]
		} else {
			extract = pr.Content
		}
		res = append(res, &Result{
			Title:   pr.Title,
			URL:     pr.URL,
			Extract: extract,
			Snippet: search.Snippet,
		})
	}
	return &SearchResponse{Results: res}, nil
}

type wikipedia struct {
	conf   *Config
	client *internal.WikipediaClient
}

// Result is the page search result.
type Result struct {
	Title   string `json:"title" jsonschema_description:"The title of the search result"`
	URL     string `json:"url" jsonschema_description:"The url of the search result"`
	Extract string `json:"extract" jsonschema_description:"The extract of the search result"`
	Snippet string `json:"snippet" jsonschema_description:"The snippet of the search result"`
}

// SearchRequest is the search request.
type SearchRequest struct {
	Query string `json:"query" jsonschema_description:"The query to search the web for"`
}

// SearchResponse is the search response.
type SearchResponse struct {
	Results []*Result `json:"results" jsonschema_description:"The results of the search"`
}
