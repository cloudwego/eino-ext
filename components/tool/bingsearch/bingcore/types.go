package bingcore

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const (
	searchURL = "https://api.bing.microsoft.com/v7.0/search"
)

type Region string

const (
	RegionUS Region = "en-US"
)

type SafeSearch string

const (
	SafeSearchOff      SafeSearch = "Off"
	SafeSearchModerate SafeSearch = "Moderate"
	SafeSearchStrict   SafeSearch = "Strict"
)

type TimeRange string

const (
	TimeRangeDay   TimeRange = "Day"
	TimeRangeWeek  TimeRange = "Week"
	TimeRangeMonth TimeRange = "Month"
)

type SearchParams struct {
	Query string `json:"q"`

	Region Region `json:"mkt"`

	SafeSearch SafeSearch `json:"safe_search"`

	TimeRange TimeRange `json:"freshness"`

	Offset int `json:"offset"`

	Count int `json:"count"`

	cacheKey string
}

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

func (s *SearchParams) build() url.Values {
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

func (s *SearchParams) getCacheKey() string {
	params := s.build().Encode()
	hash := md5.Sum([]byte(params))
	return fmt.Sprintf("%s_%x", s.Query, hash)
}

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
