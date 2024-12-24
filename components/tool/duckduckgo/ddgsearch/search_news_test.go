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
	"fmt"
	"testing"
	"time"
)

func TestDDGS_News(t *testing.T) {
	// Create a client with custom configuration for testing
	cfg := &Config{
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		params  *NewsParams
		want    func(*NewsResponse) error
		wantErr bool
	}{
		{
			name: "basic_search",
			params: &NewsParams{
				Query:      "technology news",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				MaxResults: 5,
			},
			want: func(resp *NewsResponse) error {
				if len(resp.Results) == 0 {
					return fmt.Errorf("expected results, got none")
				}
				for i, r := range resp.Results {
					if r.Title == "" {
						return fmt.Errorf("result %d has empty title", i)
					}
					if r.URL == "" {
						return fmt.Errorf("result %d has empty URL", i)
					}
					if r.Source == "" {
						return fmt.Errorf("result %d has empty source", i)
					}
					if r.Date == "" {
						return fmt.Errorf("result %d has empty date", i)
					}
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "empty_query",
			params: &NewsParams{
				Query: "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "search_with_max_results",
			params: &NewsParams{
				Query:      "technology",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				MaxResults: 2,
			},
			want: func(resp *NewsResponse) error {
				if len(resp.Results) > 2 {
					return fmt.Errorf("expected at most 2 results, got %d", len(resp.Results))
				}
				// Print sample results for debugging
				t.Logf("Sample results for %q:", "search with max results")
				for i, r := range resp.Results {
					t.Logf("  %d. %s (%s) - %s", i+1, r.Title, r.Source, r.Date)
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "search_with_time_range",
			params: &NewsParams{
				Query:      "artificial intelligence",
				Region:     RegionUS,
				SafeSearch: SafeSearchModerate,
				TimeRange:  TimeRangeDay,
				MaxResults: 3,
			},
			want: func(resp *NewsResponse) error {
				if len(resp.Results) == 0 {
					return fmt.Errorf("expected results, got none")
				}
				// Verify date is within the last day
				now := time.Now()
				for i, r := range resp.Results {
					date, err := time.Parse(time.RFC3339, r.Date)
					if err != nil {
						return fmt.Errorf("result %d has invalid date format: %s", i, r.Date)
					}
					if now.Sub(date) > 24*time.Hour {
						return fmt.Errorf("result %d date %s is older than 24 hours", i, r.Date)
					}
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "search_with_region",
			params: &NewsParams{
				Query:      "news",
				Region:     RegionJP,
				SafeSearch: SafeSearchModerate,
				MaxResults: 3,
			},
			want: func(resp *NewsResponse) error {
				if len(resp.Results) == 0 {
					return fmt.Errorf("expected results, got none")
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *NewsResponse
			var err error

			// Retry logic for flaky tests
			for retries := 0; retries < 3; retries++ {
				if retries > 0 {
					t.Logf("Retry %d for %s", retries, tt.name)
					time.Sleep(time.Second * time.Duration(retries))
				}

				resp, err = client.News(ctx, tt.params)

				if (err != nil) == tt.wantErr {
					break // Test passed
				}

				if err != nil {
					t.Logf("Retry %d failed: %v", retries+1, err)
					continue
				}

				if tt.want != nil {
					if err := tt.want(resp); err == nil {
						break // Test passed
					} else {
						t.Logf("Retry %d failed: %v", retries+1, err)
					}
				}
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("DDGS.News() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.want != nil {
				if err := tt.want(resp); err != nil {
					t.Errorf("DDGS.News() validation failed: %v", err)
				}
			}
		})

		// Add delay between tests to avoid rate limiting
		time.Sleep(time.Second)
	}
}
