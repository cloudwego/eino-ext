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
	"github.com/cloudwego/eino/components/retriever"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

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

// mockConn is a mock implementation of PgxConn for testing.
type mockConn struct{}

func (m *mockConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
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

// Ensure mock types implement interfaces
var _ embedding.Embedder = (*mockEmbedder)(nil)
var _ PgxConn = (*mockConn)(nil)
var _ pgx.Rows = (*mockRows)(nil)
var _ retriever.Option = WithWhereClause("")
var _ retriever.Option = WithDistanceFunction(DistanceCosine)
