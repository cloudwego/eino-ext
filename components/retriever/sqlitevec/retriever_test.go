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

package sqlitevec

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
)

func TestNewRetrieverValidation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	emb := &mockEmbedding{vector: []float64{0.1, 0.2, 0.3}}

	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: "config is nil",
		},
		{
			name: "nil db",
			config: &Config{
				Embedding: emb,
				VectorDim: 3,
			},
			wantErr: "db not provided",
		},
		{
			name: "nil embedding",
			config: &Config{
				DB:        db,
				VectorDim: 3,
			},
			wantErr: "embedding not provided",
		},
		{
			name: "invalid vector dim",
			config: &Config{
				DB:        db,
				Embedding: emb,
			},
			wantErr: "vector dim must be positive",
		},
		{
			name: "invalid document table",
			config: &Config{
				DB:            db,
				DocumentTable: "bad-name",
				Embedding:     emb,
				VectorDim:     3,
			},
			wantErr: "invalid document table",
		},
		{
			name: "invalid vector table",
			config: &Config{
				DB:          db,
				VectorTable: "bad-name",
				Embedding:   emb,
				VectorDim:   3,
			},
			wantErr: "invalid vector table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRetriever(ctx, tt.config)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if got != nil {
				t.Fatalf("expected nil retriever")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestRetrieverRetrieve(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedTestData(t, db)

	ret, err := NewRetriever(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		TopK:      2,
		Embedding: &mockEmbedding{vector: []float64{0.1, 0.2, 0.31}},
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}

	docs, err := ret.Retrieve(ctx, "near doc one")
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID != "doc-1" {
		t.Fatalf("expected doc-1 first, got %s", docs[0].ID)
	}
	if docs[0].Content != "first document" {
		t.Fatalf("unexpected content: %s", docs[0].Content)
	}
	if docs[0].MetaData["source"] != "test" {
		t.Fatalf("metadata not restored: %#v", docs[0].MetaData)
	}
	if _, ok := docs[0].MetaData[metadataDistanceKey].(float64); !ok {
		t.Fatalf("missing distance metadata: %#v", docs[0].MetaData)
	}
	if docs[0].Score() <= docs[1].Score() {
		t.Fatalf("expected first doc score to be greater, got %f <= %f", docs[0].Score(), docs[1].Score())
	}
}

func TestRetrieverRetrieveOptions(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedTestData(t, db)

	ret, err := NewRetriever(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		Embedding: &mockEmbedding{vector: []float64{0.1, 0.2, 0.31}},
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}

	docs, err := ret.Retrieve(ctx, "near doc one", retriever.WithTopK(1))
	if err != nil {
		t.Fatalf("Retrieve with topK failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}

	docs, err = ret.Retrieve(ctx, "near doc one", WithMaxDistance(0.001))
	if err != nil {
		t.Fatalf("Retrieve with max distance failed: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs after max distance filter, got %d", len(docs))
	}

	docs, err = ret.Retrieve(ctx, "near doc one", retriever.WithScoreThreshold(1.1))
	if err != nil {
		t.Fatalf("Retrieve with score threshold failed: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected no docs after score threshold filter, got %d", len(docs))
	}
}

func TestRetrieverRetrieveValidation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	seedTestData(t, db)

	tests := []struct {
		name    string
		emb     embedding.Embedder
		opts    []retriever.Option
		wantErr string
	}{
		{
			name:    "nil option embedding",
			emb:     &mockEmbedding{vector: []float64{0.1, 0.2, 0.3}},
			opts:    []retriever.Option{retriever.WithEmbedding(nil)},
			wantErr: "embedding not provided",
		},
		{
			name:    "invalid top k",
			emb:     &mockEmbedding{vector: []float64{0.1, 0.2, 0.3}},
			opts:    []retriever.Option{retriever.WithTopK(0)},
			wantErr: "top k must be positive",
		},
		{
			name: "embedding error",
			emb: &mockEmbedding{
				err: errors.New("embed failed"),
			},
			wantErr: "embed failed",
		},
		{
			name: "embedding length mismatch",
			emb: &mockEmbedding{
				vectors: [][]float64{
					{0.1, 0.2, 0.3},
					{0.4, 0.5, 0.6},
				},
			},
			wantErr: "invalid embedding result length",
		},
		{
			name: "embedding dimension mismatch",
			emb: &mockEmbedding{
				vector: []float64{0.1, 0.2},
			},
			wantErr: "invalid vector dimension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := NewRetriever(ctx, &Config{
				DB:        db,
				VectorDim: 3,
				Embedding: tt.emb,
			})
			if err != nil {
				t.Fatalf("NewRetriever failed: %v", err)
			}

			_, err = ret.Retrieve(ctx, "query", tt.opts...)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestRetrieverCorruptMetadata(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	createTestSchema(t, db)
	insertTestDocument(t, db, 1, "doc-1", "bad metadata", "{bad-json}", []float64{0.1, 0.2, 0.3})

	ret, err := NewRetriever(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		Embedding: &mockEmbedding{vector: []float64{0.1, 0.2, 0.3}},
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}

	_, err = ret.Retrieve(ctx, "query")
	if err == nil {
		t.Fatalf("expected corrupt metadata error")
	}
	if !strings.Contains(err.Error(), "unmarshal metadata failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRetrieverTypeAndCallbacks(t *testing.T) {
	ret, err := NewRetriever(context.Background(), &Config{
		DB:        openTestDB(t),
		VectorDim: 3,
		Embedding: &mockEmbedding{
			vector: []float64{0.1, 0.2, 0.3},
		},
	})
	if err != nil {
		t.Fatalf("NewRetriever failed: %v", err)
	}
	if got := ret.GetType(); got != "SQLiteVec" {
		t.Fatalf("unexpected type: %s", got)
	}
	if !ret.IsCallbacksEnabled() {
		t.Fatalf("callbacks should be enabled")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func seedTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	createTestSchema(t, db)
	insertTestDocument(t, db, 1, "doc-1", "first document", `{"source":"test"}`, []float64{0.1, 0.2, 0.3})
	insertTestDocument(t, db, 2, "doc-2", "second document", `{"source":"test"}`, []float64{0.9, 0.8, 0.7})
	insertTestDocument(t, db, 3, "doc-3", "third document", `{}`, []float64{0.2, 0.2, 0.2})
}

func createTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE eino_sqlitevec_documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_id TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		metadata_json TEXT NOT NULL DEFAULT '{}'
	)`)
	if err != nil {
		t.Fatalf("create document table failed: %v", err)
	}
	_, err = db.Exec(`CREATE VIRTUAL TABLE eino_sqlitevec_vectors USING vec0(
		embedding float[3]
	)`)
	if err != nil {
		t.Fatalf("create vector table failed: %v", err)
	}
}

func insertTestDocument(t *testing.T, db *sql.DB, rowID int64, docID, content, metadataJSON string, vector []float64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO eino_sqlitevec_documents (id, doc_id, content, metadata_json) VALUES (?, ?, ?, ?)`,
		rowID, docID, content, metadataJSON)
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		t.Fatalf("marshal vector failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO eino_sqlitevec_vectors (rowid, embedding) VALUES (?, ?)`, rowID, string(vectorJSON))
	if err != nil {
		t.Fatalf("insert vector failed: %v", err)
	}
}

type mockEmbedding struct {
	vector  []float64
	vectors [][]float64
	err     error
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.vectors != nil {
		return m.vectors, nil
	}
	if m.vector == nil {
		return nil, fmt.Errorf("vector not set")
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = append([]float64(nil), m.vector...)
	}
	return result, nil
}
