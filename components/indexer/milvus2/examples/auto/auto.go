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

// This example demonstrates using AUTOINDEX for vector storage.
// AUTOINDEX lets Milvus automatically choose the optimal index type
// based on data characteristics - simplest to use.
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
)

func main() {
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}

	ctx := context.Background()

	// Create an indexer with AUTOINDEX (default when IndexBuilder is nil)
	// Milvus will automatically select the best index type
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   "demo_auto",
		Vector: &milvus2.VectorConfig{
			VectorField: "vector",
			Dimension:   128,
			MetricType:  milvus2.IP, // Inner Product
			// IndexBuilder: nil means AUTOINDEX is used
		},
		Embedding: &mockEmbedding{dim: 128},
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}
	log.Println("AUTOINDEX indexer created successfully")

	// Store sample documents
	docs := []*schema.Document{
		{ID: "auto-1", Content: "AUTOINDEX automatically selects the best index"},
		{ID: "auto-2", Content: "Simplest option for getting started"},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
	}
	log.Printf("Stored documents: %v", ids)
}

// mockEmbedding generates random embeddings for demonstration
type mockEmbedding struct{ dim int }

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dim)
		for j := range vec {
			vec[j] = float64(i+j) * 0.01
		}
		result[i] = vec
	}
	return result, nil
}
