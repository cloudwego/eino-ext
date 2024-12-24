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
	"io"
	"net/http"
	"net/url"
	"time"
)

// News performs a DuckDuckGo news search with the given parameters.
func (d *DDGS) News(ctx context.Context, params *NewsParams) (*NewsResponse, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Get vqd token
	vqd, err := d.getVQD(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to get vqd: %w", err)
	}

	// Prepare safe search parameter
	safeSearchMap := map[SafeSearch]string{
		SafeSearchStrict:   "1",
		SafeSearchModerate: "-1",
		SafeSearchOff:      "-2",
	}

	// Build query parameters
	queryParams := url.Values{
		"l":     {string(params.Region)},
		"o":     {"json"},
		"noamp": {"1"},
		"q":     {params.Query},
		"vqd":   {vqd},
		"p":     {safeSearchMap[params.SafeSearch]},
		"t":     {"n"}, // Ensure we're requesting news
	}

	if params.TimeRange != "" {
		queryParams.Set("df", string(params.TimeRange))
	}

	maxResults := params.MaxResults
	if maxResults <= 0 || maxResults > 120 {
		maxResults = 30 // Default to first page
	}

	var allResults []NewsResult
	seenURLs := make(map[string]bool)

	// Fetch results in batches of 30
	for offset := 0; offset < maxResults; offset += 30 {
		queryParams.Set("s", fmt.Sprintf("%d", offset))

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, newsURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set query parameters
		req.URL.RawQuery = queryParams.Encode()

		// Set headers
		for k, v := range d.headers {
			req.Header.Set(k, v)
		}

		// Ensure we have a User-Agent header
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		}

		// Set additional required headers
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Referer", "https://duckduckgo.com/")
		req.Header.Set("Authority", "duckduckgo.com")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")

		// Send request with retry
		var resp *http.Response
		var lastErr error
		for retries := 0; retries < 3; retries++ {
			if retries > 0 {
				time.Sleep(time.Second * time.Duration(retries))
			}

			resp, lastErr = d.client.Do(req)
			if lastErr == nil && resp.StatusCode == http.StatusOK {
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		if lastErr != nil {
			return nil, fmt.Errorf("failed to send request after retries: %w", lastErr)
		}
		if resp == nil {
			return nil, fmt.Errorf("no response received after retries")
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Debug the response
		if len(body) == 0 {
			return nil, fmt.Errorf("empty response body")
		}

		// Try to parse the response
		var raw rawNewsResponse
		if err := json.Unmarshal(body, &raw); err != nil {
			// Try to get more information about the response
			bodyStr := truncateString(string(body), 200)
			return nil, fmt.Errorf("failed to parse news response (status: %d, body: %s): %w",
				resp.StatusCode, bodyStr, err)
		}

		// Process results
		for _, r := range raw.Results {
			if !seenURLs[r.URL] {
				seenURLs[r.URL] = true

				// Convert Unix timestamp to ISO8601
				date := time.Unix(r.Date, 0).UTC().Format(time.RFC3339)

				result := NewsResult{
					Date:   date,
					Title:  r.Title,
					Body:   r.Excerpt,
					URL:    normalizeURL(r.URL),
					Image:  normalizeURL(r.Image),
					Source: r.Source,
				}
				allResults = append(allResults, result)
			}
		}

		// If we got less than 30 results, there are no more to fetch
		if len(raw.Results) < 30 {
			break
		}
	}

	// If we got no results at all, return an error with more context
	if len(allResults) == 0 {
		return nil, fmt.Errorf("no news results found for query: %s", params.Query)
	}

	// Trim results to max requested
	if len(allResults) > maxResults {
		allResults = allResults[:maxResults]
	}

	return &NewsResponse{
		Results: allResults,
	}, nil
}
