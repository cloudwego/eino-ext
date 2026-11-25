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

package pinecone

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(
	ctx context.Context,
	texts []string,
	opts ...embedding.Option,
) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3, 0.4}
	}
	return result, nil
}

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()
		Mock(pc.NewClient).Return(&pc.Client{}, nil).Build()

		mockClient, _ := pc.NewClient(pc.NewClientParams{})
		mockDim := int32(4)
		mockName := "test-store"
		mockIndex := &pc.Index{
			Name:      mockName,
			Metric:    defaultMetricType,
			Dimension: &mockDim,
			Tags: &pc.IndexTags{
				"mockTag": "mockValue",
			},
		}
		mockEmb := &mockEmbedding{}

		Mock(GetMethod(mockClient, "DescribeIndex")).
			To(func(ctx context.Context, idxName string) (*pc.Index, error) {
				if idxName == mockName {
					return mockIndex, nil
				} else {
					return nil, fmt.Errorf("mockDescribeIndex not found index: %s", idxName)
				}
			}).Build()

		PatchConvey("test retriever config check", func() {
			PatchConvey("test client not provided", func() {
				retriever, err := NewRetriever(ctx, &RetrieverConfig{})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] milvus client not provided"))
				convey.So(retriever, convey.ShouldBeNil)
			})

			PatchConvey("test embedding not provided", func() {
				retriever, err := NewRetriever(ctx, &RetrieverConfig{
					Client: mockClient,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] embedding not provided"))
				convey.So(retriever, convey.ShouldBeNil)
			})

			PatchConvey("test ScoreThreshold is illegal", func() {
				retriever, err := NewRetriever(ctx, &RetrieverConfig{
					Client:         mockClient,
					Embedding:      mockEmb,
					ScoreThreshold: -1,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] invalid search params"))
				convey.So(retriever, convey.ShouldBeNil)
			})
		})

		PatchConvey("test retriever index validate", func() {
			PatchConvey("test index metric type mismatch", func() {
				mockMetricType := pc.Euclidean
				retriever, err := NewRetriever(ctx, &RetrieverConfig{
					Client:     mockClient,
					Embedding:  mockEmb,
					IndexName:  mockName,
					MetricType: mockMetricType,
				})
				expectedErr := fmt.Errorf("[validate] index metric and config "+
					"metric mismatch, index: %s, config: %s", defaultMetricType, mockMetricType)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewRetriever] failed to validate index, "+
					"err: %w", expectedErr))
				convey.So(retriever, convey.ShouldBeNil)
			})
		})
	})
}

func TestRetriever_Retrieve(t *testing.T) {
	PatchConvey("test Retriever.Retrieve", t, func() {
		ctx := context.Background()
		Mock(pc.NewClient).Return(&pc.Client{}, nil).Build()

		mockClient, _ := pc.NewClient(pc.NewClientParams{})
		mockDim := int32(4)
		mockName := "test-store"
		mockIndex := &pc.Index{
			Name:      mockName,
			Metric:    defaultMetricType,
			Dimension: &mockDim,
			Tags: &pc.IndexTags{
				"mockTag": "mockValue",
			},
		}
		mockEmb := &mockEmbedding{}
		mockQuery := "pinecone"

		Mock(GetMethod(mockClient, "DescribeIndex")).
			To(func(ctx context.Context, idxName string) (*pc.Index, error) {
				if idxName == mockName {
					return mockIndex, nil
				} else {
					return nil, fmt.Errorf("mockDescribeIndex not found index: %s", idxName)
				}
			}).Build()

		retriever, err := NewRetriever(ctx, &RetrieverConfig{
			Client:    mockClient,
			Embedding: mockEmb,
			IndexName: mockName,
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(retriever, convey.ShouldNotBeNil)

		PatchConvey("test retriever check", func() {
			PatchConvey("test embedding is not provided", func() {
				retriever.config.Embedding = nil
				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] embedding not provided"))
				convey.So(docs, convey.ShouldBeEmpty)
			})

			PatchConvey("test describe index mismatch", func() {
				mockErr := fmt.Errorf("mockDescribeIndex not found index")
				Mock(GetMethod(mockClient, "DescribeIndex")).
					To(func(ctx context.Context, idxName string) (*pc.Index, error) {
						return nil, mockErr
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] "+
					"failed to describe index, err: %w", mockErr))
				convey.So(docs, convey.ShouldBeEmpty)
			})

			PatchConvey("test create index connection failed", func() {
				mockErr := fmt.Errorf("mock create index connection failed")
				Mock(GetMethod(mockClient, "Index")).
					To(func(in pc.NewIndexConnParams, dialOpts ...grpc.DialOption) (*pc.IndexConnection, error) {
						return nil, mockErr
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] "+
					"failed to create IndexConnection for Host: %w", mockErr))
				convey.So(docs, convey.ShouldBeEmpty)
			})
		})

		PatchConvey("test query pinecone", func() {
			mockIndexConn := &pc.IndexConnection{}
			Mock(GetMethod(mockClient, "Index")).
				To(func(in pc.NewIndexConnParams, dialOpts ...grpc.DialOption) (*pc.IndexConnection, error) {
					return mockIndexConn, nil
				}).Build()

			PatchConvey("test query failed", func() {
				mockErr := fmt.Errorf("mockQueryByVectorValues failed")
				Mock(GetMethod(mockIndexConn, "QueryByVectorValues")).
					To(func(ctx context.Context, in *pc.QueryByVectorValuesRequest) (*pc.QueryVectorsResponse, error) {
						return nil, mockErr
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] "+
					"error encountered when querying by vector: %w", mockErr))
				convey.So(docs, convey.ShouldBeEmpty)
			})

			PatchConvey("test search result number is empty", func() {
				mockSearchResp := &pc.QueryVectorsResponse{
					Matches: []*pc.ScoredVector{},
				}
				Mock(GetMethod(mockIndexConn, "QueryByVectorValues")).
					To(func(ctx context.Context, in *pc.QueryByVectorValuesRequest) (*pc.QueryVectorsResponse, error) {
						return mockSearchResp, nil
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] no results found"))
				convey.So(docs, convey.ShouldBeEmpty)
			})

			PatchConvey("test search result not contain content meta", func() {
				mockMeta, _ := structpb.NewStruct(map[string]any{})
				mockSearchResp := &pc.QueryVectorsResponse{
					Matches: []*pc.ScoredVector{
						&pc.ScoredVector{Vector: &pc.Vector{Id: "mockId1", Metadata: mockMeta}},
						&pc.ScoredVector{Vector: &pc.Vector{Id: "mockId2", Metadata: mockMeta}},
					},
				}
				Mock(GetMethod(mockIndexConn, "QueryByVectorValues")).
					To(func(ctx context.Context, in *pc.QueryByVectorValuesRequest) (*pc.QueryVectorsResponse, error) {
						return mockSearchResp, nil
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[pinecone retriever] "+
					"failed to convert search result to schema.Document: "+
					"[converter] content field not found, field: %s", defaultField))
				convey.So(docs, convey.ShouldBeEmpty)
			})

			PatchConvey("test search success", func() {
				mockMeta, _ := structpb.NewStruct(map[string]any{
					defaultField: "mockContent",
				})
				mockSearchResp := &pc.QueryVectorsResponse{
					Matches: []*pc.ScoredVector{
						&pc.ScoredVector{Vector: &pc.Vector{Id: "mockId1", Metadata: mockMeta}},
						&pc.ScoredVector{Vector: &pc.Vector{Id: "mockId2", Metadata: mockMeta}},
					},
				}
				mockResult := []*schema.Document{
					&schema.Document{ID: "mockId1", Content: "mockContent", MetaData: make(map[string]any)},
					&schema.Document{ID: "mockId2", Content: "mockContent", MetaData: make(map[string]any)},
				}
				Mock(GetMethod(mockIndexConn, "QueryByVectorValues")).
					To(func(ctx context.Context, in *pc.QueryByVectorValuesRequest) (*pc.QueryVectorsResponse, error) {
						return mockSearchResp, nil
					}).Build()

				docs, err := retriever.Retrieve(ctx, mockQuery)
				convey.So(err, convey.ShouldBeNil)
				convey.So(docs, convey.ShouldEqual, mockResult)
			})
		})
	})
}
