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

package search_mode

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
)

// EmbedQuery embeds the query string into a vector.
// It handles the callback context for the embedding operation.
func EmbedQuery(ctx context.Context, emb embedding.Embedder, query string) ([]float32, error) {
	if emb == nil {
		return nil, fmt.Errorf("[Retriever] embedding not provided")
	}

	runInfo := &callbacks.RunInfo{
		Component: components.ComponentOfEmbedding,
	}

	if embType, ok := components.GetType(emb); ok {
		runInfo.Type = embType
	}

	runInfo.Name = runInfo.Type + string(runInfo.Component)

	// Wrap the context with callback info
	ctx = callbacks.ReuseHandlers(ctx, runInfo)

	vectors, err := emb.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("[Retriever] failed to embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("[Retriever] invalid embedding result: expected 1, got %d", len(vectors))
	}

	queryVector := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		queryVector[i] = float32(v)
	}
	return queryVector, nil
}
