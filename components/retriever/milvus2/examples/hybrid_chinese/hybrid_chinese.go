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

// This example demonstrates Chinese text retrieval using BM25 sparse vectors in Hybrid mode.
// Run the indexer hybrid_chinese example first to create the collection:
//
//	cd ../../../indexer/milvus2/examples/hybrid_chinese && go run .
//
// Requires Milvus 2.5+ for server-side BM25 function support.
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

	// Collection created by indexer hybrid_chinese example
	collectionName := "eino_hybrid_chinese"
	denseField := "vector"
	sparseField := "sparse_vector"

	// Define Reranker: RRF combines scores from multiple searches
	reranker := milvusclient.NewRRFReranker().WithK(60)

	// Define Hybrid Mode with Dense + Sparse (BM25) SubRequests
	hybridMode := search_mode.NewHybrid(reranker,
		// Dense vector search (semantic similarity)
		&search_mode.SubRequest{
			VectorField: denseField,
			VectorType:  milvus2.DenseVector,
			TopK:        10,
			MetricType:  milvus2.L2,
		},
		// Sparse vector search (BM25 keyword matching)
		// Uses raw query text - Milvus generates sparse vector server-side
		&search_mode.SubRequest{
			VectorField: sparseField,
			VectorType:  milvus2.SparseVector,
			TopK:        10,
			MetricType:  milvus2.BM25,
		},
	)

	// Create Retriever
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig:      &milvusclient.ClientConfig{Address: addr},
		Collection:        collectionName,
		VectorField:       denseField,
		SparseVectorField: sparseField,
		OutputFields:      []string{"id", "content", "metadata"},
		TopK:              5,
		SearchMode:        hybridMode,
		Embedding:         &mockDenseEmbedding{dim: 128},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Chinese Hybrid Retriever created successfully")

	// Search with Chinese query text
	// - Dense: text is embedded via mockDenseEmbedding
	// - Sparse: text is passed directly to Milvus for BM25 processing with Chinese tokenizer
	queries := []string{
		"向量数据库",
		"AI 应用",
		"框架",
	}

	for _, query := range queries {
		fmt.Printf("\n=== Query: %s ===\n", query)

		docs, err := retriever.Retrieve(ctx, query)
		if err != nil {
			log.Fatalf("Failed to retrieve: %v", err)
		}

		fmt.Printf("Found %d documents:\n", len(docs))
		for i, doc := range docs {
			fmt.Printf("%d. [ID: %s] [Score: %.4f] %s\n", i+1, doc.ID, doc.Score(), doc.Content)
		}
	}
}

// mockDenseEmbedding for demo purposes
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
