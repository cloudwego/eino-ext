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
	"os"

	opensearch "github.com/opensearch-project/opensearch-go/v2"

	"github.com/cloudwego/eino-ext/components/retriever/opensearch2"
	"github.com/cloudwego/eino-ext/components/retriever/opensearch2/search_mode"
)

const (
	indexName = "eino_opensearch_test_sparse"
)

func main() {
	ctx := context.Background()

	// Default credentials, can be overridden by env vars
	addr := os.Getenv("OPENSEARCH_ADDR")
	if addr == "" {
		addr = "http://localhost:9200"
	}
	username := os.Getenv("OPENSEARCH_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("OPENSEARCH_PASSWORD")
	if password == "" {
		password = "admin"
	}

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{addr},
		Username:  username,
		Password:  password,
	})
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	// Note: Neural Sparse search usually requires a model to be deployed in OpenSearch
	// and an ingest pipeline to be configured to encode text into sparse vectors during indexing.
	// This example assumes such index "eino_opensearch_test_sparse" and field "sparse_vector_field" exists.

	ret, err := opensearch2.NewRetriever(ctx, &opensearch2.RetrieverConfig{
		Client: client,
		Index:  indexName,
		// Demonstrate NeuralSparse search mode
		SearchMode: search_mode.NeuralSparse("sparse_vector_field", &search_mode.NeuralSparseConfig{
			// ModelID: "your-model-id", // Optional: if different from default
		}),
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	query := "semantic query"

	fmt.Printf("\n--- Retrieving for query: '%s' using NeuralSparse ---\n", query)

	retDocs, err := ret.Retrieve(ctx, query)
	if err != nil {
		log.Printf("Failed to retrieve (this is expected if index/model doesn't exist): %v", err)
		return
	}

	for _, d := range retDocs {
		fmt.Printf("Doc: %s, Score: %.4f\nContent: %s\n", d.ID, d.Score(), d.Content)
	}
}
