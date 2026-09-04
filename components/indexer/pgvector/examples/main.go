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

	"github.com/cloudwego/eino-ext/components/indexer/pgvector"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This example demonstrates how to use the pgvector indexer.
// Prerequisites:
// 1. PostgreSQL installed with pgvector extension
// 2. Database created: CREATE DATABASE eino_example;
// 3. Table created:
//    CREATE EXTENSION IF NOT EXISTS vector;
//    CREATE TABLE documents (
//        id TEXT PRIMARY KEY,
//        content TEXT NOT NULL,
//        embedding vector(1536),
//        metadata JSONB
//    );
// 4. Connection string matches your database setup

func main() {
	ctx := context.Background()

	// Connect to PostgreSQL
	// Update the connection string to match your database configuration
	connString := "postgres://test_user:test_password@localhost:5433/eino_test?sslmode=disable"
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create indexer config
	config := &pgvector.IndexerConfig{
		Conn:      pool,
		TableName: "documents",
		Embedding: &mockEmbedder{}, // In production, use real embedder
		BatchSize: 10,
	}

	// Create indexer
	idxr, err := pgvector.NewIndexer(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	// Sample documents to index
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "PostgreSQL is a powerful open-source relational database.",
			MetaData: map[string]any{
				"category": "database",
				"tags":     []string{"postgresql", "sql"},
			},
		},
		{
			ID:      "doc2",
			Content: "pgvector is an extension for vector similarity search.",
			MetaData: map[string]any{
				"category": "database",
				"tags":     []string{"pgvector", "extension"},
			},
		},
		{
			ID:      "doc3",
			Content: "Machine learning models can be embedded as vectors for similarity search.",
			MetaData: map[string]any{
				"category": "ml",
				"tags":     []string{"ml", "embedding", "search"},
			},
		},
	}

	// Store documents
	ids, err := idxr.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store documents: %v", err)
	}

	fmt.Printf("Successfully indexed %d documents\n", len(ids))
	for _, id := range ids {
		fmt.Printf("  - %s\n", id)
	}
}

// mockEmbedder is a mock embedding implementation for demonstration.
// In production, replace with real embedder like:
//
//	import "github.com/cloudwego/eino/components/embedding/openai"
//	embedding := openai.NewEmbedder()
type mockEmbedder struct{}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	// Return mock 3-dimensional vectors for demonstration
	// In production, your embedder should return vectors matching your model's dimensions
	result := make([][]float64, len(texts))
	for i := range result {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}
