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

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WikipediaClient is a client for the Wikipedia API.
type WikipediaClient struct {
	// httpClient is the HTTP client used to make requests.
	httpClient *http.Client
	// baseURL is the base URL for the Wikipedia API.
	baseURL string
	// userAgent is the user agent used in the requests.
	userAgent string
	// language is the language used in the requests.
	language string
	// topK is the number of search results to return.
	topK int
}

// NewClient creates a new Wikipedia client.
func NewClient(opts ...ClientOption) *WikipediaClient {
	c := &WikipediaClient{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Search searches the Wikipedia for the query and returns the search results.
// API documentation: https://www.mediawiki.org/wiki/API:Search
func (c *WikipediaClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrInvalidParameters
	}

	params := url.Values{
		"action":   []string{"query"},
		"list":     []string{"search"},
		"srsearch": []string{query},
		"srlimit":  []string{fmt.Sprintf("%d", c.topK)},
		"srprop":   []string{"wordcount|snippet"},
		"format":   []string{"json"},
	}

	var response struct {
		Query struct {
			Search []struct {
				Title     string `json:"title"`
				PageID    int    `json:"pageid"`
				Snippet   string `json:"snippet"`
				WordCount int    `json:"wordcount"`
			} `json:"search"`
		} `json:"query"`
		Error *APIError `json:"error"`
	}

	if err := c.makeRequest(ctx, params, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, response.Error
	}

	results := make([]SearchResult, 0, len(response.Query.Search))
	for _, item := range response.Query.Search {
		results = append(results, SearchResult{
			Title:     item.Title,
			PageID:    item.PageID,
			Snippet:   cleanBasicHTML(item.Snippet),
			WordCount: item.WordCount,
			URL:       c.buildPageURL(item.Title),
			Language:  c.language,
		})
	}

	return results, nil
}

// GetPage retrieves the Wikipedia page by title.
func (c *WikipediaClient) GetPage(ctx context.Context, title string) (*Page, error) {
	params := url.Values{
		"action":      []string{"query"},
		"prop":        []string{"extracts|revisions"},
		"titles":      []string{title},
		"exlimit":     []string{"1"},
		"explaintext": []string{"1"},
		"rvprop":      []string{"timestamp"},
		"format":      []string{"json"},
	}

	var response struct {
		Query struct {
			Pages map[string]struct {
				PageID    int    `json:"pageid"`
				Title     string `json:"title"`
				Extract   string `json:"extract"`
				Revisions []struct {
					Timestamp time.Time `json:"timestamp"`
				} `json:"revisions"`
			} `json:"pages"`
		} `json:"query"`
		Error *APIError `json:"error"`
	}

	if err := c.makeRequest(ctx, params, &response); err != nil {
		return nil, err
	}

	for _, page := range response.Query.Pages {
		if page.PageID == 0 {
			return nil, ErrPageNotFound
		}

		var lastUpdated time.Time
		if len(page.Revisions) > 0 {
			lastUpdated = page.Revisions[0].Timestamp
		}

		return &Page{
			Title:       page.Title,
			PageID:      page.PageID,
			Content:     page.Extract,
			URL:         c.buildPageURL(page.Title),
			LastUpdated: lastUpdated,
		}, nil
	}

	return nil, ErrPageNotFound
}

// buildPageURL builds the URL for the Wikipedia page.
func (c *WikipediaClient) buildPageURL(title string) string {
	return fmt.Sprintf("https://%s.wikipedia.org/wiki/%s",
		c.language,
		url.PathEscape(title))
}

// makeRequest makes a request to the Wikipedia API.
func (c *WikipediaClient) makeRequest(ctx context.Context, params url.Values, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body failed: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", err)
	}

	return nil
}

// cleanBasicHTML removes some basic HTML tags from the snippet.
func cleanBasicHTML(snippet string) string {
	return strings.NewReplacer(
		"<span class=\"searchmatch\">", "",
		"</span>", "",
		"&nbsp;", " ",
	).Replace(snippet)
}
