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
	"github.com/cloudwego/eino/components/retriever"
)

type mockSearchClient struct {
	customCommandFn func(ctx context.Context, args []string) (any, error)
}

func (m *mockSearchClient) CustomCommand(ctx context.Context, args []string) (any, error) {
	if m.customCommandFn != nil {
		return m.customCommandFn(ctx, args)
	}
	return nil, nil
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(_ context.Context, _ []string, _ ...embedding.Option) ([][]float64, error) {
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

func TestNewRetriever(t *testing.T) {
	ctx := context.Background()
	client := &mockSearchClient{}

	tests := []struct {
		name    string
		config  *RetrieverConfig
		wantErr string
	}{
		{
			name:    "embedding not provided",
			config:  &RetrieverConfig{Client: client, Index: "idx"},
			wantErr: "[NewRetriever] embedding not provided for valkey retriever",
		},
		{
			name:    "index not provided",
			config:  &RetrieverConfig{Client: client, Embedding: &mockEmbedding{}},
			wantErr: "[NewRetriever] valkey index not provided",
		},
		{
			name:    "client not provided",
			config:  &RetrieverConfig{Index: "idx", Embedding: &mockEmbedding{}},
			wantErr: "[NewRetriever] valkey client not provided",
		},
		{
			name:   "success with defaults",
			config: &RetrieverConfig{Client: client, Index: "idx", Embedding: &mockEmbedding{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewRetriever(ctx, tt.config)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				if r != nil {
					t.Fatal("expected nil retriever on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if r == nil {
					t.Fatal("expected non-nil retriever")
				}
			}
		})
	}
}

func TestNewRetriever_DoesNotMutateConfig(t *testing.T) {
	ctx := context.Background()
	cfg := &RetrieverConfig{
		Client:    &mockSearchClient{},
		Index:     "idx",
		Embedding: &mockEmbedding{},
	}
	_, err := NewRetriever(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TopK != 0 {
		t.Fatal("NewRetriever mutated caller's config")
	}
}

func TestRetriever_Retrieve_EmbeddingError(t *testing.T) {
	ctx := context.Background()
	client := &mockSearchClient{}
	mockErr := fmt.Errorf("embedding failed")

	r := &Retriever{config: &RetrieverConfig{
		Client:            client,
		Index:             "idx",
		Embedding:         &mockEmbedding{err: mockErr},
		TopK:              5,
		VectorField:       defaultReturnFieldVectorContent,
		ReturnFields:      []string{defaultReturnFieldContent, defaultReturnFieldVectorContent},
		DocumentConverter: defaultResultParser([]string{defaultReturnFieldContent, defaultReturnFieldVectorContent}),
		Dialect:           2,
	}}

	_, err := r.Retrieve(ctx, "test query")
	if err == nil || err.Error() != "embedding failed" {
		t.Fatalf("expected embedding error, got: %v", err)
	}
}

func TestRetriever_Retrieve_InvalidVectorLength(t *testing.T) {
	ctx := context.Background()
	client := &mockSearchClient{}

	r := &Retriever{config: &RetrieverConfig{
		Client:            client,
		Index:             "idx",
		Embedding:         &mockEmbedding{sizeForCall: []int{2}, dims: 10},
		TopK:              5,
		VectorField:       defaultReturnFieldVectorContent,
		ReturnFields:      []string{defaultReturnFieldContent, defaultReturnFieldVectorContent},
		DocumentConverter: defaultResultParser([]string{defaultReturnFieldContent, defaultReturnFieldVectorContent}),
		Dialect:           2,
	}}

	_, err := r.Retrieve(ctx, "test query")
	if err == nil {
		t.Fatal("expected error for invalid vector length")
	}
	expected := "[valkey retriever] invalid return length of vector, got=2, expected=1"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestRetriever_Retrieve_KNN(t *testing.T) {
	ctx := context.Background()
	expVec := make([]float64, 10)
	for i := range expVec {
		expVec[i] = 1.1
	}

	client := &mockSearchClient{
		customCommandFn: func(_ context.Context, args []string) (any, error) {
			// Mock FT.SEARCH response: [totalCount, {key: map[field]value}]
			return []any{
				int64(2),
				map[string]any{
					"doc:1": map[string]any{"content": "hello", "vector_content": string(vector2Bytes(expVec))},
					"doc:2": map[string]any{"content": "world", "vector_content": string(vector2Bytes(expVec))},
				},
			}, nil
		},
	}

	r, err := NewRetriever(ctx, &RetrieverConfig{
		Client:    client,
		Index:     "test_index",
		Embedding: &mockEmbedding{sizeForCall: []int{1}, dims: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, err := r.Retrieve(ctx, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	for _, doc := range docs {
		if doc.Content != "hello" && doc.Content != "world" {
			t.Fatalf("unexpected content: %s", doc.Content)
		}
	}
}

func TestRetriever_Retrieve_VectorRange(t *testing.T) {
	ctx := context.Background()
	expVec := make([]float64, 10)
	for i := range expVec {
		expVec[i] = 1.1
	}

	client := &mockSearchClient{
		customCommandFn: func(_ context.Context, args []string) (any, error) {
			return []any{
				int64(1),
				map[string]any{
					"doc:1": map[string]any{"content": "hello", "vector_content": string(vector2Bytes(expVec))},
				},
			}, nil
		},
	}

	dis := 10.0
	r, err := NewRetriever(ctx, &RetrieverConfig{
		Client:            client,
		Index:             "test_index",
		DistanceThreshold: &dis,
		Embedding:         &mockEmbedding{sizeForCall: []int{1}, dims: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, err := r.Retrieve(ctx, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].Content != "hello" {
		t.Fatalf("unexpected content: %s", docs[0].Content)
	}
}

func TestVector2Bytes_Bytes2Vector_Roundtrip(t *testing.T) {
	input := []float64{1.5, 2.5, 3.5, 0.0, -1.0}
	b := vector2Bytes(input)
	output := bytes2Vector(b)
	if len(output) != len(input) {
		t.Fatalf("length mismatch: got %d, want %d", len(output), len(input))
	}
	for i := range input {
		diff := input[i] - output[i]
		if diff > 0.001 || diff < -0.001 {
			t.Fatalf("value mismatch at %d: got %f, want %f", i, output[i], input[i])
		}
	}
}

func TestRetriever_GetType(t *testing.T) {
	r := &Retriever{}
	if r.GetType() != "Valkey" {
		t.Fatalf("expected Valkey, got %s", r.GetType())
	}
}

func TestRetriever_IsCallbacksEnabled(t *testing.T) {
	r := &Retriever{}
	if !r.IsCallbacksEnabled() {
		t.Fatal("expected callbacks enabled")
	}
}

func TestDefaultResultParser(t *testing.T) {
	parser := defaultResultParser([]string{defaultReturnFieldContent, defaultReturnFieldVectorContent})
	vec := []float64{1.0, 2.0}

	t.Run("success", func(t *testing.T) {
		doc, err := parser(context.Background(), FtSearchDocument{
			Key: "id1",
			Fields: map[string]any{
				defaultReturnFieldContent:       "hello",
				defaultReturnFieldVectorContent: string(vector2Bytes(vec)),
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.ID != "id1" || doc.Content != "hello" {
			t.Fatalf("unexpected doc: %+v", doc)
		}
	})

	t.Run("missing field is skipped", func(t *testing.T) {
		doc, err := parser(context.Background(), FtSearchDocument{
			Key:    "id1",
			Fields: map[string]any{defaultReturnFieldVectorContent: string(vector2Bytes(vec))},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Content != "" {
			t.Fatalf("expected empty content for missing field, got %q", doc.Content)
		}
	})

	t.Run("non-string content field returns error", func(t *testing.T) {
		_, err := parser(context.Background(), FtSearchDocument{
			Key: "id1",
			Fields: map[string]any{
				defaultReturnFieldContent:       123,
				defaultReturnFieldVectorContent: string(vector2Bytes(vec)),
			},
		})
		if err == nil {
			t.Fatal("expected error for non-string content")
		}
	})
}

func TestRetriever_Retrieve_FilterQueryValidation(t *testing.T) {
	ctx := context.Background()

	r := &Retriever{config: &RetrieverConfig{
		Client:            &mockSearchClient{},
		Index:             "idx",
		Embedding:         &mockEmbedding{sizeForCall: []int{1}, dims: 4},
		TopK:              5,
		VectorField:       defaultReturnFieldVectorContent,
		ReturnFields:      []string{defaultReturnFieldContent},
		DocumentConverter: defaultResultParser([]string{defaultReturnFieldContent}),
		Dialect:           2,
	}}

	tests := []struct {
		name    string
		filter  string
		wantErr bool
	}{
		{"valid tag filter", "@category:{electronics}", false},
		{"injection with =>", "@tag:{x}=>[KNN 100 @vec $v]", true},
		{"injection with KNN", "*)=>[KNN 9999 @vec $v AS d", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Retrieve(ctx, "query", retriever.WrapImplSpecificOptFn(func(o *implOptions) {
				o.FilterQuery = tt.filter
			}))
			if tt.wantErr && err == nil {
				t.Fatal("expected error for disallowed filter")
			}
			if !tt.wantErr && err != nil && err.Error() == "[valkey retriever] filter contains disallowed syntax" {
				t.Fatalf("unexpected validation error for valid filter: %v", err)
			}
		})
	}
}

func TestValidateFilterQuery(t *testing.T) {
	tests := []struct {
		filter  string
		wantErr bool
	}{
		{"@tag:{val}", false},
		{"@num:[0 100]", false},
		{"*", false},
		{"@x:{y}=>[KNN 10 @v $v]", true},
		{"something [KNN bypass", true},
	}
	for _, tt := range tests {
		err := validateFilterQuery(tt.filter)
		if tt.wantErr && err == nil {
			t.Errorf("expected error for %q", tt.filter)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("unexpected error for %q: %v", tt.filter, err)
		}
	}
}
