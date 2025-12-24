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
	"math/rand"
	"os"

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
	collectionName := "eino_sparse_test"
	// For hybrid mode (dense + sparse), set dim > 0.
	// For sparse-only mode, set dim = 0.
	dim := 128

	// Create a new Milvus client
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: addr})
	if err != nil {
		log.Fatalf("failed to create milvus client: %v", err)
	}
	defer cli.Close(ctx)

	// Clean up existing collection
	_ = cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))

	// Create Indexer with Sparse Vector Support
	// This example uses "Bring Your Own Vectors" (BYOV) style where vectors are provided in the document.
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		Client:            cli,
		Collection:        collectionName,
		Dimension:         int64(dim),      // Set to 0 for sparse-only mode
		SparseVectorField: "sparse_vector", // Enable sparse vector field
		// SparseIndexBuilder is optional, defaults to SPARSE_INVERTED_INDEX
	})
	if err != nil {
		log.Fatalf("failed to create indexer: %v", err)
	}

	// Prepare documents with sparse vectors
	docs := make([]*schema.Document, 0, 10)
	for i := 0; i < 10; i++ {
		doc := &schema.Document{
			ID:      fmt.Sprintf("doc_%d", i),
			Content: fmt.Sprintf("Document %d with sparse features.", i),
		}

		// Generate a random dense vector (if using hybrid mode)
		if dim > 0 {
			vector := make([]float64, dim)
			for j := 0; j < dim; j++ {
				vector[j] = rand.Float64()
			}
			doc.WithDenseVector(vector)
		}

		// Generate a random sparse vector
		// Represented as a map[int]float64
		sparseVec := make(map[int]float64)
		// Add some random features
		for k := 0; k < 5; k++ {
			idx := rand.Intn(1000) // Sparse feature index
			val := rand.Float64()  // Feature weight
			sparseVec[idx] = val
		}
		doc.WithSparseVector(sparseVec)

		docs = append(docs, doc)
	}

	// Store the documents
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("failed to store documents: %v", err)
	}

	log.Printf("Successfully stored %d documents with sparse vectors!", len(ids))
	log.Printf("Stored IDs: %v", ids)
}
