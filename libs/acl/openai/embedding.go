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

package openai

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/openai/openai-go"
)

type EmbeddingEncodingFormat string

const (
	EmbeddingEncodingFormatFloat  EmbeddingEncodingFormat = "float"
	EmbeddingEncodingFormatBase64 EmbeddingEncodingFormat = "base64"
)

func (cm *Client) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) (
	embeddings [][]float64, err error) {

	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	options := &embedding.Options{
		Model: &cm.config.Model,
	}
	options = embedding.GetCommonOptions(options, opts...)

	req := &openai.EmbeddingNewParams{
		Input: openai.F[openai.EmbeddingNewParamsInputUnion](openai.EmbeddingNewParamsInputArrayOfStrings(texts)),
		Model: openai.F(*options.Model),
	}
	if cm.config.User != nil {
		req.User = openai.F(*cm.config.User)
	}
	if cm.config.EncodingFormat != nil {
		req.EncodingFormat = openai.F(openai.EmbeddingNewParamsEncodingFormat(*cm.config.EncodingFormat))
	} else {
		req.EncodingFormat = openai.F(openai.EmbeddingNewParamsEncodingFormat(EmbeddingEncodingFormatFloat))
	}
	if cm.config.Dimensions != nil {
		req.Dimensions = openai.F(int64(*cm.config.Dimensions))
	}

	conf := &embedding.Config{
		Model:          req.Model.Value,
		EncodingFormat: string(req.EncodingFormat.Value),
	}

	ctx = callbacks.OnStart(ctx, &embedding.CallbackInput{
		Texts:  texts,
		Config: conf,
	})

	resp, err := cm.cli.Embeddings.New(ctx, *req)
	if err != nil {
		return nil, err
	}

	embeddings = make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		res := make([]float64, len(d.Embedding))
		for j, emb := range d.Embedding {
			res[j] = emb
		}
		embeddings[i] = res
	}

	usage := &embedding.TokenUsage{
		PromptTokens: int(resp.Usage.PromptTokens),
		TotalTokens:  int(resp.Usage.TotalTokens),
	}

	_ = callbacks.OnEnd(ctx, &embedding.CallbackOutput{
		Embeddings: embeddings,
		Config:     conf,
		TokenUsage: usage,
	})

	return embeddings, nil
}
