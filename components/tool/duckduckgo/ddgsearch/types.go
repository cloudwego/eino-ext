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

// Common constants
var (
	baseURL   = "https://duckduckgo.com"
	searchURL = "https://links.duckduckgo.com/d.js"
	newsURL   = "https://duckduckgo.com/news.js"
)

// Region represents a geographical region for search results.
// Different regions may return different search results based on local relevance.
// others can be found at: https://duckduckgo.com/duckduckgo-help-pages/settings/params/
type Region string

// Available regions for DuckDuckGo search
const (
	// RegionWT represents World region (No specific region, default)
	RegionWT Region = "wt-wt"
	// RegionUS represents United States region
	RegionUS Region = "us-en"
	// RegionUK represents United Kingdom region
	RegionUK Region = "uk-en"
	// RegionDE represents Germany region
	RegionDE Region = "de-de"
	// RegionFR represents France region
	RegionFR Region = "fr-fr"
	// RegionJP represents Japan region
	RegionJP Region = "jp-jp"
	// RegionCN represents China region
	RegionCN Region = "cn-zh"
	// RegionRU represents Russia region
	RegionRU Region = "ru-ru"
)

// SafeSearch represents the safe search level for filtering explicit content.
type SafeSearch string

const (
	// SafeSearchStrict enables strict filtering of explicit content
	SafeSearchStrict SafeSearch = "strict"
	// SafeSearchModerate enables moderate filtering of explicit content
	SafeSearchModerate SafeSearch = "moderate"
	// SafeSearchOff disables filtering of explicit content
	SafeSearchOff SafeSearch = "off"
)

// TimeRange represents the time range for search results.
type TimeRange string

const (
	// TimeRangeDay limits results to the past day
	TimeRangeDay TimeRange = "d"
	// TimeRangeWeek limits results to the past week
	TimeRangeWeek TimeRange = "w"
	// TimeRangeMonth limits results to the past month
	TimeRangeMonth TimeRange = "m"
	// TimeRangeYear limits results to the past year
	TimeRangeYear TimeRange = "y"
	// TimeRangeAll includes results from all time periods
	TimeRangeAll TimeRange = ""
)

// NewsParams configures the news search behavior.
type NewsParams struct {
	// Query is the search term or phrase
	Query string `json:"query"`

	// Region specifies the geographical region for results
	// Use one of the Region constants (e.g., RegionUS, RegionUK)
	Region Region `json:"region"`

	// SafeSearch controls filtering of explicit content
	// Use one of the SafeSearch constants
	SafeSearch SafeSearch `json:"safe_search"`

	// TimeRange limits results to a specific time period
	// Use one of the TimeRange constants
	TimeRange TimeRange `json:"time_range"`

	// MaxResults limits the number of results returned.
	// Set to 0 for no limit. Note that:
	// 1. DuckDuckGo API typically returns 10 results per page
	// 2. This parameter only truncates results when the API returns more results than MaxResults
	// 3. To get more results, use NextPage() to paginate through results
	MaxResults int `json:"max_results"`
}

// SearchParams configures the search behavior.
// Example usage:
//
//	params := &SearchParams{
//		Query:      "golang tutorials",        // Required
//		Region:     RegionCN,                  // Optional, defaults to RegionCN
//		SafeSearch: SafeSearchModerate,        // Optional, defaults to Moderate
//		TimeRange:  TimeRangeMonth,            // Optional, defaults to All
//		Page:       1,                         // Optional, defaults to 1
//		MaxResults: 10,                        // Optional, defaults to all results
//	}
//
// see more at: https://duckduckgo.com/duckduckgo-help-pages/settings/params/
type SearchParams struct {
	// Query is the search term or phrase
	Query string `json:"query"`

	// Region specifies the geographical region for results
	// Use one of the Region constants (e.g., RegionUS, RegionUK)
	Region Region `json:"region"`

	// SafeSearch controls filtering of explicit content
	// Use one of the SafeSearch constants
	SafeSearch SafeSearch `json:"safe_search"`

	// TimeRange limits results to a specific time period
	// Use one of the TimeRange constants
	TimeRange TimeRange `json:"time_range"`

	// Page specifies which page of results to return
	// Starts from 1
	Page int `json:"page"`

	// MaxResults limits the number of results returned.
	// Set to 0 for no limit. Note that:
	// 1. DuckDuckGo API typically returns 10 results per page
	// 2. This parameter only truncates results when the API returns more results than MaxResults
	// 3. To get more results, use NextPage() to paginate through results
	MaxResults int `json:"max_results"`

	// cacheKey is used internally for caching search results
	cacheKey string `json:"-"`
}

// NextPage returns a new SearchParams with the page number incremented.
// This is useful for paginating through search results.
//
// Example usage:
//
//	params := &SearchParams{
//		Query:      "golang",
//		MaxResults: 10,
//	}
//
//	// Get first page results
//	results1, err := client.Search(ctx, params)
//	if err != nil {
//		return err
//	}
//
//	// Get next page results
//	nextParams := params.NextPage()
//	results2, err := client.Search(ctx, nextParams)
func (p *SearchParams) NextPage() *SearchParams {
	return &SearchParams{
		Query:      p.Query,
		Region:     p.Region,
		SafeSearch: p.SafeSearch,
		TimeRange:  p.TimeRange,
		MaxResults: p.MaxResults,
		Page:       p.Page + 1,
	}
}

// SearchResult represents a single search result.
// Contains the title, URL, and description of the result.
type SearchResult struct {
	// Title is the title of the search result
	Title string `json:"t"`

	// URL is the web address of the result
	URL string `json:"u"`

	// Description is a brief summary of the result content
	Description string `json:"a"`
}

// SearchResponse represents the complete response from a search request.
type SearchResponse struct {
	// Results contains the list of search results
	Results []SearchResult `json:"results"`

	// NoResults indicates whether the search returned any results
	NoResults bool `json:"noResults"`
}

// NewsResult represents a single news search result from DuckDuckGo.
type NewsResult struct {
	Date   string `json:"date"`   // ISO8601 formatted date
	Title  string `json:"title"`  // News article title
	Body   string `json:"body"`   // News article excerpt/summary
	URL    string `json:"url"`    // Article URL
	Image  string `json:"image"`  // URL of the article's image
	Source string `json:"source"` // News source name
}

// NewsResponse represents the complete response from a news search request.
type NewsResponse struct {
	Results []NewsResult `json:"results"` // List of news results
}

// rawNewsResponse represents the raw response from DuckDuckGo news API.
type rawNewsResponse struct {
	Results []struct {
		Date    int64  `json:"date"`    // Unix timestamp
		Title   string `json:"title"`   // Article title
		Excerpt string `json:"excerpt"` // Article excerpt
		URL     string `json:"url"`     // Article URL
		Image   string `json:"image"`   // Image URL
		Source  string `json:"source"`  // Source name
	} `json:"results"`
	Query         string `json:"query"`          // The search query
	QueryEncoded  string `json:"queryEncoded"`   // URL encoded query
	ResponseType  string `json:"response_type"`  // Type of response
	QueryCategory string `json:"query_category"` // Category of query
}
