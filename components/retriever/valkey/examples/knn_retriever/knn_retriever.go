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

	vr "github.com/cloudwego/eino-ext/components/retriever/valkey"
)

// This example demonstrates using the Valkey retriever for KNN vector search.
// Prerequisites:
//   - Valkey 9.1+ with Search module running on localhost:6379
//   - An index created with: FT.CREATE my_index ON HASH PREFIX 1 doc: SCHEMA
//     content TEXT vector_content VECTOR HNSW 6 TYPE FLOAT32 DIM 1024 DISTANCE_METRIC COSINE
//   - Documents indexed (see indexer example)
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

	// 2. Create retriever with your embedding model
	r, err := vr.NewRetriever(ctx, &vr.RetrieverConfig{
		Client:    client,
		Index:     "my_index",
		TopK:      5,
		Embedding: &mockEmbedding{dims: 1024}, // replace with real embedder
	})
	if err != nil {
		panic(err)
	}

	// 3. Retrieve documents
	docs, err := r.Retrieve(ctx, "tourist attractions in Europe")
	if err != nil {
		panic(err)
	}

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}

	// 4. Retrieve with filter
	docs, err = r.Retrieve(ctx, "tourist attractions",
		vr.WithFilterQuery("@category:{europe}"))
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nFiltered results: %d\n", len(docs))
	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}
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
