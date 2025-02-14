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

package redis

import (
	"context"
	"fmt"
	"log"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()
		mockClient := &redis.Client{}

		PatchConvey("test embedding not provided", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:    mockClient,
				Index:     "asd",
				Embedding: nil,
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] embedding not provided for redis retriever"))
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test index not provided", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:    mockClient,
				Index:     "",
				Embedding: &mockEmbedding{},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] redis index not provided"))
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test redis client not provided", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:    nil,
				Index:     "asd",
				Embedding: &mockEmbedding{},
			})
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] redis client not provided"))
			convey.So(r, convey.ShouldBeNil)
		})

		PatchConvey("test success", func() {
			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:    mockClient,
				Index:     "asd",
				Embedding: &mockEmbedding{},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(r, convey.ShouldNotBeNil)
		})
	})
}

func TestRetrieve(t *testing.T) {
	PatchConvey("test Retrieve", t, func() {
		ctx := context.Background()
		mockClient := redis.NewClient(&redis.Options{Addr: "123"})
		expv := make([]float64, 10)
		for i := range expv {
			expv[i] = 1.1
		}
		d1 := &schema.Document{ID: "1", Content: "asd"}
		d1.WithDenseVector(expv)
		d2 := &schema.Document{ID: "2", Content: "qwe"}
		d2.WithDenseVector(expv)
		docs := []*schema.Document{d1, d2}

		PatchConvey("test Embedding not provided", func() {
			r := &Retriever{config: &RetrieverConfig{Embedding: nil}}
			resp, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[redis retriever] embedding not provided"))
			convey.So(resp, convey.ShouldBeNil)
		})

		PatchConvey("test Embedding error", func() {
			mockErr := fmt.Errorf("mock err")
			r := &Retriever{config: &RetrieverConfig{Embedding: &mockEmbedding{err: mockErr}}}
			resp, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeError, mockErr)
			convey.So(resp, convey.ShouldBeNil)
		})

		PatchConvey("test vector size invalid", func() {
			r := &Retriever{config: &RetrieverConfig{Embedding: &mockEmbedding{sizeForCall: []int{2}, dims: 10}}}
			resp, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[redis retriever] invalid return length of vector, got=2, expected=1"))
			convey.So(resp, convey.ShouldBeNil)
		})

		PatchConvey("test vector range query", func() {
			dis := 10.2
			//origin := mockClient.FTSearchWithArgs
			var (
				//cmd     *redis.FTSearchCmd
				mockCmd *redis.FTSearchCmd
			)

			//Mock(GetMethod(mockClient, "FTSearchWithArgs")).To(
			//	func(ctx context.Context, index string, query string, options *redis.FTSearchOptions) *redis.FTSearchCmd {
			//		cmd = origin(ctx, index, query, options)
			//		return mockCmd
			//	}).Origin(&origin).Build()

			Mock(GetMethod(mockClient, "FTSearchWithArgs")).Return(mockCmd).Build()
			Mock(GetMethod(mockCmd, "Result")).Return(redis.FTSearchResult{
				Total: 2,
				Docs: []redis.Document{
					{
						ID: "1",
						Fields: map[string]string{
							defaultReturnFieldContent:       d1.Content,
							defaultReturnFieldVectorContent: string(vector2Bytes(expv)),
						},
					},
					{
						ID: "2",
						Fields: map[string]string{
							defaultReturnFieldContent:       d2.Content,
							defaultReturnFieldVectorContent: string(vector2Bytes(expv)),
						},
					},
				},
			}, nil).Build()

			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:            mockClient,
				Index:             "test_index",
				DistanceThreshold: &dis,
				Embedding:         &mockEmbedding{sizeForCall: []int{1}, dims: 10},
			})
			convey.So(err, convey.ShouldBeNil)
			resp, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeNil)
			//s := "FT.SEARCH test_index @vector_content:[VECTOR_RANGE $distance_threshold $vector]=>{$yield_distance_as: distance} RETURN 2 content vector_content SORTBY distance ASC LIMIT 0 5"
			//convey.So(strings.HasPrefix(cmd.String(), s), convey.ShouldBeTrue)
			for i := range resp {
				got := resp[i]
				exp := docs[i]
				convey.So(got.ID, convey.ShouldEqual, exp.ID)
				convey.So(got.Content, convey.ShouldEqual, exp.Content)
				convey.So(len(got.DenseVector()), convey.ShouldEqual, len(exp.DenseVector()))
				for j, gf := range got.DenseVector() {
					convey.So(gf, convey.ShouldAlmostEqual, exp.DenseVector()[j], 0.01)
				}
			}
		})

		PatchConvey("test knn vector search", func() {
			//origin := mockClient.FTSearchWithArgs
			var (
				//cmd     *redis.FTSearchCmd
				mockCmd *redis.FTSearchCmd
			)

			//Mock(GetMethod(mockClient, "FTSearchWithArgs")).To(
			//	func(ctx context.Context, index string, query string, options *redis.FTSearchOptions) *redis.FTSearchCmd {
			//		cmd = origin(ctx, index, query, options)
			//		return mockCmd
			//	}).Origin(&origin).Build()

			Mock(GetMethod(mockClient, "FTSearchWithArgs")).Return(mockCmd).Build()
			Mock(GetMethod(mockCmd, "Result")).Return(redis.FTSearchResult{
				Total: 2,
				Docs: []redis.Document{
					{
						ID: "1",
						Fields: map[string]string{
							defaultReturnFieldContent:       d1.Content,
							defaultReturnFieldVectorContent: string(vector2Bytes(expv)),
						},
					},
					{
						ID: "2",
						Fields: map[string]string{
							defaultReturnFieldContent:       d2.Content,
							defaultReturnFieldVectorContent: string(vector2Bytes(expv)),
						},
					},
				},
			}, nil).Build()

			r, err := NewRetriever(ctx, &RetrieverConfig{
				Client:    mockClient,
				Index:     "test_index",
				Embedding: &mockEmbedding{sizeForCall: []int{1}, dims: 10},
			})
			convey.So(err, convey.ShouldBeNil)
			resp, err := r.Retrieve(ctx, "test_query")
			convey.So(err, convey.ShouldBeNil)
			//s := "FT.SEARCH test_index (*)=>[KNN 5 @vector_content $vector AS distance] RETURN 2 content vector_content SORTBY distance ASC LIMIT 0 5"
			//convey.So(strings.HasPrefix(cmd.String(), s), convey.ShouldBeTrue)
			for i := range resp {
				got := resp[i]
				exp := docs[i]
				convey.So(got.ID, convey.ShouldEqual, exp.ID)
				convey.So(got.Content, convey.ShouldEqual, exp.Content)
				convey.So(len(got.DenseVector()), convey.ShouldEqual, len(exp.DenseVector()))
				for j, gf := range got.DenseVector() {
					convey.So(gf, convey.ShouldAlmostEqual, exp.DenseVector()[j], 0.01)
				}
			}
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
		log.Fatal("unexpected")
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
