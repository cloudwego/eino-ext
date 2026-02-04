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

// This example creates the "demo_milvus2" collection used by multiple retriever examples.
// Run this first, then run any of the retriever examples:
//   - retriever/milvus2/examples/grouping
//   - retriever/milvus2/examples/iterator
//   - retriever/milvus2/examples/scalar
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
)

func main() {
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}

	ctx := context.Background()
	collectionName := "demo_milvus2"
	vectorField := "vector"
	dim := 128
	itemCount := 60 // Enough for iterator (50+) and grouping tests

	// Create client
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: addr})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close(ctx)

	// Drop existing collection
	_ = cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))

	// Create schema with category field for grouping
	s := entity.NewSchema().
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName(vectorField).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dim))).
		WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(65535)).
		WithField(entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON)).
		WithField(entity.NewField().WithName("category").WithDataType(entity.FieldTypeVarChar).WithMaxLength(255))

	err = cli.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(collectionName, s))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created collection: %s", collectionName)

	// Create HNSW index with COSINE metric
	idx := milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200).Build(milvus2.COSINE)
	t, err := cli.CreateIndex(ctx, milvusclient.NewCreateIndexOption(collectionName, vectorField, idx))
	if err != nil {
		log.Fatal(err)
	}
	if err := t.Await(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Created HNSW index with COSINE metric")

	// Load collection
	loadTask, err := cli.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collectionName))
	if err != nil {
		log.Fatal(err)
	}
	if err := loadTask.Await(ctx); err != nil {
		log.Fatal(err)
	}

	// Create indexer with custom DocumentConverter that extracts category for grouping
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		Client:     cli,
		Collection: collectionName,
		Vector: &milvus2.VectorConfig{
			VectorField:  vectorField,
			Dimension:    int64(dim),
			MetricType:   milvus2.COSINE,
			IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		},
		Embedding: &mockEmbedding{dim: dim},
		DocumentConverter: func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error) {
			ids := make([]string, 0, len(docs))
			contents := make([]string, 0, len(docs))
			vecs := make([][]float32, 0, len(docs))
			metadatas := make([][]byte, 0, len(docs))
			categories := make([]string, 0, len(docs))

			for idx, doc := range docs {
				ids = append(ids, doc.ID)
				contents = append(contents, doc.Content)

				// Convert float64 vector to float32
				vec := make([]float32, len(vectors[idx]))
				for i, v := range vectors[idx] {
					vec[i] = float32(v)
				}
				vecs = append(vecs, vec)

				// Marshal metadata to JSON
				metadata, err := json.Marshal(doc.MetaData)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal metadata: %w", err)
				}
				metadatas = append(metadatas, metadata)

				// Extract category for grouping search
				cat := ""
				if c, ok := doc.MetaData["category"].(string); ok {
					cat = c
				}
				categories = append(categories, cat)
			}

			vecDim := 0
			if len(vecs) > 0 {
				vecDim = len(vecs[0])
			}

			return []column.Column{
				column.NewColumnVarChar("id", ids),
				column.NewColumnVarChar("content", contents),
				column.NewColumnFloatVector("vector", vecDim, vecs),
				column.NewColumnJSONBytes("metadata", metadatas),
				column.NewColumnVarChar("category", categories),
			}, nil
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Insert documents with categories and metadata for all example scenarios
	categories := []string{"electronics", "books", "clothing"}
	tags := []string{"coding", "life", "travel"}

	var docs []*schema.Document
	for i := 0; i < itemCount; i++ {
		cat := categories[i%len(categories)]
		tag := tags[i%len(tags)]
		year := 2023 + (i % 2)

		docs = append(docs, &schema.Document{
			ID:      fmt.Sprintf("doc_%d", i),
			Content: fmt.Sprintf("Content %d in category %s about %s", i, cat, tag),
			MetaData: map[string]any{
				"category": cat,
				"tag":      tag,
				"year":     year,
			},
		})
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Stored %d documents", len(ids))
	log.Printf("  Categories: %v", categories)
	log.Printf("  Tags: %v", tags)
	log.Printf("  Years: 2023, 2024")
	log.Println("Collection ready for retriever examples (grouping, iterator, scalar)")
}

type mockEmbedding struct{ dim int }

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.dim)
		for j := range vec {
			// Distinct vectors based on index for iterator test
			vec[j] = float64(i) * 0.01
		}
		result[i] = vec
	}
	return result, nil
}
