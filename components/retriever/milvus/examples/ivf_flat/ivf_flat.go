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

// Package main demonstrates how to use IVF_FLAT search mode with Milvus retriever.
// IVF_FLAT divides vectors into clusters and searches only relevant clusters,
// providing a good balance between speed and accuracy for large datasets.
// This example uses a FloatVector collection created by the ivf_flat_index example.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino-ext/components/retriever/milvus/search_mode"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")

	// Create a client
	ctx := context.Background()
	cli, err := client.NewClient(ctx, client.Config{
		Address:  addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer cli.Close()

	// Create IVF_FLAT search mode with COSINE similarity
	// nprobe=16 searches 16 clusters, good balance for most datasets
	ivfMode, err := search_mode.SearchModeIvfFlat(&search_mode.IvfFlatConfig{
		NProbe: 16,            // Number of clusters to search
		Metric: entity.COSINE, // Cosine similarity (must match index metric)
	})
	if err != nil {
		log.Fatalf("Failed to create IVF_FLAT search mode: %v", err)
		return
	}

	// Create a retriever with IVF_FLAT search mode
	// Note: This requires the "ivf_flat_collection" to exist (run ivf_flat_index example first)
	retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:     cli,
		Collection: "ivf_flat_collection",
		OutputFields: []string{
			"id",
			"content",
		},
		TopK:            10,
		SearchMode:      ivfMode,
		VectorConverter: floatVectorConverter,
		Embedding:       &mockEmbedding{},
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "similarity search")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}

	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("  ID: %s\n", doc.ID)
		fmt.Printf("  Content: %s\n", doc.Content)
	}
}

// floatVectorConverter converts float64 vectors to FloatVector
func floatVectorConverter(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
	vec := make([]entity.Vector, 0, len(vectors))
	for _, vector := range vectors {
		vec32 := make([]float32, len(vector))
		for i, v := range vector {
			vec32[i] = float32(v)
		}
		vec = append(vec, entity.FloatVector(vec32))
	}
	return vec, nil
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
