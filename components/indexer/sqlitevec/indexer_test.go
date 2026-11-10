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
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

func TestNewIndexerValidation(t *testing.T) {
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
			got, err := NewIndexer(ctx, tt.config)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if got != nil {
				t.Fatalf("expected nil indexer")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestIndexerStore(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	idx, err := NewIndexer(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		Embedding: &mockEmbedding{
			vector: []float64{0.1, 0.2, 0.3},
		},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	ids, err := idx.Store(ctx, []*schema.Document{
		{
			ID:      "doc-1",
			Content: "first document",
			MetaData: map[string]any{
				"source": "test",
				"rank":   float64(1),
			},
		},
		{
			ID:      "doc-2",
			Content: "second document",
		},
	})
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if got, want := len(ids), 2; got != want {
		t.Fatalf("expected %d ids, got %d", want, got)
	}
	if ids[0] != "doc-1" || ids[1] != "doc-2" {
		t.Fatalf("unexpected ids: %#v", ids)
	}

	var content, metadataJSON string
	if err := db.QueryRow(`SELECT content, metadata_json FROM eino_sqlitevec_documents WHERE doc_id = ?`, "doc-1").
		Scan(&content, &metadataJSON); err != nil {
		t.Fatalf("select document failed: %v", err)
	}
	if content != "first document" {
		t.Fatalf("unexpected content: %q", content)
	}
	if !strings.Contains(metadataJSON, `"source":"test"`) {
		t.Fatalf("metadata not stored correctly: %s", metadataJSON)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM eino_sqlitevec_vectors`).Scan(&count); err != nil {
		t.Fatalf("count vectors failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 vectors, got %d", count)
	}
}

func TestIndexerStoreValidation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	idx, err := NewIndexer(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		Embedding: &mockEmbedding{
			vector: []float64{0.1, 0.2, 0.3},
		},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	ids, err := idx.Store(ctx, nil)
	if err != nil {
		t.Fatalf("empty store failed: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty ids, got %#v", ids)
	}

	tests := []struct {
		name    string
		docs    []*schema.Document
		opts    []indexer.Option
		wantErr string
	}{
		{
			name:    "nil document",
			docs:    []*schema.Document{nil},
			wantErr: "document is nil",
		},
		{
			name: "empty id",
			docs: []*schema.Document{{
				Content: "empty id",
			}},
			wantErr: "document id is empty",
		},
		{
			name: "nil call option embedding",
			docs: []*schema.Document{{
				ID:      "doc-1",
				Content: "content",
			}},
			opts:    []indexer.Option{indexer.WithEmbedding(nil)},
			wantErr: "embedding not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := idx.Store(ctx, tt.docs, tt.opts...)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestIndexerEmbeddingErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		emb     embedding.Embedder
		wantErr string
	}{
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
				vectors: [][]float64{{0.1, 0.2, 0.3}},
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
			idx, err := NewIndexer(ctx, &Config{
				DB:        openTestDB(t),
				VectorDim: 3,
				Embedding: tt.emb,
			})
			if err != nil {
				t.Fatalf("NewIndexer failed: %v", err)
			}

			_, err = idx.Store(ctx, []*schema.Document{
				{ID: "doc-1", Content: "one"},
				{ID: "doc-2", Content: "two"},
			})
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestIndexerStoreUpsert(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	idx, err := NewIndexer(ctx, &Config{
		DB:        db,
		VectorDim: 3,
		Embedding: &mockEmbedding{
			vectors: [][]float64{
				{0.1, 0.2, 0.3},
				{0.9, 0.8, 0.7},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}

	if _, err := idx.Store(ctx, []*schema.Document{{ID: "doc-1", Content: "old"}}); err != nil {
		t.Fatalf("first store failed: %v", err)
	}
	if _, err := idx.Store(ctx, []*schema.Document{
		{ID: "doc-1", Content: "new", MetaData: map[string]any{"updated": true}},
	}); err != nil {
		t.Fatalf("second store failed: %v", err)
	}

	var content, metadataJSON string
	if err := db.QueryRow(`SELECT content, metadata_json FROM eino_sqlitevec_documents WHERE doc_id = ?`, "doc-1").
		Scan(&content, &metadataJSON); err != nil {
		t.Fatalf("select document failed: %v", err)
	}
	if content != "new" {
		t.Fatalf("expected updated content, got %q", content)
	}
	if !strings.Contains(metadataJSON, `"updated":true`) {
		t.Fatalf("expected updated metadata, got %s", metadataJSON)
	}

	var vectorCount int
	if err := db.QueryRow(`SELECT count(*) FROM eino_sqlitevec_vectors`).Scan(&vectorCount); err != nil {
		t.Fatalf("count vectors failed: %v", err)
	}
	if vectorCount != 1 {
		t.Fatalf("expected one vector after upsert, got %d", vectorCount)
	}
}

func TestIndexerTypeAndCallbacks(t *testing.T) {
	idx, err := NewIndexer(context.Background(), &Config{
		DB:        openTestDB(t),
		VectorDim: 3,
		Embedding: &mockEmbedding{
			vector: []float64{0.1, 0.2, 0.3},
		},
	})
	if err != nil {
		t.Fatalf("NewIndexer failed: %v", err)
	}
	if got := idx.GetType(); got != "SQLiteVec" {
		t.Fatalf("unexpected type: %s", got)
	}
	if !idx.IsCallbacksEnabled() {
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

type mockEmbedding struct {
	vector  []float64
	vectors [][]float64
	err     error
	calls   int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.vectors != nil {
		start := m.calls
		m.calls += len(texts)
		end := start + len(texts)
		if end > len(m.vectors) {
			end = len(m.vectors)
		}
		return m.vectors[start:end], nil
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = append([]float64(nil), m.vector...)
	}
	return result, nil
}
