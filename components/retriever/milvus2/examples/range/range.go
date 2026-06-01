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

// This example demonstrates range search (searching within a similarity radius).
//
// Prerequisites:
// Run the hnsw indexer first to create the collection:
//
//	cd ../../../indexer/milvus2/examples/hnsw && go run .
//
// The collection "demo_hnsw" will be used for search.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

func main() {
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}

	ctx := context.Background()

	// 1. Create Retriever with Range Search
	// Range search returns all vectors within a similarity/distance radius.
	// For COSINE: radius is minimum similarity (0.99 = very similar).
	// For L2: radius is maximum distance.
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   "demo_hnsw", // Using collection from hnsw example
		VectorField:  "vector",
		OutputFields: []string{"id", "content", "metadata"},
		TopK:         100, // TopK acts as a limit for range search results
		SearchMode:   search_mode.NewRange(milvus2.COSINE, 0.99),
		Embedding:    &mockEmbedding{dim: 128},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Range Retriever created successfully")

	// 2. Perform Search
	// This should return documents that are very close to the query
	docs, err := retriever.Retrieve(ctx, "vector search query")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	fmt.Printf("\nFound %d documents within radius 0.99:\n", len(docs))
	for i, doc := range docs {
		fmt.Printf("\n--- Document %d ---\n", i+1)
		fmt.Printf("ID: %s\n", doc.ID)
		fmt.Printf("Score: %v\n", doc.Score())
	}
}

// mockEmbedding generates embeddings for demonstration
type mockEmbedding struct{ dim int }

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dim)
		for j := range vec {
			// Matches the logic in hnsw example to ensure high similarity
			vec[j] = float64(j) * 0.01
		}
		result[i] = vec
	}
	return result, nil
}
