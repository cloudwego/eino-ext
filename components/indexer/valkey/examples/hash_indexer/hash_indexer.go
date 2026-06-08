//go:build ignore

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

package main

import (
	"context"
	"fmt"

	glide "github.com/valkey-io/valkey-glide/go/v2"
	"github.com/valkey-io/valkey-glide/go/v2/config"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"

	vi "github.com/cloudwego/eino-ext/components/indexer/valkey"
)

// This example demonstrates indexing documents as Valkey Hashes.
// Prerequisites:
//   - Valkey 9.1+ with Search module running on localhost:6379
//   - Create an index first:
//     FT.CREATE my_index ON HASH PREFIX 1 doc: SCHEMA
//     content TEXT vector_content VECTOR HNSW 6 TYPE FLOAT32 DIM 1024 DISTANCE_METRIC COSINE
func main() {
	ctx := context.Background()

	// 1. Create Valkey GLIDE client
	cfg := config.NewClientConfiguration().
		WithAddress(&config.NodeAddress{Host: "localhost", Port: 6379})
	client, err := glide.NewClient(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create client: %v", err))
	}
	defer client.Close()

	// 2. Create indexer
	indexer, err := vi.NewIndexer(ctx, &vi.IndexerConfig{
		Client:       client,
		KeyPrefix:    "doc:",
		DocumentType: vi.DocumentTypeHash,
		BatchSize:    10,
		Embedding:    &mockEmbedding{dims: 1024}, // replace with real embedder
	})
	if err != nil {
		panic(err)
	}

	// 3. Store documents
	docs := []*schema.Document{
		{ID: "1", Content: "Eiffel Tower: Located in Paris, France"},
		{ID: "2", Content: "Great Wall of China: One of the greatest wonders of the world"},
		{ID: "3", Content: "Grand Canyon: Located in Arizona, USA"},
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Stored %d documents: %v\n", len(ids), ids)
}

// mockEmbedding is a placeholder - replace with a real embedding implementation.
type mockEmbedding struct {
	dims int
}

func (m *mockEmbedding) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = make([]float64, m.dims)
	}
	return vectors, nil
}
