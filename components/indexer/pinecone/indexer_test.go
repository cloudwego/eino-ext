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
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()

		PatchConvey("test pinecone NewClient failed", func() {
			mockErr := fmt.Errorf("mock err")
			Mock(pinecone.NewClient).Return(nil, mockErr).Build()
			i, err := NewIndexer(ctx, &IndexerConfig{})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("pinecone: Failed to create Client: %w", mockErr))
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test DescribeIndex failed", func() {
			mockErr := fmt.Errorf("mock err")
			pc := &pinecone.Client{}
			Mock(pinecone.NewClient).Return(pc, nil).Build()
			Mock(GetMethod(pc, "DescribeIndex")).Return(nil, mockErr).Build()
			i, err := NewIndexer(ctx, &IndexerConfig{IndexName: "mock_index"})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("pinecone: Failed to describe index mock_index: %w", mockErr))
			convey.So(i, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			pc := &pinecone.Client{}
			idx := &pinecone.Index{}
			Mock(pinecone.NewClient).Return(pc, nil).Build()
			Mock(GetMethod(pc, "DescribeIndex")).Return(idx, nil).Build()
			Mock(GetMethod(pc, "Index")).Return(&pinecone.IndexConnection{}, nil).Build()
			i, err := NewIndexer(ctx, &IndexerConfig{IndexName: "mock_index"})
			convey.So(err, convey.ShouldBeNil)
			convey.So(i, convey.ShouldNotBeNil)
		})
	})
}

func TestMakeBatchRequest(t *testing.T) {
	PatchConvey("test makeBatchRequest", t, func() {
		ctx := context.Background()
		d1 := &schema.Document{ID: "1", Content: "asd"}
		d2 := &schema.Document{ID: "2", Content: "qwe", MetaData: map[string]any{
			"mock_field_1": map[string]any{"extra_field_1": "asd"},
			"mock_field_2": int64(123),
		}}
		d1.WithSparseVector(map[int]float64{123: 0.1})
		d2.WithSparseVector(map[int]float64{321: 0.2})
		docs := []*schema.Document{d1, d2}

		PatchConvey("test DocumentToMetadata failed", func() {
			mockErr := fmt.Errorf("mock err")
			i := &Indexer{conf: &IndexerConfig{
				DocumentToMetadata: func(ctx context.Context, doc *schema.Document) (map[string]any, error) {
					return nil, mockErr
				},
			}}
			pvs, err := i.makeBatchRequest(ctx, docs, &indexer.Options{
				Embedding: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeBatchRequest] DocumentToMetadata failed, %w", mockErr))
			convey.So(pvs, convey.ShouldBeNil)
		})

		PatchConvey("test embedding not provided", func() {
			i := &Indexer{conf: &IndexerConfig{
				DocumentToMetadata: defaultDocumentToMetadata,
			}}
			pvs, err := i.makeBatchRequest(ctx, docs, &indexer.Options{
				Embedding: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeBatchRequest] embedding not provided from config"))
			convey.So(pvs, convey.ShouldBeNil)
		})

		PatchConvey("test embed error", func() {
			mockErr := fmt.Errorf("mock err")
			i := &Indexer{conf: &IndexerConfig{
				DocumentToMetadata: defaultDocumentToMetadata,
			}}
			pvs, err := i.makeBatchRequest(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{err: mockErr},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeBatchRequest] embed error, %w", mockErr))
			convey.So(pvs, convey.ShouldBeNil)
		})

		PatchConvey("test vector size invalid", func() {
			i := &Indexer{conf: &IndexerConfig{
				DocumentToMetadata: defaultDocumentToMetadata,
			}}
			pvs, err := i.makeBatchRequest(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{sizeForCall: []int{1}, dims: 10},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[makeBatchRequest] invalid return length of vector, got=1, expected=2"))
			convey.So(pvs, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			i := &Indexer{conf: &IndexerConfig{
				DocumentToMetadata: defaultDocumentToMetadata,
			}}
			pvs, err := i.makeBatchRequest(ctx, docs, &indexer.Options{
				Embedding: &mockEmbedding{sizeForCall: []int{2}, dims: 10},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(pvs), convey.ShouldEqual, 2)
		})
	})
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
