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

package pgvector

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// mockEmbedder is a mock implementation of embedding.Embedder for testing.
type mockEmbedder struct {
	vectors [][]float64
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	if len(m.vectors) == 0 {
		// Return default vectors
		result := make([][]float64, len(texts))
		for i := range result {
			result[i] = make([]float64, 3)
			result[i][0] = 0.1
			result[i][1] = 0.2
			result[i][2] = 0.3
		}
		return result, nil
	}
	return m.vectors, nil
}

// TestNewIndexer tests the NewIndexer function with mock connection.
func TestNewIndexer(t *testing.T) {
	ctx := context.Background()

	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)
	assert.NotNil(t, indexer)
	assert.Equal(t, "PGVector", indexer.GetType())
}

func TestNewIndexerMissingConn(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Embedding: &mockEmbedder{},
	}

	_, err := NewIndexer(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection not provided")
}

// mockConn is a mock implementation of PgxConn for testing.
type mockConn struct{}

func (m *mockConn) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0 0 0"), nil
}

func (m *mockConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}

func (m *mockConn) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return &mockBatchResults{}
}

func (m *mockConn) Ping(ctx context.Context) error {
	return nil
}

type mockRows struct{}

func (m *mockRows) Close() {
	return
}

func (m *mockRows) Err() error {
	return nil
}

func (m *mockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("0 0 0")
}

func (m *mockRows) Next() bool {
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	return nil
}

func (m *mockRows) Values() ([]any, error) {
	return nil, nil
}

func (m *mockRows) RawValues() [][]byte {
	return nil
}

func (m *mockRows) Conn() *pgx.Conn {
	return nil
}

func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

type mockBatchResults struct{}

func (m *mockBatchResults) Exec() (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0 0 0"), nil
}

func (m *mockBatchResults) Query() (pgx.Rows, error) {
	return &mockRows{}, nil
}

func (m *mockBatchResults) QueryRow() pgx.Row {
	return &mockRow{}
}

func (m *mockBatchResults) Close() error {
	return nil
}

type mockRow struct{}

func (m *mockRow) Scan(dest ...any) error {
	return nil
}

// Ensure mock types implement interfaces
var _ embedding.Embedder = (*mockEmbedder)(nil)
var _ PgxConn = (*mockConn)(nil)
var _ pgx.BatchResults = (*mockBatchResults)(nil)

func TestIndexerIsCallbacksEnabled(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)
	assert.True(t, indexer.IsCallbacksEnabled())
}

func TestStoreDocuments(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
		TableName: "test_table",
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)

	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "test content 1",
			MetaData: map[string]any{
				"key": "value1",
			},
		},
		{
			ID:      "doc2",
			Content: "test content 2",
			MetaData: map[string]any{
				"key": "value2",
			},
		},
	}

	ids, err := indexer.Store(ctx, docs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ids))
	assert.Equal(t, "doc1", ids[0])
	assert.Equal(t, "doc2", ids[1])
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{
			name:       "valid simple",
			identifier: "table_name",
			wantErr:    false,
		},
		{
			name:       "valid with underscore",
			identifier: "_table",
			wantErr:    false,
		},
		{
			name:       "valid with digits",
			identifier: "table123",
			wantErr:    false,
		},
		{
			name:       "empty",
			identifier: "",
			wantErr:    true,
		},
		{
			name:       "starts with digit",
			identifier: "123table",
			wantErr:    true,
		},
		{
			name:       "contains hyphen",
			identifier: "table-name",
			wantErr:    true,
		},
		{
			name:       "contains space",
			identifier: "table name",
			wantErr:    true,
		},
		{
			name:       "starts with special char",
			identifier: "$table",
			wantErr:    true,
		},
		{
			name:       "SQL injection attempt",
			identifier: "table; DROP TABLE users--",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentifier(tt.identifier)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewIndexerInvalidTableName(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
		TableName: "invalid-table-name",
	}

	_, err := NewIndexer(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid table name")
}

func TestStoreEmptyDocuments(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)

	_, err = indexer.Store(ctx, []*schema.Document{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "documents list is empty")
}

func TestStoreNilDocument(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)

	docs := []*schema.Document{
		{ID: "doc1", Content: "content1"},
		nil,
		{ID: "doc2", Content: "content2"},
	}

	_, err = indexer.Store(ctx, docs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document at index 1 is nil")
}

func TestStoreMissingEmbedding(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn: &mockConn{},
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)

	docs := []*schema.Document{
		{ID: "doc1", Content: "content1"},
	}

	_, err = indexer.Store(ctx, docs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding not provided")
}

func TestStoreInvalidBatchSize(t *testing.T) {
	ctx := context.Background()
	config := &IndexerConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
		BatchSize: -1,
	}

	indexer, err := NewIndexer(ctx, config)
	assert.NoError(t, err)

	docs := []*schema.Document{
		{ID: "doc1", Content: "content1"},
	}

	_, err = indexer.Store(ctx, docs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid batch size")
}
