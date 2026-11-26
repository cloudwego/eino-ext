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
	"fmt"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// Helper function for creating float64 pointers
func float64Ptr(f float64) *float64 {
	return &f
}

// mockEmbedder is a mock implementation of embedding.Embedder for testing.
type mockEmbedder struct {
	vector []float64
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	if m.vector != nil {
		result := make([][]float64, len(texts))
		for i := range result {
			result[i] = m.vector
		}
		return result, nil
	}
	// Return default 3-dimensional vector
	result := make([][]float64, len(texts))
	for i := range result {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

func TestDistanceFunction(t *testing.T) {
	tests := []struct {
		name  string
		fn    DistanceFunction
		op    string
		valid bool
	}{
		{
			name:  "cosine distance",
			fn:    DistanceCosine,
			op:    "<=>",
			valid: true,
		},
		{
			name:  "l2 distance",
			fn:    DistanceL2,
			op:    "<->",
			valid: true,
		},
		{
			name:  "ip distance",
			fn:    DistanceIP,
			op:    "<#>",
			valid: true,
		},
		{
			name:  "invalid distance",
			fn:    DistanceFunction("invalid"),
			op:    "<=>",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn.Validate()
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.op, tt.fn.Operator())
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestNewRetriever(t *testing.T) {
	ctx := context.Background()

	config := &RetrieverConfig{
		Conn:             &mockConn{},
		Embedding:        &mockEmbedder{},
		DistanceFunction: DistanceCosine,
	}

	r, err := NewRetriever(ctx, config)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "PGVector", r.GetType())
	assert.True(t, r.IsCallbacksEnabled())
}

func TestNewRetrieverMissingEmbedding(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn: &mockConn{},
	}

	_, err := NewRetriever(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding not provided")
}

func TestNewRetrieverMissingConn(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Embedding: &mockEmbedder{},
	}

	_, err := NewRetriever(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection not provided")
}

func TestNewRetrieverInvalidDistanceFunction(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn:             &mockConn{},
		Embedding:        &mockEmbedder{},
		DistanceFunction: DistanceFunction("invalid"),
	}

	_, err := NewRetriever(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid distance function")
}

func TestWithWhereClause(t *testing.T) {
	opt := WithWhereClause("metadata->>'category' = 'tech'")
	assert.NotNil(t, opt)
}

func TestWithDistanceFunction(t *testing.T) {
	opt := WithDistanceFunction(DistanceL2)
	assert.NotNil(t, opt)
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

func TestNewRetrieverInvalidTableName(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn:      &mockConn{},
		Embedding: &mockEmbedder{},
		TableName: "invalid-table-name",
	}

	_, err := NewRetriever(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid table name")
}

func TestCalculateScore(t *testing.T) {
	tests := []struct {
		name             string
		distanceFunction DistanceFunction
		distance         float64
		expectedScore    float64
	}{
		{
			name:             "cosine distance",
			distanceFunction: DistanceCosine,
			distance:         0.2,
			expectedScore:    0.8,
		},
		{
			name:             "l2 distance",
			distanceFunction: DistanceL2,
			distance:         1.0,
			expectedScore:    0.5,
		},
		{
			name:             "ip distance",
			distanceFunction: DistanceIP,
			distance:         0.5,
			expectedScore:    0.6666666666666666,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			config := &RetrieverConfig{
				Conn:             &mockConn{},
				Embedding:        &mockEmbedder{},
				DistanceFunction: tt.distanceFunction,
			}
			r, _ := NewRetriever(ctx, config)
			score := r.calculateScore(tt.distance)
			assert.InDelta(t, tt.expectedScore, score, 0.0001)
		})
	}
}

func TestCalculateThresholdDistance(t *testing.T) {
	tests := []struct {
		name             string
		distanceFunction DistanceFunction
		scoreThreshold   float64
		expectedDistance float64
	}{
		{
			name:             "cosine threshold",
			distanceFunction: DistanceCosine,
			scoreThreshold:   0.8,
			expectedDistance: 0.2,
		},
		{
			name:             "l2 threshold",
			distanceFunction: DistanceL2,
			scoreThreshold:   1.0,
			expectedDistance: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			config := &RetrieverConfig{
				Conn:             &mockConn{},
				Embedding:        &mockEmbedder{},
				DistanceFunction: tt.distanceFunction,
			}
			r, _ := NewRetriever(ctx, config)
			distance := r.calculateThresholdDistance(tt.scoreThreshold)
			assert.InDelta(t, tt.expectedDistance, distance, 0.0001)
		})
	}
}

func TestNewRetrieverPingFailed(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn:      &mockConn{pingFail: true},
		Embedding: &mockEmbedder{},
	}

	_, err := NewRetriever(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestRetrieveSuccess(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn:             &mockConnWithRows{},
		Embedding:        &mockEmbedder{},
		DistanceFunction: DistanceCosine,
		TopK:             5,
	}

	r, err := NewRetriever(ctx, config)
	assert.NoError(t, err)

	docs, err := r.Retrieve(ctx, "test query")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(docs))
	assert.Equal(t, "doc1", docs[0].ID)
	assert.Equal(t, "doc2", docs[1].ID)
	assert.Equal(t, 1.0, docs[0].Score())
}

func TestRetrieveQueryFailed(t *testing.T) {
	ctx := context.Background()
	config := &RetrieverConfig{
		Conn:             &mockConn{queryFail: true},
		Embedding:        &mockEmbedder{},
		DistanceFunction: DistanceCosine,
		TopK:             5,
	}

	r, err := NewRetriever(ctx, config)
	assert.NoError(t, err)

	_, err = r.Retrieve(ctx, "test query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query failed")
}

func TestRetrieveWithScoreThreshold(t *testing.T) {
	ctx := context.Background()
	threshold := 0.8
	config := &RetrieverConfig{
		Conn:             &mockConnWithRows{},
		Embedding:        &mockEmbedder{},
		DistanceFunction: DistanceCosine,
		TopK:             5,
		ScoreThreshold:   &threshold,
	}

	r, err := NewRetriever(ctx, config)
	assert.NoError(t, err)

	docs, err := r.Retrieve(ctx, "test query")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(docs))
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name            string
		whereClause     string
		scoreThreshold  *float64
		distanceFunc    DistanceFunction
		expectedSubstr  string
	}{
		{
			name:         "no filters",
			whereClause:  "",
			scoreThreshold: nil,
			distanceFunc: DistanceCosine,
			expectedSubstr: "ORDER BY distance ASC LIMIT $2",
		},
		{
			name:         "with where clause",
			whereClause:  "metadata->>'category' = 'tech'",
			scoreThreshold: nil,
			distanceFunc: DistanceCosine,
			expectedSubstr: "WHERE metadata->>'category' = 'tech'",
		},
		{
			name:            "with score threshold",
			whereClause:     "",
			scoreThreshold:  float64Ptr(0.8),
			distanceFunc:    DistanceCosine,
			expectedSubstr:  "(embedding <=> $1) < 0.200000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			config := &RetrieverConfig{
				Conn:             &mockConn{},
				Embedding:        &mockEmbedder{},
				DistanceFunction: tt.distanceFunc,
			}
			r, _ := NewRetriever(ctx, config)

			query := r.buildSearchQuery(tt.whereClause, tt.scoreThreshold)
			assert.Contains(t, query, tt.expectedSubstr)
		})
	}
}

// mockConnWithRows is a mock that returns actual rows
type mockConnWithRows struct{}

func (m *mockConnWithRows) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return newMockRowsWithData(), nil
}

func (m *mockConnWithRows) Ping(ctx context.Context) error {
	return nil
}

type mockRowsWithData struct {
	currentRow int
	rows       []struct {
		id       string
		content  string
		metadata map[string]any
		distance float64
	}
}

func newMockRowsWithData() *mockRowsWithData {
	return &mockRowsWithData{
		currentRow: 0,
		rows: []struct {
			id       string
			content  string
			metadata map[string]any
			distance float64
		}{
			{
				id:       "doc1",
				content:  "test content 1",
				metadata: map[string]any{"category": "test"},
				distance: 0.0,
			},
			{
				id:       "doc2",
				content:  "test content 2",
				metadata: map[string]any{"category": "test"},
				distance: 0.1,
			},
		},
	}
}

func (m *mockRowsWithData) Close() {}
func (m *mockRowsWithData) Err() error { return nil }
func (m *mockRowsWithData) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("0 0 0")
}
func (m *mockRowsWithData) Next() bool {
	if m.currentRow < len(m.rows) {
		m.currentRow++
		return true
	}
	return false
}

func (m *mockRowsWithData) Scan(dest ...any) error {
	if m.currentRow > 0 && m.currentRow <= len(m.rows) {
		row := m.rows[m.currentRow-1]
		if len(dest) >= 4 {
			if str, ok := dest[0].(*string); ok {
				*str = row.id
			}
			if str, ok := dest[1].(*string); ok {
				*str = row.content
			}
			if meta, ok := dest[2].(*map[string]any); ok {
				*meta = row.metadata
			}
			if f, ok := dest[3].(*float64); ok {
				*f = row.distance
			}
		}
	}
	return nil
}

func (m *mockRowsWithData) Values() ([]any, error) { return nil, nil }
func (m *mockRowsWithData) RawValues() [][]byte { return nil }
func (m *mockRowsWithData) Conn() *pgx.Conn { return nil }
func (m *mockRowsWithData) FieldDescriptions() []pgconn.FieldDescription { return nil }


// mockConn is a mock implementation of PgxConn for testing.
type mockConn struct {
	pingFail   bool
	queryFail  bool
}

func (m *mockConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFail {
		return nil, fmt.Errorf("query failed")
	}
	return &mockRows{}, nil
}

func (m *mockConn) Ping(ctx context.Context) error {
	if m.pingFail {
		return fmt.Errorf("ping failed")
	}
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

// Ensure mock types implement interfaces
var _ embedding.Embedder = (*mockEmbedder)(nil)
var _ PgxConn = (*mockConn)(nil)
var _ pgx.Rows = (*mockRows)(nil)
var _ retriever.Option = WithWhereClause("")
var _ retriever.Option = WithDistanceFunction(DistanceCosine)
