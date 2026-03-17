package sougousearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewTool(t *testing.T) {
	ctx := context.Background()

	t.Run("default config", func(t *testing.T) {
		tl, err := NewTool(ctx, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tl == nil {
			t.Fatal("expected tool, got nil")
		}

		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if info.Name != "sougou_search" {
			t.Errorf("expected name sougou_search, got %s", info.Name)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		conf := &Config{
			ToolName: "custom_sougou",
			ToolDesc: "Custom description",
		}
		tl, err := NewTool(ctx, conf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		info, err := tl.Info(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if info.Name != "custom_sougou" {
			t.Errorf("expected name custom_sougou, got %s", info.Name)
		}
		if info.Desc != "Custom description" {
			t.Errorf("expected custom description, got %s", info.Desc)
		}
	})
}

func TestSougouSearch(t *testing.T) {
	ctx := context.Background()

	// Mock server for Tencent Cloud WSA API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and headers
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		query, ok := reqBody["Query"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if query == "error" {
			// return error format
			errResp := map[string]interface{}{
				"Response": map[string]interface{}{
					"Error": map[string]interface{}{
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
		itemJSON := `{"title":"Result 1","url":"https://example.com/1","passage":"Snippet 1"}`
		res := map[string]interface{}{
			"Response": map[string]interface{}{
				"Query":     query,
				"Pages":     []string{itemJSON},
				"RequestId": "mock-req-id",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
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
}
