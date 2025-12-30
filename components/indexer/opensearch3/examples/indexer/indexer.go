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
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/cloudwego/eino-ext/components/indexer/opensearch3"
)

const (
	indexName = "eino_opensearch_test"
	embDim    = 1024
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

	if err := ensureIndex(ctx, client, indexName); err != nil {
		log.Fatalf("Failed to ensure index: %v", err)
	}

	mockEmb := &MockEmbedder{}

	idx, err := opensearch3.NewIndexer(ctx, &opensearch3.IndexerConfig{
		Client: client,
		Index:  indexName,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]opensearch3.FieldValue, err error) {
			return map[string]opensearch3.FieldValue{
				"content": {
					Value:    doc.Content,
					EmbedKey: "vector",
				},
				"author": {Value: doc.MetaData["author"]},
			}, nil
		},
		Embedding: mockEmb,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	documents := []string{
		"CloudWeGo is a high-performance microservice framework.",
		"Eino is a framework for building LLM applications.",
		"OpenSearch is a community-driven, open source search and analytics suite.",
	}

	var docs []*schema.Document
	for i, content := range documents {
		docs = append(docs, &schema.Document{
			ID:      fmt.Sprintf("doc_%d", i+1),
			Content: content,
			MetaData: map[string]any{
				"author": "CloudWeGo",
			},
		})
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store documents: %v", err)
	}
	fmt.Printf("Stored documents: %v\n", ids)
}

func ensureIndex(ctx context.Context, client *opensearchapi.Client, indexName string) error {
	// Check if index exists
	existsResp, err := client.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{indexName},
	})
	if err != nil {
		// If error is 404, it means index doesn't exist, so we can proceed to create it.
		// opensearch-go v4 might return error for 404 status in Exists?
		if !strings.Contains(err.Error(), "404") {
			return err
		}
	} else if existsResp.StatusCode == 200 {
		return nil
	} else if existsResp.StatusCode != 404 {
		return fmt.Errorf("error checking index existence: %d", existsResp.StatusCode)
	}

	mapping := fmt.Sprintf(`{
		"settings": {
			"index": {
				"knn": true
			}
		},
		"mappings": {
			"properties": {
				"content": { "type": "text" },
				"vector": {
					"type": "knn_vector",
					"dimension": %d,
					"method": {
						"name": "hnsw",
						"engine": "faiss",
						"space_type": "innerproduct"
					}
				},
				"author": { "type": "keyword" }
			}
		}
	}`, embDim)

	_, err = client.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	})
	if err != nil {
		return err
	}
	// Create returns IndicesCreateResp
	// But check error?
	// Assuming Create returns success struct or error.

	fmt.Printf("Created index: %s\n", indexName)
	return nil
}

// MockEmbedder produces deterministic vectors based on word content.
// Texts sharing common words produce similar vectors, enabling approximate search demo.
type MockEmbedder struct{}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	var res [][]float64
	for _, text := range texts {
		res = append(res, textToVector(text, embDim))
	}
	return res, nil
}

// textToVector creates a deterministic vector from text using word-based hashing.
// Words are hashed to specific dimensions, so texts with common words have similar vectors.
func textToVector(text string, dim int) []float64 {
	vec := make([]float64, dim)
	// Split into words (simple tokenization)
	words := strings.Fields(strings.ToLower(text))
	for _, word := range words {
		// Hash each word to a dimension index
		h := hashWord(word)
		idx := int(h % uint64(dim))
		vec[idx] += 1.0
	}
	// L2 normalize for cosine similarity
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

// hashWord produces a simple hash for a word
func hashWord(word string) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for _, c := range word {
		h ^= uint64(c)
		h *= 1099511628211 // FNV prime
	}
	return h
}
