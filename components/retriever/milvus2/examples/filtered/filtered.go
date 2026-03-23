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

// This example demonstrates filtered vector search using Milvus boolean expressions.
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

	// Create retriever
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   "demo_hnsw",
		VectorField:  "vector",
		OutputFields: []string{"id", "content"},
		TopK:         10,
		SearchMode:   search_mode.NewApproximate(milvus2.COSINE),
		Embedding:    &mockEmbedding{dim: 128},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Retriever created successfully")

	// Search without filter
	fmt.Println("=== Search without filter ===")
	docs, err := retriever.Retrieve(ctx, "vector search")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}
	fmt.Printf("Found %d documents\n", len(docs))
	for _, doc := range docs {
		fmt.Printf("- %s: %s\n", doc.ID, doc.Content)
	}

	// Search with filter on ID field
	// Milvus filter syntax: https://milvus.io/docs/boolean.md
	fmt.Println("\n=== Search with filter (id == 'hnsw-1') ===")
	filteredDocs, err := retriever.Retrieve(ctx, "vector search",
		milvus2.WithFilter("id == 'hnsw-1'"))
	if err != nil {
		log.Fatalf("Failed to retrieve with filter: %v", err)
	}
	fmt.Printf("Found %d documents\n", len(filteredDocs))
	for _, doc := range filteredDocs {
		fmt.Printf("- %s: %s\n", doc.ID, doc.Content)
	}

	// More filter examples (uncomment to try):
	// milvus2.WithFilter("id in ['hnsw-1', 'hnsw-2']")  // IN clause
	// milvus2.WithFilter("id like 'hnsw%'")            // LIKE pattern
}

// mockEmbedding generates embeddings for demonstration
type mockEmbedding struct{ dim int }

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dim)
		for j := range vec {
			vec[j] = float64(j) * 0.01
		}
		result[i] = vec
	}
	return result, nil
}
