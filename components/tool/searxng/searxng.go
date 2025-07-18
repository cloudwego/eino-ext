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

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// 全局变量定义支持的参数值
var (
	validTimeRanges = []string{"day", "month", "year"}
	validLanguages  = []string{"all", "en", "zh", "zh-CN", "zh-TW", "fr", "de", "es", "ja", "ko", "ru", "ar", "pt", "it", "nl", "pl", "tr"}
	validSafeSearch = []int{0, 1, 2}
	validEngines    = []string{"google", "duckduckgo", "baidu", "bing", "360search", "yahoo", "quark"}
)

type SearchRequest struct {
	Query      string  `json:"query"`
	PageNo     int     `json:"pageno"`
	TimeRange  *string `json:"time_range,omitempty"`
	Language   *string `json:"language,omitempty"`
	SafeSearch *int    `json:"safesearch,omitempty"`
	Engines    *string `json:"engines,omitempty"`
}

func (s *SearchRequest) validate() error {
	if s.Query == "" {
		return errors.New("query is required")
	}

	if s.PageNo <= 0 {
		return errors.New("pageno must be greater than 0")
	}

	if s.TimeRange != nil {
		if err := validateInSlice(*s.TimeRange, validTimeRanges, "time_range"); err != nil {
			return err
		}
	}

	if s.Language != nil {
		if err := validateInSlice(*s.Language, validLanguages, "language"); err != nil {
			return err
		}
	}

	if s.SafeSearch != nil {
		if err := validateInSlice(*s.SafeSearch, validSafeSearch, "safesearch"); err != nil {
			return err
		}
	}

	if s.Engines != nil {
		if err := validateEngines(*s.Engines); err != nil {
			return err
		}
	}

	return nil
}

func (s *SearchRequest) build() url.Values {
	params := url.Values{}
	params.Set("q", s.Query)
	params.Set("pageno", strconv.Itoa(s.PageNo))
	params.Set("format", "json")
	if s.TimeRange != nil {
		params.Set("time_range", *s.TimeRange)
	}
	if s.Language != nil {
		params.Set("language", *s.Language)
	}
	if s.SafeSearch != nil {
		params.Set("safesearch", strconv.Itoa(*s.SafeSearch))
	}
	if s.Engines != nil {
		params.Set("engines", *s.Engines)
	}
	return params
}

// validateInSlice 使用泛型验证值是否在给定的切片中
func validateInSlice[T comparable](value T, validValues []T, paramName string) error {
	for _, valid := range validValues {
		if value == valid {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %+v", paramName, validValues)
}

// validateEngines 验证engines参数，支持逗号分隔的多个engines
func validateEngines(engines string) error {
	if engines == "" {
		return nil
	}

	// 分割逗号分隔的engines
	engineList := strings.Split(engines, ",")
	for _, engine := range engineList {
		engine = strings.TrimSpace(engine)
		if engine == "" {
			continue
		}

		// 检查每个engine是否在有效列表中
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
	Title   string `json:"title" jsonschema:"description=The title of the search result"`
	Content string `json:"content" jsonschema:"description=The content of the search result"`
	URL     string `json:"url" jsonschema:"description=The URL of the search result"`
	Engine  string `json:"engine" jsonschema:"description=The engine of the search result"`
}

type SearchResponse struct {
	Query           string          `json:"query" jsonschema:"description=The query of the search"`
	NumberOfResults int             `json:"number_of_results" jsonschema:"description=The number of results of the search"`
	Results         []*SearchResult `json:"results"  jsonschema:"description=The results of the search"`
}

type SearxngClient struct {
	client *http.Client
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
}

func NewClient(config *ClientConfig) (*SearxngClient, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}

	sc := &SearxngClient{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config: config,
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

		resp, err = s.client.Do(req)
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
	query := params.build()

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

func (s *SearxngClient) SearchStream(ctx context.Context, params *SearchRequest) (*schema.StreamReader[*SearchResult], error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	resp, err := s.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	// Create StreamReader from Results
	streamReader := schema.StreamReaderFromArray(resp.Results)

	return streamReader, nil
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

func getSearchSchema() *schema.ToolInfo {
	sc := &openapi3.Schema{
		Type:     openapi3.TypeObject,
		Required: []string{"query"},
		Properties: map[string]*openapi3.SchemaRef{
			"query": {
				Value: &openapi3.Schema{
					Type:        openapi3.TypeString,
					Description: "The search query. This is the main input for the web search",
				},
			},
			"pageno": {
				Value: &openapi3.Schema{
					Type:        openapi3.TypeInteger,
					Description: "The page number of the search results. Default is 1",
					Default:     1,
				},
			},
			"time_range": {
				Value: &openapi3.Schema{
					Description: "Time range of search",
					OneOf: []*openapi3.SchemaRef{
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"day"},
								Description: "Search information from the past day",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"month"},
								Description: "Search information from the past month",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"year"},
								Description: "Search information from the past year",
							},
						},
					},
				},
			},
			"language": {
				Value: &openapi3.Schema{
					Description: "Language of search",
					Default:     "all",
					OneOf: []*openapi3.SchemaRef{
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"all"},
								Description: "Search in all languages",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"en"},
								Description: "Search in English",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"zh"},
								Description: "Search in Chinese",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"zh-CN"},
								Description: "Search in Chinese (Simplified)",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"zh-TW"},
								Description: "Search in Chinese (Traditional)",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"fr"},
								Description: "Search in French",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"de"},
								Description: "Search in German",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"es"},
								Description: "Search in Spanish",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"ja"},
								Description: "Search in Japanese",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"ko"},
								Description: "Search in Korean",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"ru"},
								Description: "Search in Russian",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"ar"},
								Description: "Search in Arabic",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"pt"},
								Description: "Search in Portuguese",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"it"},
								Description: "Search in Italian",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"nl"},
								Description: "Search in Dutch",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"pl"},
								Description: "Search in Polish",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"tr"},
								Description: "Search in Turkish",
							},
						},
					},
				},
			},
			"safesearch": {
				Value: &openapi3.Schema{
					Description: "Safe search filter level",
					Default:     0,
					OneOf: []*openapi3.SchemaRef{
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeInteger,
								Enum:        []any{0},
								Description: "None - No safe search filtering",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeInteger,
								Enum:        []any{1},
								Description: "Moderate - Moderate safe search filtering",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeInteger,
								Enum:        []any{2},
								Description: "Strict - Strict safe search filtering",
							},
						},
					},
				},
			},
			"engines": {
				Value: &openapi3.Schema{
					Type:        openapi3.TypeString,
					Description: "Comma separated list, specifies the active search engines",
					OneOf: []*openapi3.SchemaRef{
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"google"},
								Description: "Google search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"duckduckgo"},
								Description: "DuckDuckGo search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"baidu"},
								Description: "Baidu search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"bing"},
								Description: "Bing search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"360search"},
								Description: "360 search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"yahoo"},
								Description: "Yahoo search engine",
							},
						},
						{
							Value: &openapi3.Schema{
								Type:        openapi3.TypeString,
								Enum:        []any{"quark"},
								Description: "Quark search engine",
							},
						},
					},
				},
			},
		},
	}

	toolName := "web_search"
	toolDesc := `Performs a web search using the SearXNG API, ideal for general queries, news, articles, and online content.
		Use this for broad information gathering, recent events, or when you need diverse web sources.`

	info := &schema.ToolInfo{
		Name:        toolName,
		Desc:        toolDesc,
		ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(sc),
	}

	return info
}

func BuildSearchInvokeTool(cfg *ClientConfig) (tool.InvokableTool, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	searchTool := utils.NewTool(getSearchSchema(), client.Search)
	return searchTool, nil
}

func BuildSearchStreamTool(cfg *ClientConfig) (tool.StreamableTool, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	searchTool := utils.NewStreamTool(getSearchSchema(), client.SearchStream)
	return searchTool, nil
}
