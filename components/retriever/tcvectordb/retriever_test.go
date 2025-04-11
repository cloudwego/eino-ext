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

package tcvectordb

import (
	"context"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/smartystreets/goconvey/convey"
	"github.com/tencent/vectordatabase-sdk-go/tcvectordb"
)

func TestNewRetriever(t *testing.T) {
	PatchConvey("test NewRetriever", t, func() {
		ctx := context.Background()

		PatchConvey("test embedding set error", func() {
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: &mockEmbedding{fn: func() ([][]float64, error) {
					return [][]float64{{1.1, 1.2}}, nil
				}}},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)

			ret, err = NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: false, Embedding: nil},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test new rpc client error", func() {
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: nil},
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(ret, convey.ShouldBeNil)
		})

		PatchConvey("test find database error", func() {
			client := tcvectordb.RpcClient{}

			Mock(tcvectordb.NewRpcClient).Return(client, nil)
			ret, err := NewRetriever(ctx, &RetrieverConfig{
				Url:             "testurl",
				Username:        "testusername",
				Key:             "testkey",
				EmbeddingConfig: EmbeddingConfig{UseBuiltin: true, Embedding: nil},
			})

			convey.So(err, convey.ShouldNotBeNil)
			convey.ShouldAlmostEqual(err.Error(), "[TcVectorDBRetriever] find or create db failed")
			convey.So(ret, convey.ShouldBeNil)
		})
	})
}

func TestRetrieve(t *testing.T) {
	PatchConvey("test Retrieve", t, func() {
		ctx := context.Background()

		PatchConvey("test retrieve documents successfully", func() {
			// 创建一个mock的embedding函数
			mockEmb := &mockEmbedding{
				fn: func() ([][]float64, error) {
					return [][]float64{{1.1, 1.2}}, nil
				},
			}

			// 创建一个mock的collection
			collection := &tcvectordb.Collection{}

			// Mock Search方法返回结果
			Mock(collection.Search).Return(&tcvectordb.SearchDocumentResult{
				Documents: [][]tcvectordb.Document{
					{
						{
							Id: "1",
							Fields: map[string]tcvectordb.Field{
								"text": {Val: "i'm fine, thank you"},
								"age":  {Val: 25},
							},
							Score: 0.9,
						},
					},
				},
			}, nil)

			// 创建Retriever实例
			r := &Retriever{
				collection: collection,
				config: &RetrieverConfig{
					TopK: 10,
					EmbeddingConfig: EmbeddingConfig{
						UseBuiltin: false,
						Embedding:  mockEmb,
					},
				},
			}

			// 执行检索
			docs, err := r.Retrieve(ctx, "how are you")
			convey.So(err, convey.ShouldBeNil)
			convey.So(docs, convey.ShouldNotBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
			convey.So(docs[0].Content, convey.ShouldEqual, "i'm fine, thank you")
			convey.So(docs[0].ID, convey.ShouldEqual, "1")
			convey.So(docs[0].MetaData["age"], convey.ShouldEqual, 25)
		})

		PatchConvey("test retrieve with embedding error", func() {
			// 创建一个返回错误的mock embedding
			mockEmb := &mockEmbedding{
				fn: func() ([][]float64, error) {
					return nil, fmt.Errorf("embedding error")
				},
			}

			r := &Retriever{
				config: &RetrieverConfig{
					TopK: 10,
					EmbeddingConfig: EmbeddingConfig{
						UseBuiltin: false,
						Embedding:  mockEmb,
					},
				},
			}

			docs, err := r.Retrieve(ctx, "how are you")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "embed query failed")
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("test retrieve with search error", func() {
			mockEmb := &mockEmbedding{
				fn: func() ([][]float64, error) {
					return [][]float64{{1.1, 1.2}}, nil
				},
			}

			collection := &tcvectordb.Collection{}
			Mock(collection.Search).Return(nil, fmt.Errorf("search error"))

			r := &Retriever{
				collection: collection,
				config: &RetrieverConfig{
					TopK: 10,
					EmbeddingConfig: EmbeddingConfig{
						UseBuiltin: false,
						Embedding:  mockEmb,
					},
				},
			}

			docs, err := r.Retrieve(ctx, "how are you")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "search failed")
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

type mockEmbedding struct {
	fn func() ([][]float64, error)
}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.fn()
}

func (m *mockEmbedding) GetType() string {
	return "asd"
}
