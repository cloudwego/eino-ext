package bingcore

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// searchURL is the base URL for the Bing Web Search API.
const (
	searchURL = "https://api.bing.microsoft.com/v7.0/search"
)

// Region This type represents the Bing search region.
// The region is used to customize the search results for a specific country or language.
// Supported regions: en-US, en-GB, en-CA, en-AU, de-DE, fr-FR, zh-CN, zh-HK, zh-TW, ja-JP, ko-KR.
// Please refer to https://learn.microsoft.com/en-us/bing/search-apis/bing-web-search/reference/market-codes
type Region string

const (
	RegionUS Region = "en-US"
	RegionGB Region = "en-GB"
	RegionCA Region = "en-CA"
	RegionAU Region = "en-AU"
	RegionDE Region = "de-DE"
	RegionFR Region = "fr-FR"
	RegionCN Region = "zh-CN"
	RegionHK Region = "zh-HK"
	RegionTW Region = "zh-TW"
	RegionJP Region = "ja-JP"
	RegionKR Region = "ko-KR"
)

// SafeSearch This type represents the Bing search safe search setting.
type SafeSearch string

const (
	SafeSearchOff      SafeSearch = "Off"
	SafeSearchModerate SafeSearch = "Moderate"
	SafeSearchStrict   SafeSearch = "Strict"
)

// TimeRange This type represents the Bing search time range.
type TimeRange string

const (
	TimeRangeDay   TimeRange = "Day"
	TimeRangeWeek  TimeRange = "Week"
	TimeRangeMonth TimeRange = "Month"
)

// SearchParams This struct represents the search parameters for the Bing Web Search API.
// The search parameters include the search query, region, safe search setting, time range, offset, and count.
// The search parameters are used to customize the search results.
// Please refer to https://learn.microsoft.com/en-us/bing/search-apis/bing-web-search/reference/query-parameters
type SearchParams struct {
	Query string `json:"q"`

	Region Region `json:"mkt"`

	SafeSearch SafeSearch `json:"safe_search"`

	TimeRange TimeRange `json:"freshness"`

	Offset int `json:"offset"`

	Count int `json:"count"`

	cacheKey string
}

// NextPage NewSearchParams creates a new SearchParams instance.
func (s *SearchParams) NextPage() *SearchParams {
	return &SearchParams{
		Query:      s.Query,
		Region:     s.Region,
		SafeSearch: s.SafeSearch,
		TimeRange:  s.TimeRange,
		Offset:     s.Offset + 1,
		Count:      s.Count,
	}
}

// NewSearchParams creates a new SearchParams instance.
func (s *SearchParams) build() url.Values {
	// Build search parameters
	params := url.Values{}

	params.Set("q", s.Query)
	params.Set("mkt", string(s.Region))
	params.Set("count", strconv.Itoa(s.Count))

	if s.TimeRange != "" {
		params.Set("freshness", string(s.TimeRange))
	}

	if s.Offset > 0 {
		params.Set("offset", strconv.Itoa(s.Offset))
	}

	if s.SafeSearch != "" {
		params.Set("safeSearch", string(s.SafeSearch))
	}

	return params
}

// getCacheKey generates a cache key for the search parameters.
// The cache key is a combination of the search query and the hash of the search parameters.
func (s *SearchParams) getCacheKey() string {
	params := s.build().Encode()
	hash := md5.Sum([]byte(params))
	return fmt.Sprintf("%s_%x", s.Query, hash)
}

// validate validates the search parameters.
func (s *SearchParams) validate() error {
	// Validate params
	if s.Query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	if s.Count <= 0 {
		return fmt.Errorf("search count must be greater than 0")
	}

	if s.Offset < 0 {
		return fmt.Errorf("search offset must be greater than or equal to 0")
	}

	return nil
}

// SearchResult This struct formats the search results provided by the Bing Web Search API.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// BingAnswer This struct formats the answers provided by the Bing Web Search API.
type BingAnswer struct {
	Type         string `json:"_type"`
	QueryContext struct {
		OriginalQuery string `json:"originalQuery"`
	} `json:"queryContext"`
	WebPages struct {
		WebSearchURL          string `json:"webSearchUrl"`
		TotalEstimatedMatches int    `json:"totalEstimatedMatches"`
		Value                 []struct {
			ID               string    `json:"id"`
			Name             string    `json:"name"`
			URL              string    `json:"url"`
			IsFamilyFriendly bool      `json:"isFamilyFriendly"`
			DisplayURL       string    `json:"displayUrl"`
			Snippet          string    `json:"snippet"`
			DateLastCrawled  time.Time `json:"dateLastCrawled"`
			SearchTags       []struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			} `json:"searchTags,omitempty"`
			About []struct {
				Name string `json:"name"`
			} `json:"about,omitempty"`
		} `json:"value"`
	} `json:"webPages"`
	RelatedSearches struct {
		ID    string `json:"id"`
		Value []struct {
			Text         string `json:"text"`
			DisplayText  string `json:"displayText"`
			WebSearchURL string `json:"webSearchUrl"`
		} `json:"value"`
	} `json:"relatedSearches"`
	RankingResponse struct {
		Mainline struct {
			Items []struct {
				AnswerType  string `json:"answerType"`
				ResultIndex int    `json:"resultIndex"`
				Value       struct {
					ID string `json:"id"`
				} `json:"value"`
			} `json:"items"`
		} `json:"mainline"`
		Sidebar struct {
			Items []struct {
				AnswerType string `json:"answerType"`
				Value      struct {
					ID string `json:"id"`
				} `json:"value"`
			} `json:"items"`
		} `json:"sidebar"`
	} `json:"rankingResponse"`
}
