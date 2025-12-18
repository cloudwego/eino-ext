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

// Package main demonstrates how to use HNSW index builder with Milvus indexer.
// HNSW provides excellent search performance with high recall.
// This example uses FloatVector with L2 distance metric.
package main

import (
	"context"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
)

const vectorDim = 2560

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

	// Create HNSW index builder
	// M=16: number of bi-directional links (higher = better quality, more memory)
	// efConstruction=200: build-time search width (higher = better quality, slower build)
	hnswBuilder, err := milvus.NewHNSWIndexBuilder(16, 200)
	if err != nil {
		log.Fatalf("Failed to create HNSW index builder: %v", err)
		return
	}

	// Create an indexer with HNSW index and FloatVector schema
	indexer, err := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:       cli,
		Collection:   "hnsw_collection",
		MetricType:   milvus.L2, // Euclidean distance for float vectors
		IndexBuilder: hnswBuilder,
		// Custom schema with FloatVector (required for HNSW with L2)
		Fields:            getFloatVectorFields(),
		DocumentConverter: floatVectorDocumentConverter,
		Embedding:         &mockEmbedding{},
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created with HNSW index")

	// Store documents
	docs := []*schema.Document{
		{
			ID:      "hnsw-doc-1",
			Content: "HNSW provides excellent search performance",
			MetaData: map[string]any{
				"source": "example",
			},
		},
		{
			ID:      "hnsw-doc-2",
			Content: "Graph-based index with high recall",
		},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
		return
	}
	log.Printf("Stored documents with HNSW index, ids: %v", ids)
}

// getFloatVectorFields returns schema fields for FloatVector storage
func getFloatVectorFields() []*entity.Field {
	return []*entity.Field{
		{
			Name:       "id",
			DataType:   entity.FieldTypeVarChar,
			PrimaryKey: true,
			AutoID:     false,
			TypeParams: map[string]string{"max_length": "256"},
		},
		{
			Name:     "vector",
			DataType: entity.FieldTypeFloatVector,
			TypeParams: map[string]string{
				"dim": "2560",
			},
		},
		{
			Name:       "content",
			DataType:   entity.FieldTypeVarChar,
			TypeParams: map[string]string{"max_length": "65535"},
		},
		{
			Name:     "metadata",
			DataType: entity.FieldTypeJSON,
		},
	}
}

// floatVectorDocumentConverter converts documents to rows with FloatVector
func floatVectorDocumentConverter(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	rows := make([]interface{}, 0, len(docs))
	for idx, doc := range docs {
		// Convert float64 to float32 for FloatVector
		vec32 := make([]float32, len(vectors[idx]))
		for i, v := range vectors[idx] {
			vec32[i] = float32(v)
		}

		metadata, _ := sonic.Marshal(doc.MetaData)
		rows = append(rows, map[string]interface{}{
			"id":       doc.ID,
			"vector":   vec32,
			"content":  doc.Content,
			"metadata": metadata,
		})
	}
	return rows, nil
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
