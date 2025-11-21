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

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	"github.com/cloudwego/eino-ext/components/retriever/milvus_new"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}

	// Create a client
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: addr,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer client.Close(ctx)

	// Create a retriever
	retriever, err := milvus_new.NewRetriever(ctx, &milvus_new.RetrieverConfig{
		Client:      client,
		Collection:  "eino_collection",
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		MetricType:     milvus_new.COSINE,
		TopK:           5,
		ScoreThreshold: 0.5,
		Embedding:      &mockEmbedding{},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}

	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("ID: %s\n", doc.ID)
		fmt.Printf("Content: %s\n", doc.Content)
		fmt.Printf("Metadata: %v\n", doc.MetaData)
		fmt.Println("---")
	}
}

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	// Return mock embeddings with 768 dimensions
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, 768)
		for j := range result[i] {
			result[i][j] = 0.1
		}
	}
	return result, nil
}
