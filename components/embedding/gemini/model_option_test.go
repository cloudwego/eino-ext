/*
 * Copyright 2026 CloudWeGo Authors
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

package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"google.golang.org/genai"
)

func TestEmbedStringsModelOption(t *testing.T) {
	type observedRequest struct{ path, model string }
	requests := make(chan observedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Requests []struct {
				Model string `json:"model"`
			} `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Requests) != 1 {
			t.Errorf("invalid embedding request: %+v, error: %v", request, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- observedRequest{path: r.URL.Path, model: request.Requests[0].Model}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[{"values":[1,2]}]}`))
	}))
	defer server.Close()
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI, APIKey: "test-key", HTTPClient: server.Client(),
		HTTPOptions: genai.HTTPOptions{BaseURL: server.URL, APIVersion: "v1beta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	embedder, err := NewEmbedder(ctx, &EmbeddingConfig{Client: client, Model: "default-model"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := embedder.EmbedStrings(ctx, []string{"hello"}, embedding.WithModel("override-model"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, [][]float64{{1, 2}}) {
		t.Fatalf("unexpected embeddings: %v", result)
	}
	_, err = embedder.EmbedStrings(ctx, []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	got := []observedRequest{<-requests, <-requests}
	want := []observedRequest{
		{path: "/v1beta/models/override-model:batchEmbedContents", model: "models/override-model"},
		{path: "/v1beta/models/default-model:batchEmbedContents", model: "models/default-model"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %+v, want %+v", got, want)
	}
}
