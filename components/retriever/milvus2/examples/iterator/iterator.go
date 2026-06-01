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

// This example demonstrates Iterator Search for traversing large result sets.
//
// Prerequisites:
// Run the demo indexer first to create the collection:
//
//	cd ../../../indexer/milvus2/examples/demo && go run .
//
// The collection "demo_milvus2" has 60 documents for testing batch retrieval.
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

	// Fetch 25 items using small batch size to force iteration
	totalTopK := 25
	batchSize := 10

	// Create retriever with Iterator search mode
	// Iterator fetches results in batches to handle large result sets efficiently.
	// BatchSize controls how many items are fetched per network call.
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   collectionName,
		VectorField:  "vector",
		OutputFields: []string{"id", "content"},
		TopK:         totalTopK,
		SearchMode:   search_mode.NewIterator(milvus2.COSINE, batchSize),
		Embedding:    &mockEmbedding{dim: dim},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	log.Printf("Starting Iterator Search (TopK=%d, BatchSize=%d)", totalTopK, batchSize)
	results, err := retriever.Retrieve(ctx, "query")
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	log.Printf("Found %d documents (Expected %d)", len(results), totalTopK)
	for i, doc := range results {
		if i == 0 || i == len(results)-1 {
			log.Printf("Result %d: ID=%s, Content=%s, Score=%.4f", i+1, doc.ID, doc.Content, doc.Score())
		}
	}

	if len(results) != totalTopK {
		log.Fatalf("FAILED: expected %d results, got %d", totalTopK, len(results))
	} else {
		log.Println("SUCCESS: Retrieved expected number of documents via iterator")
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
