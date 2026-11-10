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

	"github.com/cloudwego/eino-ext/components/retriever/pgvector"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This example demonstrates how to use the pgvector retriever.
// Prerequisites:
// 1. PostgreSQL installed with pgvector extension
// 2. Database created: CREATE DATABASE eino_test;
// 3. Table created with documents (run indexer example first)
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

	// Create retriever config
	config := &pgvector.RetrieverConfig{
		Conn:             pool,
		TableName:        "documents",
		Embedding:        &mockEmbedder{}, // In production, use real embedder
		DistanceFunction: pgvector.DistanceCosine,
		TopK:             5,
	}

	// Create retriever
	retr, err := pgvector.NewRetriever(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// Search query
	query := "PostgreSQL database features"

	// Retrieve similar documents
	docs, err := retr.Retrieve(ctx, query)
	if err != nil {
		log.Fatalf("Failed to retrieve documents: %v", err)
	}

	fmt.Printf("Found %d similar documents for query: %s\n\n", len(docs), query)

	for i, doc := range docs {
		fmt.Printf("Rank %d:\n", i+1)
		fmt.Printf("  ID: %s\n", doc.ID)
		fmt.Printf("  Content: %s\n", doc.Content)
		fmt.Printf("  Score: %.4f\n", doc.Score())
		if len(doc.MetaData) > 0 {
			fmt.Printf("  Metadata: %v\n", doc.MetaData)
		}
		fmt.Println()
	}

	// Example: Retrieve with filtering by metadata
	fmt.Println("Example: Filtering by metadata category='database'")
	filteredDocs, err := retr.Retrieve(ctx, query,
		pgvector.WithWhereClause("metadata->>'category' = 'database'"),
	)
	if err != nil {
		log.Fatalf("Failed to retrieve filtered documents: %v", err)
	}

	fmt.Printf("Found %d documents in 'database' category\n\n", len(filteredDocs))
	for i, doc := range filteredDocs {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, doc.Content, doc.Score())
	}
	fmt.Println()

	// Example: Retrieve with score threshold
	fmt.Println("Example: Using score threshold of 0.5")
	thresholdDocs, err := retr.Retrieve(ctx, query,
		retriever.WithScoreThreshold(0.5),
	)
	if err != nil {
		log.Fatalf("Failed to retrieve documents with threshold: %v", err)
	}

	fmt.Printf("Found %d documents with score >= 0.50\n", len(thresholdDocs))
	for i, doc := range thresholdDocs {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, doc.Content, doc.Score())
	}

	// Example: Using different distance function
	fmt.Println("\nExample: Using L2 distance function")
	l2Docs, err := retr.Retrieve(ctx, query,
		pgvector.WithDistanceFunction(pgvector.DistanceL2),
	)
	if err != nil {
		log.Fatalf("Failed to retrieve documents with L2 distance: %v", err)
	}

	fmt.Printf("Found %d documents using L2 distance\n", len(l2Docs))
	for i, doc := range l2Docs {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, doc.Content, doc.Score())
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
