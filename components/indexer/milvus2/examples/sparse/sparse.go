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

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// This example demonstrates Sparse-only indexing using BM25.
// Milvus will automatically generate sparse vectors from the text content.
// No dense vector configuration is provided.
//
// Prerequisites:
// - Milvus 2.5+ (required for server-side functions)

func main() {
	ctx := context.Background()
	collectionName := "eino_sparse_test"
	milvusAddr := "localhost:19530"
	sparseField := "sparse_vector"

	// Create indexer with BM25 function
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address: milvusAddr,
		},
		Collection: collectionName,
		// Vector: nil, // Explicitly nil implies no dense vector

		// BM25 requires analyzer on content field.
		FieldParams: map[string]map[string]string{
			"content": {
				"enable_analyzer": "true",
				"analyzer_params": `{"type": "standard"}`,
			},
		},
		// Functions: Auto-generated for BM25 when Sparse is set
		Sparse: &milvus2.SparseVectorConfig{
			VectorField: sparseField,
			MetricType:  milvus2.BM25,
			Method:      milvus2.SparseMethodAuto,
		},
		// Embedding is required by interface but not used if Vector is nil
		Embedding: &mockEmbedding{},
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	// Store documents
	ids, err := indexer.Store(ctx, []*schema.Document{
		{ID: "1", Content: "Milvus is a vector database."},
		{ID: "2", Content: "BM25 is a ranking function used in information retrieval."},
		{ID: "3", Content: "Sparse vectors are good for keyword search."},
	})
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
	}

	fmt.Printf("Stored %d documents: %v\n", len(ids), ids)
	fmt.Println("Use the retriever sparse example to search this collection.")
}

// mockEmbedding for demo purposes (satisfies interface only)
type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	// Not used for sparse-only indexing
	return make([][]float64, len(texts)), nil
}
