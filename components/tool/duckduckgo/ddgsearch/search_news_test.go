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

package ddgsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDDGS_News(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			if got := r.URL.Query().Get("q"); got == "" {
				t.Error("VQD request is missing the search query")
			}
			_, _ = w.Write([]byte(`<script>vqd="test-vqd";</script>`))
		case "/news.js":
			query := r.URL.Query()
			if got := query.Get("vqd"); got != "test-vqd" {
				t.Errorf("news request vqd = %q, want test-vqd", got)
			}
			if got := query.Get("t"); got != "n" {
				t.Errorf("news request t = %q, want n", got)
			}
			if query.Get("q") == "artificial intelligence" && query.Get("df") != string(TimeRangeDay) {
				t.Errorf("news request df = %q, want %q", query.Get("df"), TimeRangeDay)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{"date": 1704067200, "title": "First result", "excerpt": "first excerpt", "url": "https://example.com/first", "image": "https://example.com/first.png", "source": "Example"},
					{"date": 1704153600, "title": "Second result", "excerpt": "second excerpt", "url": "https://example.com/second", "image": "https://example.com/second.png", "source": "Example"},
					{"date": 1704240000, "title": "Third result", "excerpt": "third excerpt", "url": "https://example.com/third", "image": "https://example.com/third.png", "source": "Example"}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	useTestEndpoints(t, server.URL)

	client, err := New(&Config{
		Headers:    map[string]string{"User-Agent": "test"},
		Timeout:    time.Second,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name      string
		params    *NewsParams
		wantCount int
		wantErr   bool
	}{
		{
			name: "basic search",
			params: &NewsParams{
				Query:      "technology news",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				MaxResults: 5,
			},
			wantCount: 3,
		},
		{
			name:    "empty query",
			params:  &NewsParams{},
			wantErr: true,
		},
		{
			name: "max results",
			params: &NewsParams{
				Query:      "technology",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				MaxResults: 2,
			},
			wantCount: 2,
		},
		{
			name: "time range",
			params: &NewsParams{
				Query:      "artificial intelligence",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				TimeRange:  TimeRangeDay,
				MaxResults: 3,
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.News(context.Background(), tt.params)
			if (err != nil) != tt.wantErr {
				t.Fatalf("News() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got := len(response.Results); got != tt.wantCount {
				t.Fatalf("News() returned %d results, want %d", got, tt.wantCount)
			}
			if response.Results[0].Date != "2024-01-01T00:00:00Z" {
				t.Errorf("first result date = %q, want RFC3339 timestamp", response.Results[0].Date)
			}
			if response.Results[0].Title != "First result" || response.Results[0].Source != "Example" {
				t.Errorf("first result = %#v, want parsed fixture data", response.Results[0])
			}
		})
	}
}
