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
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/retriever/sqlitevec"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
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

	if err := seed(ctx, db); err != nil {
		log.Fatal(err)
	}

	ret, err := sqlitevec.NewRetriever(ctx, &sqlitevec.Config{
		DB:        db,
		VectorDim: 3,
		Embedding: demoEmbedding{},
	})
	if err != nil {
		log.Fatal(err)
	}

	docs, err := ret.Retrieve(ctx, "Eino", retriever.WithTopK(1))
	if err != nil {
		log.Fatal(err)
	}
	for _, doc := range docs {
		fmt.Printf("%s %.4f %s\n", doc.ID, doc.Score(), doc.Content)
	}
}

func seed(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE eino_sqlitevec_documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_id TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		metadata_json TEXT NOT NULL DEFAULT '{}'
	)`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE VIRTUAL TABLE eino_sqlitevec_vectors USING vec0(
		embedding float[3]
	)`); err != nil {
		return err
	}
	return insert(ctx, db, 1, "doc-1", "Eino supports RAG applications.", []float64{0.1, 0.2, 0.3})
}

func insert(ctx context.Context, db *sql.DB, rowID int64, docID, content string, vector []float64) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO eino_sqlitevec_documents (id, doc_id, content, metadata_json)
VALUES (?, ?, ?, '{}')`, rowID, docID, content); err != nil {
		return err
	}
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO eino_sqlitevec_vectors (rowid, embedding) VALUES (?, ?)`, rowID, string(vectorJSON))
	return err
}

type demoEmbedding struct{}

func (demoEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{0.1, 0.2, 0.31}
	}
	return vectors, nil
}
