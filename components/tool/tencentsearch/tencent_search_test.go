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

package tencentsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNewTool(t *testing.T) {
	ctx := context.Background()

	t.Run("nil config missing secret id", func(t *testing.T) {
		_, err := NewTool(ctx, nil)
		if err == nil || !strings.Contains(err.Error(), "secret_id is required") {
			t.Fatalf("expected secret_id error, got %v", err)
		}
	})

	t.Run("missing secret key", func(t *testing.T) {
		conf := &Config{SecretID: "test_id"}
		_, err := NewTool(ctx, conf)
		if err == nil || !strings.Contains(err.Error(), "secret_key is required") {
			t.Fatalf("expected secret_key error, got %v", err)
		}
	})

	t.Run("default name", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
		}
		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if info.Name != "tencent_search" {
			t.Errorf("expected name tencent_search, got %s", info.Name)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			ToolName:  "custom_tencent_search",
			ToolDesc:  "Custom description",
		}
		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if info.Name != "custom_tencent_search" {
			t.Errorf("expected name custom_tencent_search, got %s", info.Name)
		}
		if info.Desc != "Custom description" {
			t.Errorf("expected custom description, got %s", info.Desc)
		}
	})
}

func TestTencentSearch(t *testing.T) {
	ctx := context.Background()

	expectedCntByQuery := map[string]uint64{
		"eino framework":    10,
		"config defaults":   10,
		"config values":     20,
		"industry filter":   10,
		"request overrides": 20,
		"error":             10,
	}
	expectedModeByQuery := map[string]int64{
		"eino framework":    0,
		"config defaults":   0,
		"config values":     2,
		"industry filter":   0,
		"request overrides": 1,
		"error":             0,
	}
	expectedIndustryByQuery := map[string]string{
		"industry filter": "news",
	}
	requestCounts := map[string]int{}
	var mu sync.Mutex

	// Mock server for Tencent Cloud WSA API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and headers
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read request body
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		query, ok := reqBody["Query"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		requestCounts[query]++
		mu.Unlock()
		if expectedCnt, ok := expectedCntByQuery[query]; ok {
			gotCnt, ok := reqBody["Cnt"].(float64)
			if !ok || uint64(gotCnt) != expectedCnt {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if expectedMode, ok := expectedModeByQuery[query]; ok {
			gotMode, ok := reqBody["Mode"].(float64)
			if !ok || int64(gotMode) != expectedMode {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if expectedIndustry, ok := expectedIndustryByQuery[query]; ok {
			gotIndustry, ok := reqBody["Industry"].(string)
			if !ok || gotIndustry != expectedIndustry {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		if query == "error" {
			// return error format
			errResp := map[string]any{
				"Response": map[string]any{
					"Error": map[string]any{
						"Code":    "InternalError",
						"Message": "mock error",
					},
					"RequestId": "mock-req-id-err",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(errResp)
			return
		}

		// Success format
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Response":{"Query":"` + query + `","Pages":["{\"title\":\"Result 1\",\"url\":\"https://example.com/1\",\"passage\":\"Snippet 1\"}"],"RequestId":"mock-req-id"}}`))
	}))
	defer mockServer.Close()

	// Parse out the host from mockServer.URL (remove http://)
	endpoint := strings.TrimPrefix(mockServer.URL, "http://")

	t.Run("success search", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint, // point SDK to our mock server
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		req := &SearchRequest{
			Query: "eino framework",
		}
		reqJSON, _ := json.Marshal(req)

		resp, err := tl.InvokableRun(ctx, string(reqJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		var result SearchResult
		if err := json.Unmarshal([]byte(resp), &result); err != nil {
			t.Fatalf("expected valid json, got %v", err)
		}

		if result.Query != "eino framework" {
			t.Errorf("expected query 'eino framework', got %s", result.Query)
		}
		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
		if result.Items[0].Title != "Result 1" {
			t.Errorf("expected title 'Result 1', got %s", result.Items[0].Title)
		}
	})

	t.Run("default config uses default cnt and mode", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if conf.Cnt != defaultCnt {
			t.Fatalf("expected default cnt %d, got %d", defaultCnt, conf.Cnt)
		}
		if conf.Mode != defaultMode {
			t.Fatalf("expected default mode %d, got %d", defaultMode, conf.Mode)
		}

		req := &SearchRequest{Query: "config defaults"}
		reqJSON, _ := json.Marshal(req)

		if _, err = tl.InvokableRun(ctx, string(reqJSON)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("config values are used when request omits cnt and mode", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
			Cnt:       20,
			Mode:      2,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		req := &SearchRequest{Query: "config values"}
		reqJSON, _ := json.Marshal(req)

		if _, err = tl.InvokableRun(ctx, string(reqJSON)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		req := &SearchRequest{
			Query: "error",
		}
		reqJSON, _ := json.Marshal(req)

		_, err = tl.InvokableRun(ctx, string(reqJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		req := &SearchRequest{}
		reqJSON, _ := json.Marshal(req)

		_, err = tl.InvokableRun(ctx, string(reqJSON))
		if err == nil || !strings.Contains(err.Error(), "query is required") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("invalid config cnt returns error", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
			Cnt:       3,
		}

		_, err := NewTool(ctx, conf)
		if err == nil || !strings.Contains(err.Error(), "invalid cnt: 3") {
			t.Fatalf("expected invalid cnt error, got %v", err)
		}
	})

	t.Run("invalid config mode returns error", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
			Mode:      3,
		}

		_, err := NewTool(ctx, conf)
		if err == nil || !strings.Contains(err.Error(), "invalid mode: 3") {
			t.Fatalf("expected invalid mode error, got %v", err)
		}
	})

	t.Run("invalid request cnt returns error without sending request", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
			Cnt:       20,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		cnt := uint64(3)
		req := &SearchRequest{
			Query: "request invalid cnt",
			Cnt:   &cnt,
		}
		reqJSON, _ := json.Marshal(req)

		_, err = tl.InvokableRun(ctx, string(reqJSON))
		if err == nil || !strings.Contains(err.Error(), "invalid cnt: 3") {
			t.Fatalf("expected invalid cnt error, got %v", err)
		}
		mu.Lock()
		gotRequests := requestCounts["request invalid cnt"]
		mu.Unlock()
		if gotRequests != 0 {
			t.Fatalf("expected invalid request cnt to be blocked before request, got %d requests", gotRequests)
		}
	})

	t.Run("invalid request mode returns error without sending request", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
			Mode:      1,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		mode := int64(3)
		req := &SearchRequest{
			Query: "request invalid mode",
			Mode:  &mode,
		}
		reqJSON, _ := json.Marshal(req)

		_, err = tl.InvokableRun(ctx, string(reqJSON))
		if err == nil || !strings.Contains(err.Error(), "invalid mode: 3") {
			t.Fatalf("expected invalid mode error, got %v", err)
		}
		mu.Lock()
		gotRequests := requestCounts["request invalid mode"]
		mu.Unlock()
		if gotRequests != 0 {
			t.Fatalf("expected invalid request mode to be blocked before request, got %d requests", gotRequests)
		}
	})

	t.Run("industry passthrough", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		industry := "news"
		req := &SearchRequest{
			Query:    "industry filter",
			Industry: &industry,
		}
		reqJSON, _ := json.Marshal(req)

		if _, err = tl.InvokableRun(ctx, string(reqJSON)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("valid request cnt and mode passthrough", func(t *testing.T) {
		conf := &Config{
			SecretID:  "test_id",
			SecretKey: "test_key",
			Endpoint:  endpoint,
		}

		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		cnt := uint64(20)
		mode := int64(1)
		req := &SearchRequest{
			Query: "request overrides",
			Cnt:   &cnt,
			Mode:  &mode,
		}
		reqJSON, _ := json.Marshal(req)

		if _, err = tl.InvokableRun(ctx, string(reqJSON)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}
