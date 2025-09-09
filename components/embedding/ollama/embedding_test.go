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

package ollama

import (
	"context"
	"fmt"
	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	callbacksHelper "github.com/cloudwego/eino/utils/callbacks"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
)

func TestEmbedding(t *testing.T) {
	model := "nomic-embed-text"
	expectedRequest := &api.EmbedRequest{
		Model: model,
		Input: []string{"hello world"},
	}
	mockEmbeddings := [][]float32{{-0.006788351107388735, -0.0013259865809231997, -0.17136646807193756, 0.008495260030031204}}
	mockResponse := &api.EmbedResponse{
		Model:           model,
		Embeddings:      mockEmbeddings,
		TotalDuration:   30318500, // 30.3185ms
		LoadDuration:    17898200, //  17.8982ms
		PromptEvalCount: 2,
	}

	expectedDimensions := len(mockEmbeddings[0])

	t.Run("invalid param - missing config", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, nil)

		assert.NotNil(t, err)
	})

	t.Run("invalid param - invalid base_url", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, &EmbeddingConfig{
			BaseURL: "http://example.com:port",
			Model:   model,
			Timeout: 10 * time.Second,
		})

		assert.NotNil(t, err)
	})

	t.Run("optional base_url", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewEmbedder(ctx, &EmbeddingConfig{
			BaseURL: "",
			Model:   model,
			Timeout: 10 * time.Second,
		})

		assert.Nil(t, err)
	})

	t.Run("invalid param - missing model", func(t *testing.T) {
		ctx := context.Background()
		emb, err := NewEmbedder(ctx, &EmbeddingConfig{
			BaseURL: defaultBaseUrl,
			Timeout: 10 * time.Second,
		})

		_, err = emb.EmbedStrings(ctx, []string{"hello world"})

		assert.NotNil(t, err)
	})

	t.Run("full param", func(t *testing.T) {
		ctx := context.Background()
		emb, err := NewEmbedder(ctx, &EmbeddingConfig{
			BaseURL: "http://localhost:11434",
			Model:   model,
			Timeout: 10 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}

		defer mockey.Mock((*api.Client).Embed).To(func(ctx context.Context, req *api.EmbedRequest) (res *api.EmbedResponse, err error) {
			if !reflect.DeepEqual(req, expectedRequest) {
				t.Fatal("ollama embedding request is unexpected")
			}
			return mockResponse, nil
		}).Build().UnPatch()

		callbackHandler := &callbacksHelper.EmbeddingCallbackHandler{
			OnStart: func(ctx context.Context, runInfo *callbacks.RunInfo, input *embedding.CallbackInput) context.Context {
				if !reflect.DeepEqual(input.Texts, expectedRequest.Input) {
					t.Fatal("request.Texts is unexpected")
				}
				if !reflect.DeepEqual(input.Config.Model, expectedRequest.Model) {
					t.Fatal("Ollama embedding request is unexpected")
				}
				return ctx
			},
			OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *embedding.CallbackOutput) context.Context {
				assert.Equal(t, len(output.Embeddings[0]), expectedDimensions)
				if !reflect.DeepEqual(output.Extra, map[string]any{
					TotalDuration:   mockResponse.TotalDuration,
					LoadDuration:    mockResponse.LoadDuration,
					PromptEvalCount: mockResponse.PromptEvalCount,
				}) {
					t.Fatal("Ollama embedding response is unexpected")
				}
				return ctx
			},
			OnError: func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context {
				fmt.Printf("Ollama embedding error: %v", err)
				return ctx
			},
		}

		handler := callbacksHelper.NewHandlerHelper().
			Embedding(callbackHandler).
			Handler()

		chain := compose.NewChain[[]string, [][]float64]()
		chain.AppendEmbedding(emb)

		run, err := chain.Compile(ctx)
		if err != nil {
			return
		}

		outEmbeddings, err := run.Invoke(ctx, (expectedRequest.Input).([]string), compose.WithCallbacks(handler))
		if err != nil {
			t.Fatalf("run.Invoke failed, err=%v", err)
		}

		assert.Equal(t, len(outEmbeddings[0]), expectedDimensions)
	})
}
