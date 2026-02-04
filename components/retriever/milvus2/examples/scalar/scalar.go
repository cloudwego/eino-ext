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

// This example demonstrates Scalar Search (metadata-only filtering).
//
// Prerequisites:
// Run the demo indexer first to create the collection:
//
//	cd ../../../indexer/milvus2/examples/demo && go run .
//
// The collection "demo_milvus2" has documents with tag and year metadata.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
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

	// Create retriever with Scalar search mode
	// Scalar uses the Query API (metadata filtering) instead of vector search.
	// The query string is treated as a boolean filter expression.
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{Address: addr},
		Collection:   collectionName,
		TopK:         10,
		VectorField:  "vector",
		OutputFields: []string{"content", "metadata"},
		SearchMode:   search_mode.NewScalar(),
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}
	log.Println("Retriever created successfully")

	// Case A: Query by ID
	fmt.Println("\n--- Query: id == 'doc_1' ---")
	res, err := retriever.Retrieve(ctx, "id == 'doc_1'")
	printRes(res, err)

	// Case B: Query by JSON Metadata
	fmt.Println("\n--- Query: metadata['tag'] == 'coding' ---")
	res, err = retriever.Retrieve(ctx, "metadata['tag'] == 'coding'")
	printRes(res, err)

	// Case C: Complex Query
	fmt.Println("\n--- Query: metadata['year'] == 2024 and metadata['tag'] == 'life' ---")
	res, err = retriever.Retrieve(ctx, "metadata['year'] == 2024 && metadata['tag'] == 'life'")
	printRes(res, err)
}

func printRes(docs []*schema.Document, err error) {
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	for _, d := range docs {
		fmt.Printf("ID: %s, Content: %s\n", d.ID, d.Content)
	}
}
