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

package milvus2

import (
	"context"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func TestAutoSearchMode_Retrieve_Std(t *testing.T) {
	ctx := context.Background()
	mockClient := &milvusclient.Client{}
	mockEmb := &mockEmbedding{dims: 128}
	auto := &autoSearchMode{}

	mockConverter := func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
		return []*schema.Document{}, nil
	}

	t.Run("Dense with SearchParams", func(t *testing.T) {
		PatchConvey("mock", t, func() {
			Mock(GetMethod(mockClient, "Search")).Return([]milvusclient.ResultSet{
				{ResultCount: 1},
			}, nil).Build()

			conf := &RetrieverConfig{
				VectorField: "dense_vec",
				Embedding:   mockEmb,
				SearchParams: map[string]map[string]interface{}{
					"dense_vec": {"nprobe": 10},
				},
				DocumentConverter: mockConverter,
			}

			// We keep recover just in case, but it should pass now
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Recovered from panic: %v", r)
				}
			}()

			_, err := auto.Retrieve(ctx, mockClient, conf, "query")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("Hybrid", func(t *testing.T) {
		PatchConvey("mock", t, func() {
			Mock(GetMethod(mockClient, "HybridSearch")).Return([]milvusclient.ResultSet{
				{ResultCount: 1},
			}, nil).Build()

			conf := &RetrieverConfig{
				VectorField:       "dense_vec",
				SparseVectorField: "sparse_vec",
				Embedding:         mockEmb,
				DocumentConverter: mockConverter,
			}

			_, err := auto.Retrieve(ctx, mockClient, conf, "query")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("Sparse Only", func(t *testing.T) {
		PatchConvey("mock", t, func() {
			Mock(GetMethod(mockClient, "Search")).Return([]milvusclient.ResultSet{
				{ResultCount: 1},
			}, nil).Build()

			// Only SparseVectorField configured, no VectorField
			conf := &RetrieverConfig{
				SparseVectorField: "sparse_vec",
				DocumentConverter: mockConverter,
				// No Embedding needed for sparse-only search
			}

			_, err := auto.Retrieve(ctx, mockClient, conf, "query")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
}
