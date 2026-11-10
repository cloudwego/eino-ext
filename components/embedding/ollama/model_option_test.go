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

package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/stretchr/testify/require"
)

func TestEmbedStringsModelOption(t *testing.T) {
	models := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		models <- request.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[1,2]]}`))
	}))
	defer server.Close()
	ctx := context.Background()
	embedder, err := NewEmbedder(ctx, &EmbeddingConfig{BaseURL: server.URL, Model: "default-model", HTTPClient: server.Client()})
	require.NoError(t, err)
	result, err := embedder.EmbedStrings(ctx, []string{"hello"}, embedding.WithModel("override-model"))
	require.NoError(t, err)
	require.Equal(t, [][]float64{{1, 2}}, result)
	_, err = embedder.EmbedStrings(ctx, []string{"hello"})
	require.NoError(t, err)
	require.Equal(t, []string{"override-model", "default-model"}, []string{<-models, <-models})
}
