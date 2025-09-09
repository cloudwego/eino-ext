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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSearchRequest_validate(t *testing.T) {
	type fields struct {
		Query  string
		PageNo *int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "valid request",
			fields: fields{
				Query:  "test",
				PageNo: intPtr(1),
			},
			wantErr: false,
		},
		{
			name: "missing query",
			fields: fields{
				PageNo: intPtr(1),
			},
			wantErr: true,
		},
		{
			name: "invalid pageno",
			fields: fields{
				Query:  "test",
				PageNo: intPtr(0),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SearchRequest{
				Query:  tt.fields.Query,
				PageNo: tt.fields.PageNo,
			}
			if err := s.validate(); (err != nil) != tt.wantErr {
				t.Errorf("SearchRequest.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchRequestConfig_validate(t *testing.T) {
	type fields struct {
		TimeRange  TimeRange
		Language   Language
		SafeSearch SafeSearchLevel
		Engines    []Engine
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "valid config",
			fields: fields{
				TimeRange:  TimeRangeDay,
				Language:   LanguageEn,
				SafeSearch: SafeSearchNone,
				Engines:    []Engine{EngineGoogle, EngineBing},
			},
			wantErr: false,
		},
		{
			name: "invalid time_range",
			fields: fields{
				TimeRange: timeRangeValue("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid language",
			fields: fields{
				Language: languageValue("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid safesearch",
			fields: fields{
				SafeSearch: safeSearchValue(99),
			},
			wantErr: true,
		},
		{
			name: "invalid engines",
			fields: fields{
				Engines: enginePtr("invalid"),
			},
			wantErr: true,
		},
		{
			name: "valid engines",
			fields: fields{
				Engines: enginePtr("google,bing"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SearchRequestConfig{
				TimeRange:  tt.fields.TimeRange,
				Language:   tt.fields.Language,
				SafeSearch: tt.fields.SafeSearch,
				Engines:    tt.fields.Engines,
			}
			if err := s.validate(); (err != nil) != tt.wantErr {
				t.Errorf("SearchRequestConfig.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchRequest_build(t *testing.T) {
	type reqFields struct {
		Query  string
		PageNo *int
	}
	type configFields struct {
		TimeRange  TimeRange
		Language   Language
		SafeSearch SafeSearchLevel
		Engines    []Engine
	}
	tests := []struct {
		name   string
		req    reqFields
		config *configFields
		want   url.Values
	}{
		{
			name: "basic request",
			req: reqFields{
				Query:  "test",
				PageNo: intPtr(1),
			},
			config: nil,
			want: url.Values{
				"q":      []string{"test"},
				"pageno": []string{"1"},
				"format": []string{"json"},
			},
		},
		{
			name: "full request",
			req: reqFields{
				Query:  "test",
				PageNo: intPtr(2),
			},
			config: &configFields{
				TimeRange:  timeRangeValue("day"),
				Language:   languageValue("en"),
				SafeSearch: safeSearchValue(1),
				Engines:    enginePtr("google,bing"),
			},
			want: url.Values{
				"q":          []string{"test"},
				"pageno":     []string{"2"},
				"format":     []string{"json"},
				"time_range": []string{"day"},
				"language":   []string{"en"},
				"safesearch": []string{"1"},
				"engines":    []string{"google,bing"},
			},
			},
		}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SearchRequest{
				Query:  tt.req.Query,
				PageNo: tt.req.PageNo,
			}

			var config *SearchRequestConfig
			if tt.config != nil {
				config = &SearchRequestConfig{
					TimeRange:  tt.config.TimeRange,
					Language:   tt.config.Language,
					SafeSearch: tt.config.SafeSearch,
					Engines:    tt.config.Engines,
				}
			}

			if got := s.build(config); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchRequest.build() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateInSlice(t *testing.T) {
	validStrings := []string{"a", "b", "c"}
	validInts := []int{1, 2, 3}

	tests := []struct {
		name        string
		value       interface{}
		validValues interface{}
		paramName   string
		wantErr     bool
	}{
		{"valid string", "a", validStrings, "param", false},
		{"invalid string", "d", validStrings, "param", true},
		{"valid int", 1, validInts, "param", false},
		{"invalid int", 4, validInts, "param", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch v := tt.value.(type) {
			case string:
				err = validateInSlice(v, tt.validValues.([]string), tt.paramName)
			case int:
				err = validateInSlice(v, tt.validValues.([]int), tt.paramName)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("validateInSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEngines(t *testing.T) {
	tests := []struct {
		name    string
		engines []Engine
		wantErr bool
	}{
		{"valid single", []Engine{"google"}, false},
		{"valid multiple", []Engine{"google", "bing"}, false},
		{"invalid single", []Engine{"invalid"}, true},
		{"invalid in multiple", []Engine{"google", "invalid"}, true},
		{"empty", []Engine{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateEngines(tt.engines); (err != nil) != tt.wantErr {
				t.Errorf("validateEngines() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	tests := []struct {
		name    string
		config  *ClientConfig
		want    *SearxngClient
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &ClientConfig{
				BaseUrl: "http://localhost",
			},
			want: &SearxngClient{
				config: &ClientConfig{
					BaseUrl:    "http://localhost",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Headers:    map[string]string{},
					HttpClient: &http.Client{
						Timeout: 30 * time.Second,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with custom values",
			config: &ClientConfig{
				BaseUrl:    "http://localhost",
				Timeout:    10 * time.Second,
				MaxRetries: 5,
				Headers: map[string]string{
					"User-Agent": "test",
				},
			},
			want: &SearxngClient{
				config: &ClientConfig{
					BaseUrl:    "http://localhost",
					Timeout:    10 * time.Second,
					MaxRetries: 5,
					Headers: map[string]string{
						"User-Agent": "test",
					},
					HttpClient: &http.Client{
						Timeout: 10 * time.Second,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with request config",
			config: &ClientConfig{
				BaseUrl: "http://localhost",
				RequestConfig: &SearchRequestConfig{
					Language: LanguageEn,
				},
			},
			want: &SearxngClient{
				config: &ClientConfig{
					BaseUrl:    "http://localhost",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Headers:    map[string]string{},
					RequestConfig: &SearchRequestConfig{
						Language: LanguageEn,
					},
					HttpClient: &http.Client{
						Timeout: 30 * time.Second,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with custom http client",
			config: &ClientConfig{
				BaseUrl:    "http://localhost",
				HttpClient: customClient,
			},
			want: &SearxngClient{
				config: &ClientConfig{
					BaseUrl:    "http://localhost",
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Headers:    map[string]string{},
					HttpClient: customClient,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// a deep equal on http.Client will fail
			if got != nil {
				if got.config.HttpClient.Timeout != tt.want.config.HttpClient.Timeout {
					t.Errorf("NewClient() http client timeout not equal")
				}
				got.config.HttpClient = tt.want.config.HttpClient
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSearchResponse(t *testing.T) {
	type args struct {
		body []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *SearchResponse
		wantErr bool
	}{
		{
			name: "valid response",
			args: args{
				body: []byte(`{"query": "test", "number_of_results": 1, "results": [{"title": "title", "content": "content", "url": "url", "engine": "engine"}]}`),
			},
			want: &SearchResponse{
				Query:           "test",
				NumberOfResults: 1,
				Results: []*SearchResult{
					{
						Title:   "title",
						Content: "content",
						URL:     "url",
						Engine:  "engine",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid json",
			args: args{
				body: []byte(`invalid json`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing number_of_results",
			args: args{
				body: []byte(`{"query": "test", "results": [{"title": "title", "content": "content", "url": "url", "engine": "engine"}]}`),
			},
			want: &SearchResponse{
				Query:           "test",
				NumberOfResults: 1,
				Results: []*SearchResult{
					{
						Title:   "title",
						Content: "content",
						URL:     "url",
						Engine:  "engine",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSearchResponse(tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSearchResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSearchResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearxngClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "ratelimit" {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if r.URL.Query().Get("q") == "noresults" {
			fmt.Fprintln(w, `{"results": []}`)
			return
		}
		if r.URL.Query().Get("q") == "servererror" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, `{"query": "test", "number_of_results": 1, "results": [{"title": "title", "content": "content", "url": "url", "engine": "engine"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		BaseUrl:    server.URL,
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		params  *SearchRequest
		wantErr bool
		errStr  string
	}{
		{"nil params", nil, true, "params is nil"},
		{"validation error", &SearchRequest{Query: ""}, true, "query is required"},
		{"successful search", &SearchRequest{Query: "test", PageNo: intPtr(1)}, false, ""},
		{"rate limit", &SearchRequest{Query: "ratelimit", PageNo: intPtr(1)}, true, "rate limit reached"},
		{"no results", &SearchRequest{Query: "noresults", PageNo: intPtr(1)}, true, "no search results found"},
		{"server error", &SearchRequest{Query: "servererror", PageNo: intPtr(1)}, true, "failed to parse search results"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Search(context.Background(), tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("Search() error string = %q, want to contain %q", err.Error(), tt.errStr)
			}
		})
	}
}

func TestBuildSearchInvokeTool(t *testing.T) {
	tool, err := BuildSearchInvokeTool(&ClientConfig{BaseUrl: "http://localhost"})
	if err != nil {
		t.Fatalf("BuildSearchInvokeTool() error = %v", err)
	}
	if tool == nil {
		t.Fatal("BuildSearchInvokeTool() returned nil")
	}

	// Test with nil requestConfig
	tool, err = BuildSearchInvokeTool(&ClientConfig{BaseUrl: "http://localhost"})
	if err != nil {
		t.Fatalf("BuildSearchInvokeTool() with nil requestConfig error = %v", err)
	}
	if tool == nil {
		t.Fatal("BuildSearchInvokeTool() with nil requestConfig returned nil")
	}

	// Test with invalid requestConfig
	_, err = BuildSearchInvokeTool(&ClientConfig{
		BaseUrl: "http://localhost",
		RequestConfig: &SearchRequestConfig{
			TimeRange: "invalid",
		},
	})
	if err == nil {
		t.Fatal("BuildSearchInvokeTool() with invalid requestConfig should return an error")
	}
}

func TestBuildSearchInvokeTool_Error(t *testing.T) {
	_, err := BuildSearchInvokeTool(nil)
	if err == nil {
		t.Error("expected an error for nil config, got nil")
	}
}

func TestSendRequestWithRetry_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Ensure the request takes some time
		fmt.Fprintln(w, `{}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL, MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(ctx, req)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// Helper functions for tests
func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func timeRangeValue(s string) TimeRange {
	return TimeRange(s)
}

func languageValue(s string) Language {
	return Language(s)
}

func safeSearchValue(i int) SafeSearchLevel {
	return SafeSearchLevel(i)
}

func enginePtr(s string) []Engine {
	// 将逗号分隔的字符串转换为[]Engine
	if s == "" {
		return nil
	}
	engineStrs := strings.Split(s, ",")
	engines := make([]Engine, 0, len(engineStrs))
	for _, engineStr := range engineStrs {
		engineStr = strings.TrimSpace(engineStr)
		if engineStr != "" {
			engines = append(engines, Engine(engineStr))
		}
	}
	return engines
}

func TestSearxngClient_Search_NoDefaultUserAgent(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		fmt.Fprintln(w, `{"query": "test", "number_of_results": 1, "results": [{"title": "title", "content": "content", "url": "url", "engine": "engine"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		BaseUrl: server.URL,
		Headers: map[string]string{
			"User-Agent": "custom-agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if userAgent != "custom-agent" {
		t.Errorf("expected User-Agent 'custom-agent', got '%s'", userAgent)
	}
}

func TestSearxngClient_sendRequestWithRetry_FailedRequest(t *testing.T) {
	client, err := NewClient(&ClientConfig{
		BaseUrl:    "http://localhost:12345", // Non-existent server
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", client.config.BaseUrl, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)

	if err == nil {
		t.Error("expected an error for a failed request, got nil")
	}
	if !strings.Contains(err.Error(), "failed to send request after retries") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearxngClient_sendRequestWithRetry_BadResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1") // Set a content length but send no body
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)

	if err == nil {
		t.Error("expected an error for a bad response body, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read response body") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchRequest_validate_valid(t *testing.T) {
	// 创建并验证 SearchRequest
	req := &SearchRequest{
		Query:  "test",
		PageNo: intPtr(1),
	}

	err := req.validate()
	if err != nil {
		t.Errorf("unexpected error validating SearchRequest: %v", err)
	}

	// 创建并验证 SearchRequestConfig
	config := &SearchRequestConfig{
		TimeRange:  timeRangeValue("day"),
		Language:   languageValue("en"),
		SafeSearch: safeSearchValue(1),
		Engines:    enginePtr("google"),
	}

	err = config.validate()
	if err != nil {
		t.Errorf("unexpected error validating SearchRequestConfig: %v", err)
	}

	// 测试 build 方法
	params := req.build(config)
	if params.Get("q") != "test" {
		t.Errorf("expected query to be 'test', got '%s'", params.Get("q"))
	}
	if params.Get("time_range") != "day" {
		t.Errorf("expected time_range to be 'day', got '%s'", params.Get("time_range"))
	}
	if params.Get("language") != "en" {
		t.Errorf("expected language to be 'en', got '%s'", params.Get("language"))
	}
	if params.Get("safesearch") != "1" {
		t.Errorf("expected safesearch to be '1', got '%s'", params.Get("safesearch"))
	}
	if params.Get("engines") != "google" {
		t.Errorf("expected engines to be 'google', got '%s'", params.Get("engines"))
	}
}

func TestValidateInSlice_ErrorFormatting(t *testing.T) {
	err := validateInSlice("d", []string{"a", "b", "c"}, "test_param")
	expected := "test_param must be one of: [a b c]"
	if err == nil || err.Error() != expected {
		t.Errorf("expected error string '%s', got '%v'", expected, err)
	}
}

func TestValidateEngines_ErrorFormatting(t *testing.T) {
	engines := []Engine{"google", "invalid_engine"}
	err := validateEngines(engines)
	expected := "engine 'invalid_engine' is not supported. Valid engines are: [google duckduckgo baidu bing 360search yahoo quark]"
	if err == nil || err.Error() != expected {
		t.Errorf("expected error string '%s', got '%v'", expected, err)
	}
}

func TestSearchRequest_build_no_optionals(t *testing.T) {
	req := &SearchRequest{
		Query:  "test",
		PageNo: intPtr(1),
	}
	values := req.build(nil)
	if values.Get("time_range") != "" {
		t.Error("time_range should not be set")
	}
	if values.Get("language") != "" {
		t.Error("language should not be set")
	}
	if values.Get("safesearch") != "" {
		t.Error("safesearch should not be set")
	}
	if values.Get("engines") != "" {
		t.Error("engines should not be set")
	}
}

func TestNewClient_DefaultValues(t *testing.T) {
	cfg := &ClientConfig{BaseUrl: "http://test.com"}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.config.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", client.config.Timeout)
	}
	if client.config.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %v", client.config.MaxRetries)
	}
	if client.config.Headers == nil {
		t.Error("expected headers to be initialized, got nil")
	}
}

func TestSearxngClient_Search_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprintln(w, `{}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err = client.Search(ctx, &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestSearxngClient_Search_RequestCreationError(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://[::1]:namedport"}) // Invalid URL
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err == nil {
		t.Error("expected an error for request creation, got nil")
	}
}

func TestSearxngClient_sendRequestWithRetry_RateLimitSuccess(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempt == 0 {
			attempt++
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprintln(w, `{"results": [{"title":"t"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL, MaxRetries: 1, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.sendRequestWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestParseSearchResponse_EmptyResults(t *testing.T) {
	body := []byte(`{"query": "test", "results": []}`)
	resp, err := parseSearchResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NumberOfResults != 0 {
		t.Errorf("expected 0 results, got %d", resp.NumberOfResults)
	}
}

func TestValidateEngines_EmptyString(t *testing.T) {
	if err := validateEngines([]Engine{}); err != nil {
		t.Errorf("unexpected error for empty engines slice: %v", err)
	}
}

func TestSearxngClient_Search_DefaultUserAgent(t *testing.T) {
	var ua string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		fmt.Fprintln(w, `{"results": [{"title":"t"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	expectedUA := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	if ua != expectedUA {
		t.Errorf("expected user agent '%s', got '%s'", expectedUA, ua)
	}
}

func TestSearxngClient_sendRequestWithRetry_ParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `this is not json`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse search results") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func Test_validateInSlice(t *testing.T) {
	type args[T comparable] struct {
		value       T
		validValues []T
		paramName   string
	}
	type testCase[T comparable] struct {
		name    string
		args    args[T]
		wantErr bool
	}
	strTests := []testCase[string]{
		{
			name: "valid string",
			args: args[string]{
				value:       "a",
				validValues: []string{"a", "b", "c"},
				paramName:   "test",
			},
			wantErr: false,
		},
		{
			name: "invalid string",
			args: args[string]{
				value:       "d",
				validValues: []string{"a", "b", "c"},
				paramName:   "test",
			},
			wantErr: true,
		},
	}
	for _, tt := range strTests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateInSlice(tt.args.value, tt.args.validValues, tt.args.paramName); (err != nil) != tt.wantErr {
				t.Errorf("validateInSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	intTests := []testCase[int]{
		{
			name: "valid int",
			args: args[int]{
				value:       1,
				validValues: []int{1, 2, 3},
				paramName:   "test",
			},
			wantErr: false,
		},
		{
			name: "invalid int",
			args: args[int]{
				value:       4,
				validValues: []int{1, 2, 3},
				paramName:   "test",
			},
			wantErr: true,
		},
	}
	for _, tt := range intTests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateInSlice(tt.args.value, tt.args.validValues, tt.args.paramName); (err != nil) != tt.wantErr {
				t.Errorf("validateInSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchRequest_validate_params(t *testing.T) {
	type args struct {
		engines []Engine
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{[]Engine{}},
			wantErr: false,
		},
		{
			name:    "valid",
			args:    args{[]Engine{"google", "bing"}},
			wantErr: false,
		},
		{
			name:    "invalid",
			args:    args{[]Engine{"google", "foo"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateEngines(tt.args.engines); (err != nil) != tt.wantErr {
				t.Errorf("validateEngines() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parseSearchResponse(t *testing.T) {
	t.Run("no number of results", func(t *testing.T) {
		body := []byte(`{"results": [{"title": "t1"}, {"title": "t2"}]}`)
		res, err := parseSearchResponse(body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.NumberOfResults != 2 {
			t.Errorf("expected 2, got %d", res.NumberOfResults)
		}
	})
}

func TestSearxngClient_Search_WithHeader(t *testing.T) {
	var headerValue string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerValue = r.Header.Get("X-Test")
		fmt.Fprintln(w, `{"results": [{"title":"t"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		BaseUrl: server.URL,
		Headers: map[string]string{"X-Test": "test-value"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if headerValue != "test-value" {
		t.Errorf("expected header 'test-value', got '%s'", headerValue)
	}
}

func TestSearxngClient_sendRequestWithRetry_RetrySuccess(t *testing.T) {
	retryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryCount < 1 {
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"results":[{"title":"t"}]}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL, MaxRetries: 2, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retryCount != 1 {
		t.Errorf("expected 1 retry, got %d", retryCount)
	}
}

func Test_strPtr(t *testing.T) {
	s := "test"
	sp := strPtr(s)
	if *sp != s {
		t.Errorf("strPtr failed, expected %s, got %s", s, *sp)
	}
}

func Test_intPtr(t *testing.T) {
	i := 123
	ip := intPtr(i)
	if *ip != i {
		t.Errorf("intPtr failed, expected %d, got %d", i, *ip)
	}
}

func Test_enginePtr(t *testing.T) {
	// 测试单个引擎
	s := "google"
	ep := enginePtr(s)
	if len(ep) != 1 || string(ep[0]) != s {
		t.Errorf("enginePtr failed for single engine, expected [%s], got %v", s, ep)
	}

	// 测试多个引擎
	multi := "google,bing"
	ep = enginePtr(multi)
	if len(ep) != 2 || string(ep[0]) != "google" || string(ep[1]) != "bing" {
		t.Errorf("enginePtr failed for multiple engines, expected [google,bing], got %v", ep)
	}

	// 测试空字符串
	empty := ""
	ep = enginePtr(empty)
	if ep != nil {
		t.Errorf("enginePtr failed for empty string, expected nil, got %v", ep)
	}
}

func Test_BuildSearchInvokeTool(t *testing.T) {
	_, err := BuildSearchInvokeTool(&ClientConfig{BaseUrl: "http://localhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchRequest_validate_pageno(t *testing.T) {
	req := &SearchRequest{
		Query:  "test",
		PageNo: intPtr(0),
	}
	err := req.validate()
	if err == nil {
		t.Error("expected error for pageno <= 0")
	}
}

func TestSearchRequest_validate_time_range(t *testing.T) {
	req := &SearchRequestConfig{
		TimeRange: "invalid",
	}
	err := req.validate()
	if err == nil {
		t.Error("expected error for invalid time_range")
	}
}

func TestSearchRequest_validate_language(t *testing.T) {
	req := &SearchRequestConfig{
		Language: "invalid",
	}
	err := req.validate()
	if err == nil {
		t.Error("expected error for invalid language")
	}
}

func TestSearchRequest_validate_safesearch(t *testing.T) {
	req := &SearchRequestConfig{
		SafeSearch: 10,
	}
	err := req.validate()
	if err == nil {
		t.Error("expected error for invalid safesearch")
	}
}

func TestSearchRequest_validate_engines(t *testing.T) {
	req := &SearchRequestConfig{
		Engines: []Engine{"invalid"},
	}
	err := req.validate()
	if err == nil {
		t.Error("expected error for invalid engines")
	}
}

func TestSearchRequest_build_all_params(t *testing.T) {
	req := &SearchRequest{
		Query:  "test",
		PageNo: intPtr(2),
	}
	config := &SearchRequestConfig{
		TimeRange:  "year",
		Language:   "zh-CN",
		SafeSearch: 2,
		Engines:    []Engine{"google", "bing"},
	}
	v := req.build(config)
	if v.Get("q") != "test" {
		t.Errorf("wrong q value")
	}
	if v.Get("pageno") != "2" {
		t.Errorf("wrong pageno value")
	}
	if v.Get("time_range") != "year" {
		t.Errorf("wrong time_range value")
	}
	if v.Get("language") != "zh-CN" {
		t.Errorf("wrong language value")
	}
	if v.Get("safesearch") != "2" {
		t.Errorf("wrong safesearch value")
	}
	if v.Get("engines") != "google,bing" {
		t.Errorf("wrong engines value")
	}
	if v.Get("format") != "json" {
		t.Errorf("wrong format value")
	}
}

func TestNewClient_Proxy(t *testing.T) {
	cfg := &ClientConfig{
		BaseUrl:  "http://a.com",
		ProxyURL: "http://proxy.com",
	}
	// This test just ensures NewClient runs, but doesn't actually test proxy functionality
	// as that would require a running proxy server.
	_, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client with proxy: %v", err)
	}
}

func TestSearxngClient_Search_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{}`)
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err == nil || err.Error() != "no search results found" {
		t.Errorf("expected 'no search results found' error, got %v", err)
	}
}

func Test_validateInSlice_Comparable(t *testing.T) {
	if err := validateInSlice(1.0, []float64{1.0, 2.0}, "float"); err != nil {
		t.Errorf("unexpected error for float slice: %v", err)
	}
}

func Test_validateEngines_WithValid(t *testing.T) {
	validEngines := []string{"google", "bing"}
	err := validateInSlice("google", validEngines, "engine")
	if err != nil {
		t.Errorf("Expected no error for valid engine, but got %v", err)
	}
}

func Test_validateEngines_WithInvalid(t *testing.T) {
	validEngines := []string{"google", "bing"}
	err := validateInSlice("yahoo", validEngines, "engine")
	if err == nil {
		t.Error("Expected an error for invalid engine, but got nil")
	}
	expectedError := "engine must be one of: [google bing]"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func Test_parseSearchResponse_WithEmptyResults(t *testing.T) {
	jsonResponse := `{"results": []}`
	response, err := parseSearchResponse([]byte(jsonResponse))
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if response.NumberOfResults != 0 {
		t.Errorf("Expected NumberOfResults to be 0, but got %d", response.NumberOfResults)
	}
}

func Test_parseSearchResponse_WithNilResults(t *testing.T) {
	jsonResponse := `{}`
	response, err := parseSearchResponse([]byte(jsonResponse))
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if response.NumberOfResults != 0 {
		t.Errorf("Expected NumberOfResults to be 0, but got %d", response.NumberOfResults)
	}
}

func Test_BuildSearchInvokeTool_WithNilConfig(t *testing.T) {
	_, err := BuildSearchInvokeTool(nil)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
	expectedError := "config is nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}
}

func Test_SearxngClient_Search_WithEmptyBaseUrl(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: ""})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithNilContext(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://example.com"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err = client.sendRequestWithRetry(nil, req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithNilRequest(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://example.com"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = client.sendRequestWithRetry(context.Background(), nil)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_Search_WithNilContext(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://example.com"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = client.Search(nil, &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_Search_WithInvalidUrl(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://invalid-url"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	_, err = client.Search(context.Background(), &SearchRequest{Query: "test", PageNo: intPtr(1)})
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithInvalidUrl(t *testing.T) {
	client, err := NewClient(&ClientConfig{BaseUrl: "http://invalid-url"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", "http://invalid-url", nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()
	client, err := NewClient(&ClientConfig{BaseUrl: server.URL, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client, err := NewClient(&ClientConfig{BaseUrl: server.URL, MaxRetries: 0})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()
	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithBadJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":`)
	}))
	defer server.Close()
	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}

func Test_SearxngClient_sendRequestWithRetry_WithBadBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(1))
	}))
	defer server.Close()
	client, err := NewClient(&ClientConfig{BaseUrl: server.URL})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err = client.sendRequestWithRetry(context.Background(), req)
	if err == nil {
		t.Error("Expected an error, but got nil")
	}
}
