/*
 * Copyright 2025 CloudWeGo Authors
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

package searxng

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// TimeRange represents the time range for search
type TimeRange string

const (
	TimeRangeDay   TimeRange = "day"
	TimeRangeMonth TimeRange = "month"
	TimeRangeYear  TimeRange = "year"
)

// Language represents the language code for search
type Language string

const (
	LanguageAll  Language = "all"
	LanguageEn   Language = "en"
	LanguageZh   Language = "zh"
	LanguageZhCN Language = "zh-CN"
	LanguageZhTW Language = "zh-TW"
	LanguageFr   Language = "fr"
	LanguageDe   Language = "de"
	LanguageEs   Language = "es"
	LanguageJa   Language = "ja"
	LanguageKo   Language = "ko"
	LanguageRu   Language = "ru"
	LanguageAr   Language = "ar"
	LanguagePt   Language = "pt"
	LanguageIt   Language = "it"
	LanguageNl   Language = "nl"
	LanguagePl   Language = "pl"
	LanguageTr   Language = "tr"
)

// SafeSearchLevel represents the safe search filter level
type SafeSearchLevel int

const (
	SafeSearchNone     SafeSearchLevel = 0
	SafeSearchModerate SafeSearchLevel = 1
	SafeSearchStrict   SafeSearchLevel = 2
)

// Engine represents the search engine
type Engine string

const (
	EngineGoogle     Engine = "google"
	EngineDuckDuckGo Engine = "duckduckgo"
	EngineBaidu      Engine = "baidu"
	EngineBing       Engine = "bing"
	Engine360Search  Engine = "360search"
	EngineYahoo      Engine = "yahoo"
	EngineQuark      Engine = "quark"
)

var (
	// validTimeRanges defines the valid time range values for search
	validTimeRanges = []TimeRange{TimeRangeDay, TimeRangeMonth, TimeRangeYear}

	// validLanguages defines the valid language codes for search
	validLanguages = []Language{LanguageAll, LanguageEn, LanguageZh, LanguageZhCN, LanguageZhTW, LanguageFr, LanguageDe, LanguageEs, LanguageJa, LanguageKo, LanguageRu, LanguageAr, LanguagePt, LanguageIt, LanguageNl, LanguagePl, LanguageTr}

	// validSafeSearch defines the valid safe search levels
	validSafeSearch = []SafeSearchLevel{SafeSearchNone, SafeSearchModerate, SafeSearchStrict}

	// validEngines defines the valid search engines
	validEngines = []Engine{EngineGoogle, EngineDuckDuckGo, EngineBaidu, EngineBing, Engine360Search, EngineYahoo, EngineQuark}
)

const (
	toolName = "web_search"
	toolDesc = `Performs a web search using the SearXNG API, ideal for general queries, news, articles, and online content.
		Use this for broad information gathering, recent events, or when you need diverse web sources.`
)

type SearchRequest struct {
	Query  string `json:"query" jsonschema:"required" jsonschema_description:"The search query. This is the main input for the web search"`
	PageNo *int   `json:"pageno" jsonschema_description:"The page number of the search results. Default is 1"`
}

func (s *SearchRequest) validate() error {
	if s.Query == "" {
		return errors.New("query is required")
	}

	if s.PageNo != nil && *s.PageNo <= 0 {
		return errors.New("pageno must be greater than 0")
	}

	return nil
}

type SearchRequestConfig struct {
	TimeRange  TimeRange       `json:"time_range,omitempty"`
	Language   Language        `json:"language,omitempty"`
	SafeSearch SafeSearchLevel `json:"safesearch,omitempty"`
	Engines    []Engine        `json:"engines,omitempty"`
}

func (s *SearchRequestConfig) validate() error {
	// Only validate TimeRange when it's not zero value
	if s.TimeRange != "" {
		if err := validateInSlice(s.TimeRange, validTimeRanges, "time_range"); err != nil {
			return err
		}
	}

	// Only validate Language when it's not zero value
	if s.Language != "" {
		if err := validateInSlice(s.Language, validLanguages, "language"); err != nil {
			return err
		}
	}

	// Only validate SafeSearch when it's not zero value
	if s.SafeSearch != 0 {
		if err := validateInSlice(s.SafeSearch, validSafeSearch, "safesearch"); err != nil {
			return err
		}
	}

	if len(s.Engines) > 0 {
		if err := validateEngines(s.Engines); err != nil {
			return err
		}
	}

	return nil
}

func (s *SearchRequest) build(cfg *SearchRequestConfig) url.Values {
	params := url.Values{}
	params.Set("q", s.Query)
	if s.PageNo != nil {
		params.Set("pageno", strconv.Itoa(*s.PageNo))
	}
	params.Set("format", "json")
	if cfg != nil {
		// Only add TimeRange when it's not zero value
		if cfg.TimeRange != "" {
			params.Set("time_range", string(cfg.TimeRange))
		}
		// Only add Language when it's not zero value
		if cfg.Language != "" {
			params.Set("language", string(cfg.Language))
		}
		// Only add SafeSearch when it's not zero value
		if cfg.SafeSearch != 0 {
			params.Set("safesearch", strconv.Itoa(int(cfg.SafeSearch)))
		}
		if len(cfg.Engines) > 0 {
			// Convert []Engine to comma-separated string
			engineStrs := make([]string, len(cfg.Engines))
			for i, engine := range cfg.Engines {
				engineStrs[i] = string(engine)
			}
			params.Set("engines", strings.Join(engineStrs, ","))
		}
	}
	return params
}

// validateInSlice validates whether a value is in the given slice using generics
func validateInSlice[T comparable](value T, validValues []T, paramName string) error {
	for _, valid := range validValues {
		if value == valid {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %+v", paramName, validValues)
}

// validateEngines validates engines parameter, supports multiple engines
func validateEngines(engines []Engine) error {
	if len(engines) == 0 {
		return nil
	}

	for _, engine := range engines {
		// Check if each engine is in the valid list
		valid := false
		for _, validEngine := range validEngines {
			if engine == validEngine {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("engine '%s' is not supported. Valid engines are: %+v", engine, validEngines)
		}
	}

	return nil
}

type SearchResult struct {
	Title   string `json:"title" jsonschema_description:"The title of the search result"`
	Content string `json:"content" jsonschema_description:"The content of the search result"`
	URL     string `json:"url" jsonschema_description:"The URL of the search result"`
	Engine  string `json:"engine" jsonschema_description:"The engine of the search result"`
}

type SearchResponse struct {
	Query           string          `json:"query" jsonschema_description:"The query of the search"`
	NumberOfResults int             `json:"number_of_results" jsonschema_description:"The number of results of the search"`
	Results         []*SearchResult `json:"results"  jsonschema_description:"The results of the search"`
}

type SearxngClient struct {
	config *ClientConfig
}

// Config represents the search client configuration.
type ClientConfig struct {
	// BaseUrl specifies the base URL of the SearxNG instance.
	BaseUrl string `json:"base_url"`

	// Headers specifies custom HTTP headers to be sent with each request.
	// Common headers like "User-Agent" can be set here.
	// Example:
	//   Headers: map[string]string{
	//     "User-Agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; rv:11.0) like Gecko",
	//     "Accept-Language": "en-US",
	//   }
	Headers map[string]string `json:"headers"`

	// HttpClient specifies the custom HTTP client to be used.
	// If not specified, a default client will be used.
	HttpClient *http.Client `json:"http_client"`

	// Timeout specifies the maximum duration for a single request.
	// Default is 30 seconds if not specified.
	// Default: 30 seconds
	// Example: 5 * time.Second
	Timeout time.Duration `json:"timeout"`

	// ProxyURL specifies the proxy server URL for all requests.
	// Supports HTTP, HTTPS, and SOCKS5 proxies.
	// Default: ""
	// Example values:
	//   - "http://proxy.example.com:8080"
	//   - "socks5://localhost:1080"
	//   - "tb" (special alias for Tor Browser)
	ProxyURL string `json:"proxy_url"`

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Default: 3
	MaxRetries int `json:"max_retries"`

	// RequestConfig specifies the search request configuration.
	RequestConfig *SearchRequestConfig
}

func NewClient(cfg *ClientConfig) (*SearxngClient, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	// Validate the validity of requestConfig
	if cfg.RequestConfig != nil {
		if err := cfg.RequestConfig.validate(); err != nil {
			return nil, err
		}
	}

	// Use externally provided HTTP client, or create default one if not provided
	if cfg.HttpClient == nil {
		cfg.HttpClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	sc := &SearxngClient{
		config: cfg,
	}
	return sc, nil
}

// sendRequestWithRetry sends the request with retry logic.
func (s *SearxngClient) sendRequestWithRetry(ctx context.Context, req *http.Request) (*SearchResponse, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if req == nil {
		return nil, errors.New("request is nil")
	}
	var resp *http.Response
	var err error
	var attempt int

	for attempt = 0; attempt <= s.config.MaxRetries; attempt++ {
		// Check context cancellation
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		resp, err = s.config.HttpClient.Do(req)
		if err != nil {
			if attempt == s.config.MaxRetries {
				return nil, fmt.Errorf("failed to send request after retries: %w", err)
			}
			time.Sleep(time.Second) // Simple fixed one-second delay between retries
			continue
		}

		// Check for successful response
		if resp.StatusCode == http.StatusOK {
			break
		}

		// Check for rate limit response
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == s.config.MaxRetries {
				return nil, errors.New("rate limit reached")
			}
			time.Sleep(time.Second)
			continue
		}
	}

	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse search response
	response, err := parseSearchResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	// Check for no results
	if len(response.Results) == 0 {
		return nil, errors.New("no search results found")
	}

	return response, nil
}

// Search sends a search request to Searxng API and returns the search results.
func (s *SearxngClient) Search(ctx context.Context, params *SearchRequest) (*SearchResponse, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if params == nil {
		return nil, errors.New("params is nil")
	}

	// Validate search query
	if err := params.validate(); err != nil {
		return nil, err
	}

	// Set default SafeSearch if not provided
	query := params.build(s.config.RequestConfig)

	// Build query URL
	queryURL := fmt.Sprintf("%s?%s", s.config.BaseUrl, query.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range s.config.Headers {
		req.Header.Set(k, v)
	}

	// Set default User-Agent if not provided
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	// Send request with retry
	results, err := s.sendRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func parseSearchResponse(body []byte) (*SearchResponse, error) {
	var response SearchResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	if response.NumberOfResults == 0 {
		response.NumberOfResults = len(response.Results)
	}
	return &response, nil
}

func BuildSearchInvokeTool(cfg *ClientConfig) (tool.InvokableTool, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return utils.InferTool(toolName, toolDesc, client.Search)
}
