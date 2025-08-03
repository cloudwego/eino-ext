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
 * distributed under the License is an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gemini

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"google.golang.org/genai"
)

type EmbeddingConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

type Embedder struct {
	client *genai.Client
	conf   *EmbeddingConfig
}

func buildClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	return genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
}

func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (*Embedder, error) {
	client, err := buildClient(ctx, config.APIKey)
	if err != nil {
		return nil, err
	}

	return &Embedder{
		client: client,
		conf:   config,
	}, nil
}

func (e *Embedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (embeddings [][]float64, err error) {
	options := embedding.GetCommonOptions(&embedding.Options{
		Model: &e.conf.Model,
	}, opts...)

	conf := &embedding.Config{
		Model: *options.Model,
	}

	ctx = callbacks.EnsureRunInfo(ctx, e.GetType(), components.ComponentOfEmbedding)
	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	contents := make([]*genai.Content, 0, len(texts))
	for _, text := range texts {
		contents = append(contents, genai.NewContentFromText(text, genai.RoleUser))
	}
	resp, err := e.client.Models.EmbedContent(ctx,
		e.conf.Model,
		contents,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Convert [][]float32 to [][]float64
	embeddings = make([][]float64, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embeddings[i] = make([]float64, len(emb.Values))
		for j, v := range emb.Values {
			embeddings[i][j] = float64(v)
		}
	}

	callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		// gemini embedding does not return token usage
	})

	return embeddings, nil
}

func (e *Embedder) GetType() string {
	return "Gemini"
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
