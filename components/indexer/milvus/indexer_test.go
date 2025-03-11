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

package milvus

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/smartystreets/goconvey/convey"
)

// 模拟Embedding实现
type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

func TestNewIndexer(t *testing.T) {
	PatchConvey("test NewIndexer", t, func() {
		ctx := context.Background()
		Mock(client.NewClient).Return(&client.GrpcClient{}, nil).Build()
		mockClient, _ := client.NewClient(ctx, client.Config{})
		mockEmb := &mockEmbedding{}

		PatchConvey("test indexer config check", func() {
			PatchConvey("test client not provided", func() {
				i, err := NewIndexer(ctx, &IndexerConfig{
					Client:     nil,
					Collection: "",
					Embedding:  mockEmb,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] milvus client not provided"))
				convey.So(i, convey.ShouldBeNil)
			})

			PatchConvey("test embedding not provided", func() {
				i, err := NewIndexer(ctx, &IndexerConfig{
					Client:     mockClient,
					Collection: "",
					Embedding:  nil,
				})
				convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] embedding not provided"))
				convey.So(i, convey.ShouldBeNil)
			})
		})

		PatchConvey("test pre-check", func() {
			Mock(GetMethod(mockClient, "HasCollection")).To(func(ctx context.Context, collName string) (bool, error) {
				if collName != defaultCollection {
					return false, nil
				}
				return true, nil
			}).Build()

			PatchConvey("test collection not found", func() {
				// 模拟创建集合
				Mock(GetMethod(mockClient, "CreateCollection")).Return(nil).Build()

				// 模拟描述集合
				Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
					if collName != defaultCollection {
						return nil, fmt.Errorf("collection not found")
					}
					return &entity.Collection{
						Schema: &entity.Schema{
							Fields: getDefaultFields(),
						},
						Loaded: true,
					}, nil
				}).Build()

				i, err := NewIndexer(ctx, &IndexerConfig{
					Client:     mockClient,
					Collection: "test_collection",
					Embedding:  mockEmb,
				})
				convey.So(err, convey.ShouldBeError)
				convey.So(i, convey.ShouldBeNil)

				PatchConvey("test collection schema check", func() {
					// 模拟集合已存在但schema不匹配
					Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
						return &entity.Collection{
							Schema: &entity.Schema{
								Fields: []*entity.Field{
									{
										Name:     "different_field",
										DataType: entity.FieldTypeInt64,
									},
								},
							},
							Loaded: true,
						}, nil
					}).Build()

					i, err := NewIndexer(ctx, &IndexerConfig{
						Client:     mockClient,
						Collection: defaultCollection,
						Fields:     getDefaultFields(),
						Embedding:  mockEmb,
					})
					convey.So(err, convey.ShouldBeError, fmt.Errorf("[NewIndexer] collection schema not match"))
					convey.So(i, convey.ShouldBeNil)
				})

				PatchConvey("test collection not loaded", func() {
					// 模拟集合未加载
					Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
						return &entity.Collection{
							Schema: &entity.Schema{
								Fields: getDefaultFields(),
							},
							Loaded: false,
						}, nil
					}).Build()

					// 模拟获取加载状态
					Mock(GetMethod(mockClient, "GetLoadState")).Return(entity.LoadStateNotLoad, nil).Build()

					// 模拟描述索引
					Mock(GetMethod(mockClient, "DescribeIndex")).Return([]entity.Index{
						entity.NewGenericIndex("vector", entity.AUTOINDEX, nil),
					}, nil).Build()

					// 模拟创建索引
					Mock(GetMethod(mockClient, "CreateIndex")).Return(nil).Build()

					// 模拟加载集合
					Mock(GetMethod(mockClient, "LoadCollection")).Return(nil).Build()

					i, err := NewIndexer(ctx, &IndexerConfig{
						Client:     mockClient,
						Collection: defaultCollection,
						Embedding:  mockEmb,
					})
					convey.So(err, convey.ShouldBeNil)
					convey.So(i, convey.ShouldNotBeNil)
				})

				PatchConvey("test create indexer with custom config", func() {
					// 模拟集合已加载
					Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
						return &entity.Collection{
							Schema: &entity.Schema{
								Fields: getDefaultFields(),
							},
							Loaded: true,
						}, nil
					}).Build()

					i, err := NewIndexer(ctx, &IndexerConfig{
						Client:              mockClient,
						Collection:          defaultCollection,
						Description:         "custom description",
						PartitionNum:        0,
						Fields:              getDefaultFields(),
						SharedNum:           1,
						ConsistencyLevel:    defaultConsistencyLevel,
						MetricType:          defaultMetricType,
						Embedding:           mockEmb,
						EnableDynamicSchema: true,
					})
					convey.So(err, convey.ShouldBeNil)
					convey.So(i, convey.ShouldNotBeNil)
				})
			})
		})
	})
}

func TestIndexer_Store(t *testing.T) {
	PatchConvey("test Indexer.Store", t, func() {
		ctx := context.Background()
		Mock(client.NewClient).Return(&client.GrpcClient{}, nil).Build()
		mockClient, _ := client.NewClient(ctx, client.Config{})

		// 模拟集合已加载
		Mock(GetMethod(mockClient, "DescribeCollection")).To(func(ctx context.Context, collName string) (*entity.Collection, error) {
			return &entity.Collection{
				Schema: &entity.Schema{
					Fields: getDefaultFields(),
				},
				Loaded: true,
			}, nil
		}).Build()

		// 模拟HasCollection
		Mock(GetMethod(mockClient, "HasCollection")).Return(true, nil).Build()

		// 创建测试文档
		docs := []*schema.Document{
			{
				ID:       "doc1",
				Content:  "This is a test document",
				MetaData: map[string]interface{}{"key": "value"},
			},
			{
				ID:       "doc2",
				Content:  "This is another test document",
				MetaData: map[string]interface{}{"key2": "value2"},
			},
		}

		PatchConvey("test store with document converter error", func() {
			// 创建带有错误的文档转换器的索引器
			mockEmb := &mockEmbedding{}
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:     mockClient,
				Collection: defaultCollection,
				Embedding:  mockEmb,
				DocumentConverter: func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
					return nil, fmt.Errorf("document converter error")
				},
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			// 测试文档转换器错误的情况
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[Indexer.Store] failed to convert documents: document converter error"))
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store with insert rows error", func() {
			// 模拟InsertRows错误
			Mock(GetMethod(mockClient, "InsertRows")).Return(nil, fmt.Errorf("insert rows error")).Build()

			// 创建索引器
			mockEmb := &mockEmbedding{}
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:     mockClient,
				Collection: defaultCollection,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			// 测试插入行错误的情况
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[Indexer.Store] failed to insert rows: insert rows error"))
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store with flush error", func() {
			// 模拟InsertRows成功
			mockIDs := entity.NewColumnVarChar("id", []string{"doc1", "doc2"})
			Mock(GetMethod(mockClient, "InsertRows")).Return(mockIDs, nil).Build()

			// 模拟Flush错误
			Mock(GetMethod(mockClient, "Flush")).Return(fmt.Errorf("flush error")).Build()

			// 创建索引器
			mockEmb := &mockEmbedding{}
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:     mockClient,
				Collection: defaultCollection,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			// 测试刷新错误的情况
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeError, fmt.Errorf("[Indexer.Store] failed to flush collection: flush error"))
			convey.So(ids, convey.ShouldBeNil)
		})

		PatchConvey("test store success", func() {
			// 模拟InsertRows成功
			mockIDs := entity.NewColumnVarChar("id", []string{"doc1", "doc2"})
			Mock(GetMethod(mockClient, "InsertRows")).Return(mockIDs, nil).Build()

			// 模拟Flush成功
			Mock(GetMethod(mockClient, "Flush")).Return(nil).Build()

			// 创建索引器
			mockEmb := &mockEmbedding{}
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:     mockClient,
				Collection: defaultCollection,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			// 测试成功存储的情况
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
			convey.So(ids[0], convey.ShouldEqual, "doc1")
			convey.So(ids[1], convey.ShouldEqual, "doc2")
		})

		PatchConvey("test store with custom embedding", func() {
			// 模拟InsertRows成功
			mockIDs := entity.NewColumnVarChar("id", []string{"doc1", "doc2"})
			Mock(GetMethod(mockClient, "InsertRows")).Return(mockIDs, nil).Build()

			// 模拟Flush成功
			Mock(GetMethod(mockClient, "Flush")).Return(nil).Build()

			// 创建索引器
			mockEmb := &mockEmbedding{}
			indexer, err := NewIndexer(ctx, &IndexerConfig{
				Client:     mockClient,
				Collection: defaultCollection,
				Embedding:  mockEmb,
			})
			convey.So(err, convey.ShouldBeNil)
			convey.So(indexer, convey.ShouldNotBeNil)

			// 测试使用自定义embedding的情况
			ids, err := indexer.Store(ctx, docs)
			convey.So(err, convey.ShouldBeNil)
			convey.So(ids, convey.ShouldNotBeNil)
			convey.So(len(ids), convey.ShouldEqual, 2)
		})
	})
}
