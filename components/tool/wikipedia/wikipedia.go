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
	"github.com/cloudwego/eino-ext/components/tool/wikipedia/wikipediaclient"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"net/http"
	"time"
)

// Config is the configuration for the wikipedia search tool.
type Config struct {
	// baseUrl is the base url of the wikipedia api.
	// format: https://<language>.wikipedia.org/w/api.php
	baseUrl string // default: "https://en.wikipedia.org/w/api.php"

	// UserAgent is the user agent to use for the http client.
	// It is recommended to follow Wikipedia's robot specification:
	// https://en.wikipedia.org/robots.txt
	UserAgent string `json:"user_agent"` // default: "eino (https://github.com/cloudwego/eino)"
	// DocMaxChars is the maximum number of characters as extract for returning in the page content.
	// If the content is longer than this, it will be truncated.
	DocMaxChars int `json:"doc_max_chars"` // default: 2000
	// Timeout is the maximum time to wait for the http client to return a response.
	Timeout time.Duration `json:"timeout"` // default: 15s
	// TopK is the number of search results to return.
	TopK int `json:"top_k"` // default: 3
	// MaxRedirect is the maximum number of redirects to follow.
	MaxRedirect int `json:"max_redirect"` // default: 3
	// Language is the language to use for the wikipedia search.
	Language string `json:"language"` // default: "en"

	ToolName string `json:"tool_name"` // default: "wikipedia"
	ToolDesc string `json:"tool_desc"` // default: "wikipedia search tool"
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
		conf.ToolName = "wikipedia"
	}
	if conf.ToolDesc == "" {
		conf.ToolDesc = "wikipedia search tool"
	}
	if conf.baseUrl == "" {
		conf.baseUrl = fmt.Sprintf("https://%s.wikipedia.org/w/api.php", conf.Language)
	}
	if conf.UserAgent == "" {
		conf.UserAgent = "eino (https://github.com/cloudwego/eino)"
	}
	if conf.DocMaxChars <= 0 {
		conf.DocMaxChars = 2000
	}
	if conf.TopK <= 0 {
		conf.TopK = 4
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
	return nil
}

// newWikipedia creates a new wikipedia search tool.
func newWikipedia(_ context.Context, conf *Config) (*wikipedia, error) {
	c := wikipediaclient.NewClient(
		wikipediaclient.WithLanguage(conf.Language),
		wikipediaclient.WithUserAgent(conf.UserAgent),
		wikipediaclient.WithTopK(conf.TopK),
		wikipediaclient.WithHTTPClient(
			&http.Client{
				Timeout: conf.Timeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= conf.MaxRedirect {
						return wikipediaclient.ErrTooManyRedirects
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
		return nil, wikipediaclient.ErrPageNotFound
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
		})
	}
	return &SearchResponse{Results: res}, nil
}

type wikipedia struct {
	conf   *Config
	client *wikipediaclient.WikipediaClient
}

type Result struct {
	Title   string `json:"title" jsonschema_description:"The title of the search result"`
	URL     string `json:"url" jsonschema_description:"The url of the search result"`
	Extract string `json:"extract" jsonschema_description:"The extract of the search result"`
}

type SearchRequest struct {
	Query string `json:"query" jsonschema_description:"The query to search the web for"`
}

type SearchResponse struct {
	Results []*Result `json:"results" jsonschema_description:"The results of the search"`
}
