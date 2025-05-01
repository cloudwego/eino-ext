/*
 * Copyright 2024 CloudWeGo Authors
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
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/indexer/pgvector"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()
	timeout := 30 * time.Second
	// create ark embedder
	embedder, err := ark.NewEmbedder(context.Background(), &ark.EmbeddingConfig{
		APIKey:  "your_api_key",
		Model:   "your_model (e.g. doubao-embedding-text-240715)",
		Timeout: &timeout,
	})
	if err != nil {
		log.Fatalf("Failed to create embedder: %v", err)
	}
	// create pgvector indexer
	indexer, err := pgvector.NewIndexer(ctx, &pgvector.IndexerConfig{
		Host:      "localhost",
		Port:      5432,
		User:      "postgres",
		Password:  "postgres",
		DBName:    "vectorDB",
		SSLMode:   "disable",
		TableName: "documents",
		Dimension: 2560,
		Embedding: embedder,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}
	defer indexer.Close()

	// create documents
	docs := []*schema.Document{
		{
			ID:      uuid.New().String(),
			Content: "PostgreSQL是一个功能强大的开源对象关系数据库系统",
			MetaData: map[string]interface{}{
				"source": "database_intro",
				"author": "postgres_team",
			},
		},
		{
			ID:      uuid.New().String(),
			Content: "pgvector是PostgreSQL的一个扩展，它为PostgreSQL添加了向量相似度搜索功能",
			MetaData: map[string]interface{}{
				"source": "pgvector_intro",
				"author": "pgvector_team",
			},
		},
	}

	// add extra pgvector fields
	pgvector.SetExtraPGVectorFields(docs[1], map[string]interface{}{
		"priority": "high",
		"category": "extension",
	})

	// store documents
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store documents: %v", err)
	}

	fmt.Printf("Successfully stored %d documents with IDs: %v\n", len(ids), ids)
}
