/*
 * Copyright 2026 CloudWeGo Authors
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
	"database/sql"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/indexer/sqlitevec"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Close()
	}()

	idx, err := sqlitevec.NewIndexer(ctx, &sqlitevec.Config{
		DB:        db,
		VectorDim: 3,
		Embedding: demoEmbedding{},
	})
	if err != nil {
		log.Fatal(err)
	}

	ids, err := idx.Store(ctx, []*schema.Document{
		{ID: "doc-1", Content: "Eino supports RAG applications."},
		{ID: "doc-2", Content: "SQLite can be embedded in Go applications."},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ids)
}

type demoEmbedding struct{}

func (demoEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{float64(i) + 0.1, 0.2, 0.3}
	}
	return vectors, nil
}
