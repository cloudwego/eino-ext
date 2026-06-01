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
	"math"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/embedding"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/cloudwego/eino-ext/components/retriever/opensearch3"
	"github.com/cloudwego/eino-ext/components/retriever/opensearch3/search_mode"
)

const (
	indexName = "eino_opensearch_test"
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

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{addr},
			Username:  username,
			Password:  password,
		},
	})
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	mockEmb := &MockEmbedder{}

	ret, err := opensearch3.NewRetriever(ctx, &opensearch3.RetrieverConfig{
		Client: client,
		Index:  indexName,
		// Demonstrate Approximate search mode (upgraded from KNN)
		SearchMode: search_mode.Approximate(&search_mode.ApproximateConfig{
			VectorField: "vector",
			K:           5,
			// Enable Hybrid search if needed
			// Hybrid: true,
			// QueryFieldName: "content",
		}),
		Embedding: mockEmb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	query := "high-performance microservice framework"
	// The mock embedder returns a fixed vector, so it will match.
	// We just want to verify the pipeline works.

	fmt.Printf("\n--- Retrieving for query: '%s' ---\n", query)

	retDocs, err := ret.Retrieve(ctx, query)
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	for _, d := range retDocs {
		fmt.Printf("Doc: %s, Score: %.4f\nContent: %s\nAuthor: %v\n", d.ID, d.Score(), d.Content, d.MetaData["author"])
	}
}

const embDim = 1024

// MockEmbedder produces deterministic vectors based on word content.
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	var res [][]float64
	for _, text := range texts {
		res = append(res, textToVector(text, embDim))
	}
	return res, nil
}

func textToVector(text string, dim int) []float64 {
	vec := make([]float64, dim)
	words := strings.Fields(strings.ToLower(text))
	for _, word := range words {
		h := hashWord(word)
		idx := int(h % uint64(dim))
		vec[idx] += 1.0
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = 1.0 / math.Sqrt(norm)
		for i := range vec {
			vec[i] *= norm
		}
	}
	return vec
}

func hashWord(word string) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range word {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
