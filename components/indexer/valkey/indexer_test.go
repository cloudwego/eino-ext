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

package valkey

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
)

type mockBatchClient struct {
	execCount int
	execErr   error
}

func (m *mockBatchClient) Exec(_ context.Context, _ [][]string) ([]any, error) {
	m.execCount++
	if m.execErr != nil {
		return nil, m.execErr
	}
	return []any{}, nil
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.cnt >= len(m.sizeForCall) {
		return nil, fmt.Errorf("unexpected call")
	}
	slice := make([]float64, m.dims)
	for i := range slice {
		slice[i] = 1.1
	}
	r := make([][]float64, m.sizeForCall[m.cnt])
	m.cnt++
	for i := range r {
		r[i] = slice
	}
	return r, nil
}

func TestNewIndexer(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	tests := []struct {
		name    string
		config  *IndexerConfig
		wantErr string
	}{
		{
			name:    "embedding not provided",
			config:  &IndexerConfig{Client: client},
			wantErr: "[NewIndexer] embedding not provided for valkey indexer",
		},
		{
			name:    "client not provided",
			config:  &IndexerConfig{Embedding: &mockEmbedding{}},
			wantErr: "[NewIndexer] valkey client not provided",
		},
		{
			name:   "success",
			config: &IndexerConfig{Client: client, Embedding: &mockEmbedding{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := NewIndexer(ctx, tt.config)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				if idx != nil {
					t.Fatal("expected nil indexer on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if idx == nil {
					t.Fatal("expected non-nil indexer")
				}
			}
		})
	}
}

func TestNewIndexer_DoesNotMutateConfig(t *testing.T) {
	ctx := context.Background()
	cfg := &IndexerConfig{
		Client:    &mockBatchClient{},
		Embedding: &mockEmbedding{},
	}
	_, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BatchSize != 0 {
		t.Fatal("NewIndexer mutated caller's config")
	}
}

func TestIndexer_Store_Hash_Success(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:           client,
		KeyPrefix:        "test:",
		DocumentType:     DocumentTypeHash,
		DocumentToHashes: defaultDocumentToFields,
		BatchSize:        10,
		Embedding:        &mockEmbedding{sizeForCall: []int{2}, dims: 4},
	}}

	docs := []*schema.Document{
		{ID: "1", Content: "hello"},
		{ID: "2", Content: "world", MetaData: map[string]any{"tag": "test"}},
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if client.execCount != 1 {
		t.Fatalf("expected 1 batch exec call, got %d", client.execCount)
	}
}

func TestIndexer_Store_JSON_Success(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:         client,
		KeyPrefix:      "json:",
		DocumentType:   DocumentTypeJSON,
		DocumentToJSON: defaultDocumentToJSON,
		BatchSize:      10,
		Embedding:      &mockEmbedding{sizeForCall: []int{2}, dims: 4},
	}}

	docs := []*schema.Document{
		{ID: "1", Content: "hello"},
		{ID: "2", Content: "world", MetaData: map[string]any{"tag": "test"}},
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if client.execCount != 1 {
		t.Fatalf("expected 1 batch exec call, got %d", client.execCount)
	}
}

func TestIndexer_Store_EmbeddingError(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:           client,
		DocumentType:     DocumentTypeHash,
		DocumentToHashes: defaultDocumentToFields,
		BatchSize:        10,
		Embedding:        &mockEmbedding{err: fmt.Errorf("embed error")},
	}}

	docs := []*schema.Document{{ID: "1", Content: "hello"}}
	_, err := idx.Store(ctx, docs)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIndexer_Store_BatchSizeExceeded(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client: client,
		DocumentToHashes: func(_ context.Context, doc *schema.Document) (*Hashes, error) {
			return &Hashes{
				Key: doc.ID,
				Field2Value: map[string]FieldValue{
					"f1": {Value: "v1", EmbedKey: "e1"},
					"f2": {Value: "v2", EmbedKey: "e2"},
				},
			}, nil
		},
		BatchSize: 1,
		Embedding: &mockEmbedding{},
	}}

	docs := []*schema.Document{{ID: "1", Content: "hello"}}
	_, err := idx.Store(ctx, docs)
	if err == nil {
		t.Fatal("expected batch size error")
	}
}

func TestIndexer_Store_DocumentToHashesError(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client: client,
		DocumentToHashes: func(_ context.Context, _ *schema.Document) (*Hashes, error) {
			return nil, fmt.Errorf("conversion error")
		},
		BatchSize: 10,
		Embedding: &mockEmbedding{},
	}}

	docs := []*schema.Document{{ID: "1", Content: "hello"}}
	_, err := idx.Store(ctx, docs)
	if err == nil || err.Error() != "conversion error" {
		t.Fatalf("expected conversion error, got: %v", err)
	}
}

func TestDefaultDocumentToFields(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		doc := &schema.Document{ID: "test", Content: "hello", MetaData: map[string]any{"k": "v"}}
		h, err := defaultDocumentToFields(context.Background(), doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h.Key != "test" {
			t.Fatalf("unexpected key: %s", h.Key)
		}
		if h.Field2Value[defaultReturnFieldContent].EmbedKey != defaultReturnFieldVectorContent {
			t.Fatal("unexpected embed key")
		}
	})

	t.Run("empty id", func(t *testing.T) {
		doc := &schema.Document{Content: "hello"}
		_, err := defaultDocumentToFields(context.Background(), doc)
		if err == nil {
			t.Fatal("expected error for empty id")
		}
	})

	t.Run("metadata conflicts with reserved field", func(t *testing.T) {
		doc := &schema.Document{ID: "test", Content: "hello", MetaData: map[string]any{"content": "override"}}
		_, err := defaultDocumentToFields(context.Background(), doc)
		if err == nil {
			t.Fatal("expected error for reserved field conflict")
		}
	})
}

func TestDefaultDocumentToJSON(t *testing.T) {
	doc := &schema.Document{ID: "test", Content: "hello", MetaData: map[string]any{"k": "v"}}
	vec := []float64{1.0, 2.0}
	m, err := defaultDocumentToJSON(context.Background(), doc, vec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m[defaultReturnFieldContent] != "hello" {
		t.Fatal("content mismatch")
	}
	if m["k"] != "v" {
		t.Fatal("metadata mismatch")
	}
	vecResult := m[defaultReturnFieldVectorContent].([]float64)
	if len(vecResult) != 2 {
		t.Fatal("vector mismatch")
	}
}

func TestDefaultDocumentToJSON_ReservedFieldConflict(t *testing.T) {
	doc := &schema.Document{ID: "test", Content: "hello", MetaData: map[string]any{"vector_content": "bad"}}
	_, err := defaultDocumentToJSON(context.Background(), doc, []float64{1.0})
	if err == nil {
		t.Fatal("expected error for reserved field conflict")
	}
}

func TestIndexer_GetType(t *testing.T) {
	idx := &Indexer{}
	if idx.GetType() != "Valkey" {
		t.Fatalf("expected Valkey, got %s", idx.GetType())
	}
}

func TestIndexer_IsCallbacksEnabled(t *testing.T) {
	idx := &Indexer{}
	if !idx.IsCallbacksEnabled() {
		t.Fatal("expected callbacks enabled")
	}
}

func TestIndexer_Store_WithOptions(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:           client,
		DocumentToHashes: defaultDocumentToFields,
		BatchSize:        10,
		Embedding:        &mockEmbedding{sizeForCall: []int{1}, dims: 4},
	}}

	docs := []*schema.Document{{ID: "1", Content: "hello"}}
	ids, err := idx.Store(ctx, docs, indexer.WithEmbedding(&mockEmbedding{sizeForCall: []int{1}, dims: 4}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("unexpected ids: %v", ids)
	}
}

func TestIndexer_Store_MultiBatch(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:           client,
		DocumentToHashes: defaultDocumentToFields,
		BatchSize:        2, // Force multiple batches
		Embedding:        &mockEmbedding{sizeForCall: []int{2, 2, 1}, dims: 4},
	}}

	docs := []*schema.Document{
		{ID: "1", Content: "a"},
		{ID: "2", Content: "b"},
		{ID: "3", Content: "c"},
		{ID: "4", Content: "d"},
		{ID: "5", Content: "e"},
	}

	ids, err := idx.Store(ctx, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("expected 5 ids, got %d", len(ids))
	}
	if client.execCount != 3 {
		t.Fatalf("expected 3 batch exec calls, got %d", client.execCount)
	}
}

func TestIndexer_Store_JSON_EmptyID(t *testing.T) {
	ctx := context.Background()
	client := &mockBatchClient{}

	idx := &Indexer{config: &IndexerConfig{
		Client:         client,
		DocumentType:   DocumentTypeJSON,
		DocumentToJSON: defaultDocumentToJSON,
		BatchSize:      10,
		Embedding:      &mockEmbedding{sizeForCall: []int{1}, dims: 4},
	}}

	docs := []*schema.Document{{ID: "", Content: "hello"}}
	_, err := idx.Store(ctx, docs)
	if err == nil {
		t.Fatal("expected error for empty doc ID")
	}
}
