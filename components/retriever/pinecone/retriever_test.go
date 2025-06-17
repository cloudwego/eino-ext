/*
 * Copyright 2024 CloudWeGo Authors
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

package pinecone

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()

		PatchConvey("test pinecone NewClient failed", func() {
			mockErr := fmt.Errorf("mock err")
			Mock(pinecone.NewClient).Return(nil, mockErr).Build()
			r, err := NewRetriever(ctx, &RetrieverConfig{})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("pinecone: Failed to create Client: %w", mockErr))
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test DescribeIndex failed", func() {
			mockErr := fmt.Errorf("mock err")
			pc := &pinecone.Client{}
			Mock(pinecone.NewClient).Return(pc, nil).Build()
			Mock(GetMethod(pc, "DescribeIndex")).Return(nil, mockErr).Build()
			r, err := NewRetriever(ctx, &RetrieverConfig{IndexName: "mock_index"})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("pinecone: Failed to describe index mock_index: %w", mockErr))
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			pc := &pinecone.Client{}
			idx := &pinecone.Index{}
			Mock(pinecone.NewClient).Return(pc, nil).Build()
			Mock(GetMethod(pc, "DescribeIndex")).Return(idx, nil).Build()
			Mock(GetMethod(pc, "Index")).Return(&pinecone.IndexConnection{}, nil).Build()
			r, err := NewRetriever(ctx, &RetrieverConfig{IndexName: "mock_index"})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
		})
	})
}

func TestMakeQueryRequest(t *testing.T) {
	PatchConvey("test makeQueryRequest", t, func() {
		ctx := context.Background()
		query := "test_query"
		r := &Retriever{
			conf: &RetrieverConfig{},
		}

		PatchConvey("test embedding is nil", func() {
			req, err := r.makeQueryRequest(ctx, query, &retriever.Options{
				TopK:      of(10),
				Embedding: nil,
			}, &options{
				DenseVector:    nil,
				SparseVector:   map[int]float64{1: 1.2},
				MetadataFilter: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeQueryRequest] embedding method in config must not be nil when query not contains dense vector"))
			convey.So(req, convey.ShouldBeNil)
		})

		PatchConvey("test embed error", func() {
			mockErr := fmt.Errorf("mock err")
			req, err := r.makeQueryRequest(ctx, query, &retriever.Options{
				TopK:      of(10),
				Embedding: &mockEmbedding{err: mockErr},
			}, &options{
				DenseVector:    nil,
				SparseVector:   map[int]float64{1: 1.2},
				MetadataFilter: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeQueryRequest] embed failed, %w", mockErr))
			convey.So(req, convey.ShouldBeNil)
		})

		PatchConvey("test vector size invalid", func() {
			req, err := r.makeQueryRequest(ctx, query, &retriever.Options{
				TopK:      of(10),
				Embedding: &mockEmbedding{sizeForCall: []int{2}},
			}, &options{
				DenseVector:    nil,
				SparseVector:   map[int]float64{1: 1.2},
				MetadataFilter: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeQueryRequest] invalid return length of vector, got=2, expected=1"))
			convey.So(req, convey.ShouldBeNil)
		})

		PatchConvey("test success with embedding", func() {
			req, err := r.makeQueryRequest(ctx, query, &retriever.Options{
				TopK:      of(10),
				Embedding: &mockEmbedding{sizeForCall: []int{1}, dims: 10},
			}, &options{
				DenseVector:    nil,
				SparseVector:   map[int]float64{1: 1.2},
				MetadataFilter: map[string]interface{}{"asd": 123},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
		})

		PatchConvey("test success with dense vector", func() {
			req, err := r.makeQueryRequest(ctx, query, &retriever.Options{
				TopK: of(10),
			}, &options{
				DenseVector:    []float64{1.1, 1.2},
				SparseVector:   map[int]float64{1: 1.2},
				MetadataFilter: map[string]interface{}{"asd": 123},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(req, convey.ShouldNotBeNil)
		})
	})
}

func TestDefaultScoredVectorToDocument(t *testing.T) {
	PatchConvey("test defaultScoredVectorToDocument", t, func() {
		exp := &schema.Document{
			ID:      "test_id",
			Content: "test_content",
			MetaData: map[string]any{
				defaultContentKey: "test_content",
			},
		}
		exp.WithScore(2.1).
			WithDenseVector([]float64{1.1, 1.2}).
			WithSparseVector(map[int]float64{4: 9.8, 5: 7.5})

		sv := &pinecone.ScoredVector{
			Vector: &pinecone.Vector{
				Id:     "test_id",
				Values: []float32{1.1, 1.2},
				SparseValues: &pinecone.SparseValues{
					Indices: []uint32{4, 5},
					Values:  []float32{9.8, 7.5},
				},
				Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
					defaultContentKey: structpb.NewStringValue("test_content"),
				}},
			},
			Score: 2.1,
		}

		got, err := defaultScoredVectorToDocument(context.Background(), sv)
		convey.So(err, convey.ShouldBeNil)
		convey.So(got.ID, convey.ShouldEqual, exp.ID)
		convey.So(got.Content, convey.ShouldEqual, exp.Content)
		convey.So(got.MetaData[defaultContentKey], convey.ShouldEqual, exp.MetaData[defaultContentKey])
		convey.So(got.Score(), convey.ShouldAlmostEqual, exp.Score(), 0.01)
		for i, gd := range got.DenseVector() {
			convey.So(gd, convey.ShouldAlmostEqual, exp.DenseVector()[i], 0.01)
		}
		for k, v := range got.SparseVector() {
			convey.So(v, convey.ShouldAlmostEqual, exp.SparseVector()[k], 0.01)
		}
	})

}

func of[T any](t T) *T {
	return &t
}

type mockEmbedding struct {
	err         error
	cnt         int
	sizeForCall []int
	dims        int
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.cnt > len(m.sizeForCall) {
		panic("unexpected")
	}

	if m.err != nil {
		return nil, m.err
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
