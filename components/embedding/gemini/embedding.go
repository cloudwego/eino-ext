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

package gemini

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"google.golang.org/genai"
)

// EmbeddingConfig contains the configuration for the Gemini embedding model.
type EmbeddingConfig struct {
	// Client is the Gemimi API client instance
	// Required for making API calls to Gemini
	Client *genai.Client

	// Model specifies which Gemini embedding model to use
	// Examples: "gemini-embedding-001", "gemini-embedding-exp-03-07"
	Model string

	// Type of task for which the embedding will be used.
	TaskType string

	// Title for the text. Only applicable when TaskType is
	// `RETRIEVAL_DOCUMENT`.
	Title string

	// Reduced dimension for the output embedding. If set,
	// excessive values in the output embedding are truncated from the end.
	// Supported by newer models since 2024 only. You cannot set this value if
	// using the earlier model (`models/embedding-001`).
	OutputDimensionality *int32

	// Vertex API only. The MIME type of the input.
	MIMEType string `json:"mimeType,omitempty"`

	// Vertex API only. Whether to silently truncate inputs longer than
	// the max sequence length. If this option is set to false, oversized inputs
	// will lead to an INVALID_ARGUMENT error, similar to other text APIs.
	AutoTruncate bool `json:"autoTruncate,omitempty"`
}

type Embedder struct {
	cli *genai.Client

	conf *EmbeddingConfig
}

func NewEmbedder(ctx context.Context, cfg *EmbeddingConfig) (*Embedder, error) {
	return &Embedder{
		cli:  cfg.Client,
		conf: cfg,
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

	embedContentConfig := &genai.EmbedContentConfig{
		TaskType:             e.conf.TaskType,
		Title:                e.conf.Title,
		OutputDimensionality: e.conf.OutputDimensionality,
		MIMEType:             e.conf.MIMEType,
		AutoTruncate:         e.conf.AutoTruncate,
	}

	resp, err := e.cli.Models.EmbedContent(ctx,
		e.conf.Model,
		contents,
		embedContentConfig,
	)
	if err != nil {
		return nil, err
	}

	// Convert [][]float32 to [][]float64
	embeddings = make([][]float64, len(resp.Embeddings))
	var tokenUsage *embedding.TokenUsage
	for i, emb := range resp.Embeddings {
		embeddings[i] = make([]float64, len(emb.Values))
		for j, v := range emb.Values {
			embeddings[i][j] = float64(v)
		}
		if emb.Statistics != nil {
			if tokenUsage == nil {
				tokenUsage = &embedding.TokenUsage{
					PromptTokens: int(emb.Statistics.TokenCount),
				}
			} else {
				tokenUsage.PromptTokens += int(emb.Statistics.TokenCount)
			}
		}
	}

	callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: tokenUsage,
	})

	return embeddings, nil
}

func (e *Embedder) GetType() string {
	return "Gemini"
}

func (e *Embedder) IsCallbacksEnabled() bool {
	return true
}
