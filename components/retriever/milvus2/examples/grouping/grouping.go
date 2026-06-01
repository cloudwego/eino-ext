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

// This example demonstrates Grouping Search (returning top results per group).
//
// Prerequisites:
// Run the demo indexer first to create the collection:
//
//	cd ../../../indexer/milvus2/examples/demo && go run .
//
// The collection "demo_milvus2" has a "category" field for grouping.
package main

import (
	"context"
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
	collectionName := "demo_milvus2"
	dim := 128

	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   collectionName,
		VectorField:  "vector",
		OutputFields: []string{"id", "content", "category"},
		TopK:         10,
		SearchMode:   search_mode.NewApproximate(milvus2.COSINE),
		Embedding:    &mockEmbedding{dim: dim},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Retriever created successfully")

	// Perform grouping search (group by category, max 2 per group)
	results, err := retriever.Retrieve(ctx, "search query",
		milvus2.WithGrouping("category", 2, false),
	)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	log.Printf("Found %d documents (grouped by category, max 2 per group)", len(results))
	for i, doc := range results {
		cat := doc.MetaData["category"]
		log.Printf("  %d. ID=%s, Category=%v", i+1, doc.ID, cat)
	}
}

type mockEmbedding struct{ dim int }

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dim)
		for j := range vec {
			vec[j] = 0.1
		}
		result[i] = vec
	}
	return result, nil
}
