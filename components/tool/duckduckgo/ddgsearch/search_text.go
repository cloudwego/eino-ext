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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Search performs a search with the given parameters
func (d *DDGS) Search(ctx context.Context, params *SearchParams) (*SearchResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("search params cannot be nil")
	}

	if params.Query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Generate cache key if caching is enabled
	if d.cache != nil {
		params.cacheKey = params.getCacheKey()

		// Try to get from cache
		if cached, ok := d.cache.get(params.cacheKey); ok {
			if response, ok := cached.(*SearchResponse); ok {
				return response, nil
			}
		}
	}

	// Get VQD token
	vqd, err := d.getVQD(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to get vqd token: %w", err)
	}

	// Build search URL using SearchParams method
	searchURL := params.buildSearchURL(vqd)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request with retry
	response, err := d.sendRequestWithRetry(ctx, req, params)
	if err != nil {
		return nil, err
	}

	// Cache the response if caching is enabled
	if d.cache != nil && params.cacheKey != "" {
		d.cache.set(params.cacheKey, response)
	}

	// max results
	if params.MaxResults > 0 && len(response.Results) > params.MaxResults {
		response.Results = response.Results[:params.MaxResults]
	}

	return response, nil
}

// validate checks if the search parameters are valid
func (p *SearchParams) validate() error {
	if p.Query == "" {
		return NewSearchError("search query cannot be empty", nil)
	}
	if p.Page < 0 {
		return NewSearchError("page number cannot be negative", nil)
	}
	if p.MaxResults < 0 {
		return NewSearchError("max results cannot be negative", nil)
	}
	return nil
}

// buildSearchURL constructs the search URL with all necessary parameters
func (p *SearchParams) buildSearchURL(vqd string) string {
	// Use test endpoint if available
	endpoint := searchURL

	// Initialize URL parameters
	params := url.Values{}

	// Main search parameters
	params.Set("q", p.Query) // The search query text
	params.Set("vqd", vqd)   // Verification query ID, required by DuckDuckGo

	// Regional and language settings
	params.Set("kl", string(p.Region)) // Knowledge location - affects result relevance by region
	params.Set("l", string(p.Region))

	// Search behavior flags
	params.Set("ss", "1")   // Show snippets in results
	params.Set("sp", "1")   // Show preference cookies
	params.Set("sc", "1")   // Show category headers
	params.Set("o", "json") // Output format (JSON)

	// Optional parameters
	if p.SafeSearch != "" {
		params.Set("p", string(p.SafeSearch)) // Safe search level (strict/moderate/off)
	}
	if p.TimeRange != "" {
		params.Set("df", string(p.TimeRange)) // Date filter for results (d/w/m/y)
	}
	if p.Page > 1 {
		// Skip page results
		// Example: page 2 of max 10 results starts at result 10, page 3 at result 20, etc.
		pageSize := p.MaxResults
		if pageSize == 0 {
			pageSize = 10 // default page size
		}
		params.Set("s", strconv.Itoa((p.Page-1)*pageSize))
	}

	// Construct final URL
	return endpoint + "?" + params.Encode()
}

// getCacheKey generates a unique cache key for the search parameters
func (p *SearchParams) getCacheKey() string {
	// Use url.Values to consistently encode parameters
	v := url.Values{}
	v.Set("q", p.Query)
	v.Set("r", string(p.Region))
	v.Set("s", string(p.SafeSearch))
	v.Set("t", string(p.TimeRange))
	v.Set("p", strconv.Itoa(p.Page))

	return v.Encode()
}

// parseSearchResponse parses the search response from DuckDuckGo
func parseSearchResponse(body []byte) (*SearchResponse, error) {
	var response struct {
		Results []struct {
			Title       string `json:"t"`
			URL         string `json:"u"`
			Description string `json:"a"`
		} `json:"results"`
		NoResults bool `json:"noResults"`
	}

	err := json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if response.NoResults {
		return &SearchResponse{}, nil
	}

	results := make([]SearchResult, 0, len(response.Results))
	for _, r := range response.Results {
		if r.Description == "" && r.URL == "" && r.Title == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
		})
	}

	return &SearchResponse{
		Results: results,
	}, nil
}
