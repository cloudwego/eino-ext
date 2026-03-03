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

	"github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// This example demonstrates Sparse-only retrieval using BM25.
// It uses the collection created by the indexer sparse example.

func main() {
	ctx := context.Background()
	collectionName := "eino_sparse_test"
	milvusAddr := "localhost:19530"

	// Create retriever with Sparse search mode (BM25)
	// Sparse mode sends raw text to Milvus for server-side BM25 processing.
	// No Embedding is needed as the sparse vector is generated server-side.
	r, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address: milvusAddr,
		},
		Collection:        collectionName,
		SparseVectorField: "sparse_vector",
		SearchMode:        search_mode.NewSparse(milvus2.BM25),
		TopK:              3,
		OutputFields:      []string{"*"},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// Retrieve
	query := "information retrieval"
	docs, err := r.Retrieve(ctx, query)
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	fmt.Printf("=== Query: %s ===\n", query)
	fmt.Printf("Found %d documents:\n", len(docs))
	for _, doc := range docs {
		fmt.Printf("ID: %s, Score: %.4f, Content: %s\n", doc.ID, doc.Score(), doc.Content)
	}
}
