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

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "valid config",
			cfg: &Config{
				Headers: map[string]string{"User-Agent": "test"},
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid proxy",
			cfg: &Config{
				Proxy: "invalid://proxy",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("New() returned nil client")
			}
		})
	}
}

func TestDDGS_Search(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			// Return VQD token
			w.Write([]byte(`<script type="text/javascript">vqd="12345";</script>`))
		case "/d.js":
			// Return search results
			w.Write([]byte(`{
				"results": [
					{"t": "Test Result 1", "u": "http://example.com/1", "a": "Description 1"},
					{"t": "Test Result 2", "u": "http://example.com/2", "a": "Description 2"}
				],
				"noResults": false
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client, err := New(&Config{
		Headers: map[string]string{"User-Agent": "test"},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set test endpoints
	searchURL = server.URL + "/d.js"

	tests := []struct {
		name       string
		params     *SearchParams
		wantCount  int
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "valid search",
			params: &SearchParams{
				Query:      "test",
				MaxResults: 2,
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty query",
			params:    &SearchParams{},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:      "nil params",
			params:    nil,
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "max results limit",
			params: &SearchParams{
				Query:      "test",
				MaxResults: 1,
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := client.Search(context.Background(), tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if results == nil {
					t.Error("Search() returned nil results when error not expected")
					return
				}
				if len(results.Results) != tt.wantCount {
					t.Errorf("Search() got %d results, want %d", len(results.Results), tt.wantCount)
				}
			}
		})
	}
}

func TestDDGS_SearchCache(t *testing.T) {
	requestCount := 0

	// Create a test server that counts requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.URL.Path {
		case "/":
			// Return VQD token
			w.Write([]byte(`<script type="text/javascript">vqd="12345";</script>`))
		case "/d.js":
			// Return search results
			w.Write([]byte(`{
				"results": [
					{"t": "Test Result 1", "u": "http://example.com/1", "a": "Description 1"},
					{"t": "Test Result 2", "u": "http://example.com/2", "a": "Description 2"}
				],
				"noResults": false
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with cache enabled
	client, err := New(&Config{
		Headers: map[string]string{"User-Agent": "test"},
		Timeout: 5 * time.Second,
		Cache:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set test endpoints
	searchURL = server.URL + "/d.js"

	// First search request
	params := &SearchParams{
		Query:      "test query",
		MaxResults: 2,
	}
	response, err := client.Search(context.Background(), params)
	if err != nil {
		t.Fatalf("First search failed: %v", err)
	}

	results1 := response.Results
	if len(results1) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results1))
	}
	initialRequests := requestCount

	// Second search request with same parameters (should use cache)
	response2, err := client.Search(context.Background(), params)
	if err != nil {
		t.Fatalf("Second search failed: %v", err)
	}
	results2 := response2.Results
	if len(results2) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results2))
	}

	// Check if request count remained the same (indicating cache hit)
	if requestCount != initialRequests {
		t.Errorf("Cache not working: expected %d requests, got %d", initialRequests, requestCount)
	}

	// Verify results are identical
	for i := range results1 {
		if results1[i].URL != results2[i].URL {
			t.Errorf("Cache returned different results: expected %v, got %v", results1[i], results2[i])
		}
	}

	// Different query (should not use cache)
	params.Query = "different query"
	_, err = client.Search(context.Background(), params)
	if err != nil {
		t.Fatalf("Third search failed: %v", err)
	}
	if requestCount <= initialRequests {
		t.Error("Cache incorrectly used for different query")
	}

}
