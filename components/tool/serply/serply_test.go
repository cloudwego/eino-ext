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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	mockWebResponse = `{
  "query": "q=eino&num=2",
  "results": [
    {"title": "GitHub - cloudwego/eino", "link": "https://github.com/cloudwego/eino", "description": "LLM application development framework in Golang", "position": 1, "result_type": "organic"},
    {"title": "Eino: Overview | CloudWeGo", "link": "https://www.cloudwego.io/docs/eino/overview/", "description": "Eino aims to be the ultimate LLM application development framework in Golang", "position": 2, "result_type": "organic"}
  ],
  "ads": [], "related_questions": [], "knowledge_graph": ""
}`
	mockNewsResponse = `{
  "entries": [
    {"title": "Eino goes open source - CloudWeGo", "link": "https://news.google.com/rss/articles/abc", "published": "Mon, 24 Aug 2026 15:58:07 GMT", "source": {"href": "https://www.cloudwego.io", "title": "CloudWeGo"}, "summary": "<a href=\"https://news.google.com/rss/articles/abc\">Eino goes open source</a>"}
  ],
  "feed": {"title": "eino - Google News"}
}`
	mockScholarResponse = `{
  "articles": [
    {"title": "Attention Is All You Need", "link": "https://doi.org/10.48550/arXiv.1706.03762", "description": "A Vaswani, N Shazeer - NeurIPS, 2017", "author": {"names": "A Vaswani, N Shazeer - NeurIPS, 2017", "authors": [{"name": "Ashish Vaswani", "link": "https://openalex.org/A1"}, {"name": "Noam Shazeer", "link": "https://openalex.org/A2"}]}, "extras": {"citations": {"count": 120000, "link": "https://api.openalex.org/works?filter=cites:W1"}}, "doc": {"link": "https://arxiv.org/pdf/1706.03762", "type": "PDF"}}
  ],
  "results": []
}`
)

func TestConfig_validate(t *testing.T) {
	tests := []struct {
		name    string
		conf    *Config
		wantErr bool
		check   func(t *testing.T, conf *Config)
	}{
		{
			name:    "nil config",
			conf:    nil,
			wantErr: true,
		},
		{
			name:    "missing api key",
			conf:    &Config{},
			wantErr: true,
		},
		{
			name:    "invalid search type",
			conf:    &Config{APIKey: "key", SearchType: "images"},
			wantErr: true,
		},
		{
			name: "defaults",
			conf: &Config{APIKey: "key"},
			check: func(t *testing.T, conf *Config) {
				assert.Equal(t, "serply_search", conf.ToolName)
				assert.Equal(t, "search the web for information by serply", conf.ToolDesc)
				assert.Equal(t, SearchTypeWeb, conf.SearchType)
				assert.Equal(t, defaultBaseURL, conf.BaseURL)
				assert.Equal(t, 10, conf.Num)
				assert.NotNil(t, conf.Headers)
				assert.Equal(t, 30*time.Second, conf.Timeout)
				assert.NotNil(t, conf.HttpClient)
				assert.Equal(t, 30*time.Second, conf.HttpClient.Timeout)
			},
		},
		{
			name: "news defaults",
			conf: &Config{APIKey: "key", SearchType: SearchTypeNews},
			check: func(t *testing.T, conf *Config) {
				assert.Equal(t, "search google news for recent articles by serply", conf.ToolDesc)
			},
		},
		{
			name: "scholar defaults",
			conf: &Config{APIKey: "key", SearchType: SearchTypeScholar},
			check: func(t *testing.T, conf *Config) {
				assert.Equal(t, "search google scholar for academic papers by serply", conf.ToolDesc)
			},
		},
		{
			name: "custom values are kept and num is capped",
			conf: &Config{
				APIKey:     "key",
				ToolName:   "web_search",
				ToolDesc:   "custom desc",
				BaseURL:    "http://localhost:8080",
				Num:        500,
				Timeout:    time.Second,
				HttpClient: &http.Client{Timeout: 2 * time.Second},
			},
			check: func(t *testing.T, conf *Config) {
				assert.Equal(t, "web_search", conf.ToolName)
				assert.Equal(t, "custom desc", conf.ToolDesc)
				assert.Equal(t, "http://localhost:8080", conf.BaseURL)
				assert.Equal(t, maxResultsLimit, conf.Num)
				assert.Equal(t, 2*time.Second, conf.HttpClient.Timeout)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conf.validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.check != nil {
				tt.check(t, tt.conf)
			}
		})
	}
}

func TestNewTool(t *testing.T) {
	ctx := context.Background()

	_, err := NewTool(ctx, &Config{})
	assert.Error(t, err)

	tl, err := NewTool(ctx, &Config{APIKey: "key"})
	assert.NoError(t, err)
	assert.NotNil(t, tl)

	info, err := tl.Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "serply_search", info.Name)

	doc, err := info.ParamsOneOf.ToJSONSchema()
	assert.NoError(t, err)
	assert.Equal(t, 2, doc.Properties.Len())
	assert.Equal(t, []string{"query"}, doc.Required)
	for pair := doc.Properties.Oldest(); pair != nil; pair = pair.Next() {
		assert.NotEqual(t, "", pair.Value.Description)
	}
}

// newMockServer serves body for the expected path and records the last request.
func newMockServer(t *testing.T, status int, body string, last *http.Request) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*last = *r
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestSerplySearch_Search(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		searchType SearchType
		body       string
		req        *SearchRequest
		wantPath   string
		wantNum    string
		wantLen    int
		check      func(t *testing.T, resp *SearchResponse)
	}{
		{
			name:     "web search",
			body:     mockWebResponse,
			req:      &SearchRequest{Query: "eino"},
			wantPath: "/v1/search/",
			wantNum:  "10",
			wantLen:  2,
			check: func(t *testing.T, resp *SearchResponse) {
				assert.Equal(t, "eino", resp.Query)
				assert.Equal(t, "GitHub - cloudwego/eino", resp.Results[0].Title)
				assert.Equal(t, "https://github.com/cloudwego/eino", resp.Results[0].Link)
				assert.Equal(t, "LLM application development framework in Golang", resp.Results[0].Description)
				assert.Empty(t, resp.Results[0].Source)
				assert.Nil(t, resp.Results[0].Authors)
			},
		},
		{
			name:     "request num overrides config and trims results",
			body:     mockWebResponse,
			req:      &SearchRequest{Query: "eino", Num: 1},
			wantPath: "/v1/search/",
			wantNum:  "1",
			wantLen:  1,
		},
		{
			name:       "news search",
			searchType: SearchTypeNews,
			body:       mockNewsResponse,
			req:        &SearchRequest{Query: "eino"},
			wantPath:   "/v1/news/",
			wantNum:    "10",
			wantLen:    1,
			check: func(t *testing.T, resp *SearchResponse) {
				assert.Equal(t, "Eino goes open source - CloudWeGo", resp.Results[0].Title)
				assert.Equal(t, "https://news.google.com/rss/articles/abc", resp.Results[0].Link)
				assert.Equal(t, "CloudWeGo", resp.Results[0].Source)
				assert.Equal(t, "Mon, 24 Aug 2026 15:58:07 GMT", resp.Results[0].Published)
			},
		},
		{
			name:       "scholar search",
			searchType: SearchTypeScholar,
			body:       mockScholarResponse,
			req:        &SearchRequest{Query: "attention"},
			wantPath:   "/v1/scholar/",
			wantNum:    "10",
			wantLen:    1,
			check: func(t *testing.T, resp *SearchResponse) {
				assert.Equal(t, "Attention Is All You Need", resp.Results[0].Title)
				assert.Equal(t, "https://doi.org/10.48550/arXiv.1706.03762", resp.Results[0].Link)
				assert.Equal(t, []string{"Ashish Vaswani", "Noam Shazeer"}, resp.Results[0].Authors)
				assert.Equal(t, 120000, resp.Results[0].CitedBy)
			},
		},
		{
			name:     "empty results",
			body:     `{"results": []}`,
			req:      &SearchRequest{Query: "nothing"},
			wantPath: "/v1/search/",
			wantNum:  "10",
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var last http.Request
			srv := newMockServer(t, http.StatusOK, tt.body, &last)
			defer srv.Close()

			s, err := newSerplySearch(&Config{
				APIKey:     "test-key",
				BaseURL:    srv.URL,
				SearchType: tt.searchType,
				Headers:    map[string]string{"X-Proxy-Location": "US"},
			})
			assert.NoError(t, err)

			resp, err := s.Search(ctx, tt.req)
			assert.NoError(t, err)
			assert.Len(t, resp.Results, tt.wantLen)

			assert.Equal(t, tt.wantPath, last.URL.Path)
			assert.Equal(t, tt.req.Query, last.URL.Query().Get("q"))
			assert.Equal(t, tt.wantNum, last.URL.Query().Get("num"))
			assert.Equal(t, "test-key", last.Header.Get("X-Api-Key"))
			assert.Equal(t, "US", last.Header.Get("X-Proxy-Location"))
			assert.Equal(t, defaultUserAgent, last.Header.Get("User-Agent"))
			assert.Equal(t, "application/json", last.Header.Get("Accept"))

			if tt.check != nil {
				tt.check(t, resp)
			}
		})
	}
}

func TestSerplySearch_Search_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("empty query", func(t *testing.T) {
		s, err := newSerplySearch(&Config{APIKey: "key"})
		assert.NoError(t, err)
		_, err = s.Search(ctx, &SearchRequest{Query: "  "})
		assert.EqualError(t, err, "query is required")
		_, err = s.Search(ctx, nil)
		assert.EqualError(t, err, "query is required")
	})

	t.Run("non-200 status", func(t *testing.T) {
		var last http.Request
		srv := newMockServer(t, http.StatusUnauthorized, `{"detail":"Invalid API key"}`, &last)
		defer srv.Close()

		s, err := newSerplySearch(&Config{APIKey: "bad", BaseURL: srv.URL})
		assert.NoError(t, err)
		_, err = s.Search(ctx, &SearchRequest{Query: "eino"})
		assert.ErrorContains(t, err, "status=401")
		assert.ErrorContains(t, err, "Invalid API key")
	})

	t.Run("malformed body", func(t *testing.T) {
		var last http.Request
		srv := newMockServer(t, http.StatusOK, `not json`, &last)
		defer srv.Close()

		s, err := newSerplySearch(&Config{APIKey: "key", BaseURL: srv.URL})
		assert.NoError(t, err)
		_, err = s.Search(ctx, &SearchRequest{Query: "eino"})
		assert.ErrorContains(t, err, "failed to parse search results")
	})

	t.Run("context cancelled", func(t *testing.T) {
		var last http.Request
		srv := newMockServer(t, http.StatusOK, mockWebResponse, &last)
		defer srv.Close()

		s, err := newSerplySearch(&Config{APIKey: "key", BaseURL: srv.URL})
		assert.NoError(t, err)

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, err = s.Search(cancelled, &SearchRequest{Query: "eino"})
		assert.ErrorContains(t, err, "request failed")
	})
}

func TestTool_InvokableRun(t *testing.T) {
	ctx := context.Background()

	var last http.Request
	srv := newMockServer(t, http.StatusOK, mockWebResponse, &last)
	defer srv.Close()

	tl, err := NewTool(ctx, &Config{APIKey: "key", BaseURL: srv.URL, Num: 2})
	assert.NoError(t, err)

	args, err := json.Marshal(&SearchRequest{Query: "eino"})
	assert.NoError(t, err)

	out, err := tl.InvokableRun(ctx, string(args))
	assert.NoError(t, err)

	var resp SearchResponse
	assert.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "eino", resp.Query)
	assert.Len(t, resp.Results, 2)
	assert.Equal(t, "2", last.URL.Query().Get("num"))

	_, err = tl.InvokableRun(ctx, `{"query": ""}`)
	assert.Error(t, err)
}
