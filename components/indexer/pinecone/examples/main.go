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
	"github.com/bytedance/sonic"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/indexer/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func main() {
	// Load configuration from environment variables
	apiKey := os.Getenv("PINECONE_APIKEY")
	if apiKey == "" {
		log.Fatal("PINECONE_APIKEY environment variable is required")
	}

	// Initialize Pinecone client
	client, err := pc.NewClient(pc.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create Pinecone client: %v", err)
	}

	// Create Pinecone indexer config
	config := pinecone.IndexerConfig{
		Client:    client,
		Dimension: 2560,
		Embedding: &mockEmbedding{},
	}

	// Create an indexer
	ctx := context.Background()
	indexer, err := pinecone.NewIndexer(ctx, &config)
	if err != nil {
		log.Fatalf("Failed to create Pinecone indexer: %v", err)
	}
	log.Println("Indexer created successfully")

	// Store documents
	docs := []*schema.Document{
		{
			ID:      "pinecone-1",
			Content: "pinecone is a vector database",
			MetaData: map[string]any{
				"tag1": "pinecone",
				"tag2": "vector",
				"tag3": "database",
			},
		},
		{
			ID:      "pinecone-2",
			Content: "Pinecone is an vector database for building accurate and performant AI applications.",
		},
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store documents: %v", err)
		return
	}
	log.Printf("Stored documents successfully, ids: %v", ids)
}

type vector struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	bytes, err := os.ReadFile("./examples/embeddings.json")
	if err != nil {
		return nil, err
	}
	var v vector
	if err := sonic.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}
	res := make([][]float64, 0, len(v.Data))
	for _, data := range v.Data {
		res = append(res, data.Embedding)
	}
	return res, nil
}
