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

// This example demonstrates Hybrid Search combining Dense and Sparse vectors.
// Dense vectors capture semantic meaning, while sparse vectors enable keyword matching.
// Results from both searches are fused using RRFReranker.
//
// Prerequisites:
// - A Milvus collection with both dense and sparse vector fields.
// - Use the indexer sparse example to create such a collection.
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

	// 1. Define Reranker
	// RRFReranker (Reciprocal Rank Fusion) combines scores from multiple searches.
	reranker := milvusclient.NewRRFReranker().WithK(60)

	// 2. Define Hybrid Mode with Dense + Sparse SubRequests
	hybridMode := search_mode.NewHybrid(reranker,
		// Dense vector search (semantic similarity)
		&search_mode.SubRequest{
			VectorField: "vector",            // Dense vector field
			VectorType:  milvus2.DenseVector, // Default, can be omitted
			TopK:        10,
			MetricType:  milvus2.L2, // Must match index metric type
		},
		// Sparse vector search (keyword matching)
		&search_mode.SubRequest{
			VectorField: "sparse_vector", // Sparse vector field
			VectorType:  milvus2.SparseVector,
			TopK:        10,
			MetricType:  milvus2.IP, // Sparse uses IP metric
		},
	)

	// 3. Create Retriever with both Dense and Sparse Embedders
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig:    &milvusclient.ClientConfig{Address: addr},
		Collection:      "eino_sparse_test", // Collection created by indexer sparse example
		VectorField:     "vector",
		OutputFields:    []string{"id", "content", "metadata"},
		TopK:            5,
		SearchMode:      hybridMode,
		Embedding:       &mockDenseEmbedding{dim: 128}, // Dense embedder
		SparseEmbedding: &mockSparseEmbedding{},        // Sparse embedder
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Hybrid (Dense+Sparse) Retriever created successfully")

	// 4. Perform Search
	// The query is embedded using both dense and sparse embedders.
	docs, err := retriever.Retrieve(ctx, "machine learning algorithms")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	fmt.Printf("\nFound %d documents (Hybrid Dense+Sparse Fused):\n", len(docs))
	for i, doc := range docs {
		fmt.Printf("\n--- Document %d ---\n", i+1)
		fmt.Printf("ID: %s\n", doc.ID)
		fmt.Printf("Content: %s\n", doc.Content)
		fmt.Printf("Score: %v\n", doc.MetaData["score"])
	}
}

// mockDenseEmbedding generates dense embeddings for demonstration
type mockDenseEmbedding struct{ dim int }

func (m *mockDenseEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
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

// mockSparseEmbedding generates sparse embeddings for demonstration
// In production, use a real sparse embedding model (e.g., SPLADE, BM25).
type mockSparseEmbedding struct{}

func (m *mockSparseEmbedding) EmbedStrings(ctx context.Context, texts []string) ([]map[int]float64, error) {
	result := make([]map[int]float64, len(texts))
	for i, text := range texts {
		// Simple mock: use character codes as sparse indices
		sparse := make(map[int]float64)
		for j, c := range text {
			if j < 10 { // Limit to first 10 characters
				sparse[int(c)] = float64(j+1) * 0.1
			}
		}
		result[i] = sparse
	}
	return result, nil
}
