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
	collectionName := "eino_byov_test"
	dim := 128

	// Create a new Milvus client
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: addr})
	if err != nil {
		log.Fatalf("failed to create milvus client: %v", err)
	}
	defer cli.Close(ctx)

	// Clean up existing collection
	_ = cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))

	// Create Indexer WITHOUT an Embedder
	// This configuration allows "Bring Your Own Vectors" (BYOV)
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		Client:     cli,
		Collection: collectionName,
		Vector: &milvus2.VectorConfig{
			VectorField: "vector",
			Dimension:   int64(dim),
		},
		// Embedding field is omitted/nil
	})
	if err != nil {
		log.Fatalf("failed to create indexer: %v", err)
	}

	// Prepare documents with pre-computed vectors
	docs := make([]*schema.Document, 0, 10)
	for i := 0; i < 10; i++ {
		// Generate variable content
		doc := &schema.Document{
			ID:      fmt.Sprintf("doc_%d", i),
			Content: fmt.Sprintf("This is document number %d with a pre-computed vector.", i),
			MetaData: map[string]any{
				"source": "manual_vector_generation",
			},
		}

		// Generate a random vector
		// In a real scenario, this would come from an external model or API
		vector := make([]float64, dim)
		for j := 0; j < dim; j++ {
			vector[j] = rand.Float64()
		}

		// Attach the vector to the document using WithDenseVector
		doc.WithDenseVector(vector)

		docs = append(docs, doc)
	}

	// Store the documents
	// The indexer will detect the nil Embedder and look for vectors in the document metadata
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("failed to store documents: %v", err)
	}

	log.Printf("Successfully stored %d documents with custom vectors!", len(ids))
	log.Printf("Stored IDs: %v", ids)
}
