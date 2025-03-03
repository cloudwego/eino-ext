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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
)

func TestEmbedStrings(t *testing.T) {

	ctx := context.Background()

	embedClient, err := NewClient(ctx, &Config{
		ByAzure:    true,
		BaseURL:    "https://xxxx.com/api",
		APIKey:     "{your-api-key}",
		APIVersion: "2024-06-01",
		Model:      "gpt-4o-2024-05-13",
	})

	assert.NoError(t, err)

	defer mockey.Mock(mockey.GetMethod(embedClient.cli.Embeddings, "New")).Return(
		&openai.CreateEmbeddingResponse{
			Object: "xx",
			Data: []openai.Embedding{
				{
					Index:     0,
					Embedding: []float64{1, 2, 3},
				},
			},
			Model: openai.EmbeddingModelTextEmbeddingAda002,
			Usage: openai.CreateEmbeddingResponseUsage{
				PromptTokens: 100,
				TotalTokens:  400,
			},
		}, nil).Build().Patch().UnPatch()

	embeddings, err := embedClient.EmbedStrings(ctx, []string{"how are you"})
	assert.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Equal(t, []float64{1, 2, 3}, embeddings[0])
}
