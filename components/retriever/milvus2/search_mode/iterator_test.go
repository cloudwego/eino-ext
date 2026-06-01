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

package search_mode

import (
	"context"
	"fmt"
	"io"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

func TestNewIterator(t *testing.T) {
	convey.Convey("test NewIterator", t, func() {
		convey.Convey("test with default values", func() {
			iter := NewIterator("", 0)
			convey.So(iter, convey.ShouldNotBeNil)
			convey.So(iter.MetricType, convey.ShouldEqual, milvus2.L2)
			convey.So(iter.BatchSize, convey.ShouldEqual, 100)
		})

		convey.Convey("test with custom metric type", func() {
			iter := NewIterator(milvus2.IP, 0)
			convey.So(iter, convey.ShouldNotBeNil)
			convey.So(iter.MetricType, convey.ShouldEqual, milvus2.IP)
		})

		convey.Convey("test with custom batch size", func() {
			iter := NewIterator(milvus2.L2, 50)
			convey.So(iter, convey.ShouldNotBeNil)
			convey.So(iter.BatchSize, convey.ShouldEqual, 50)
		})

		convey.Convey("test with both custom values", func() {
			iter := NewIterator(milvus2.COSINE, 200)
			convey.So(iter, convey.ShouldNotBeNil)
			convey.So(iter.MetricType, convey.ShouldEqual, milvus2.COSINE)
			convey.So(iter.BatchSize, convey.ShouldEqual, 200)
		})
	})
}

func TestIterator_WithSearchParams(t *testing.T) {
	convey.Convey("test Iterator.WithSearchParams", t, func() {
		iter := NewIterator(milvus2.L2, 100)
		params := map[string]string{
			"nprobe": "16",
			"ef":     "64",
		}
		result := iter.WithSearchParams(params)
		convey.So(result, convey.ShouldEqual, iter)
		convey.So(iter.SearchParams, convey.ShouldResemble, params)
	})
}

func TestIterator_BuildSearchOption(t *testing.T) {
	convey.Convey("test Iterator.BuildSearchOption returns error", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}
		config := &milvus2.RetrieverConfig{
			Collection:  "test_collection",
			VectorField: "vector",
			TopK:        10,
		}

		iter := NewIterator(milvus2.L2, 100)

		opt, err := iter.BuildSearchOption(ctx, config, queryVector)
		convey.So(opt, convey.ShouldBeNil)
		convey.So(err, convey.ShouldNotBeNil)
		convey.So(err.Error(), convey.ShouldContainSubstring, "BuildSearchIteratorOption")
	})
}

func TestIterator_BuildSearchIteratorOption(t *testing.T) {
	convey.Convey("test Iterator.BuildSearchIteratorOption", t, func() {
		ctx := context.Background()
		queryVector := []float32{0.1, 0.2, 0.3}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		convey.Convey("test basic iterator option", func() {
			iter := NewIterator(milvus2.L2, 100)
			opt, err := iter.BuildSearchIteratorOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with partitions", func() {
			configWithPartitions := &milvus2.RetrieverConfig{
				Collection:   "test_collection",
				VectorField:  "vector",
				TopK:         10,
				OutputFields: []string{"id", "content"},
				Partitions:   []string{"partition1", "partition2"},
			}
			iter := NewIterator(milvus2.L2, 100)
			opt, err := iter.BuildSearchIteratorOption(ctx, configWithPartitions, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with search params", func() {
			iter := NewIterator(milvus2.L2, 100).WithSearchParams(map[string]string{
				"nprobe": "16",
				"ef":     "64",
			})
			opt, err := iter.BuildSearchIteratorOption(ctx, config, queryVector)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with filter option", func() {
			iter := NewIterator(milvus2.L2, 100)
			opt, err := iter.BuildSearchIteratorOption(ctx, config, queryVector,
				milvus2.WithFilter("id > 10"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with grouping option", func() {
			iter := NewIterator(milvus2.L2, 100)
			opt, err := iter.BuildSearchIteratorOption(ctx, config, queryVector,
				milvus2.WithGrouping("category", 3, true))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom TopK from options", func() {
			iter := NewIterator(milvus2.L2, 100)
			customTopK := 50
			opt, err := iter.BuildSearchIteratorOption(ctx, config, queryVector,
				retriever.WithTopK(customTopK))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

// Verify interface implementation
func TestIterator_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Iterator implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Iterator)(nil)
	})
}

func TestIterator_Retrieve(t *testing.T) {
	PatchConvey("test Iterator.Retrieve", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}
		mockEmb := &mockIterEmbedding{}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
			Embedding:    mockEmb,
		}

		iter := NewIterator(milvus2.L2, 10)

		PatchConvey("success", func() {
			mockIt := &mockSearchIterator{
				nextFunc: func(ctx context.Context) (milvusclient.ResultSet, error) {
					return milvusclient.ResultSet{ResultCount: 1}, nil
				},
			}
			// Use counter to return result then EOF
			called := 0
			mockIt.nextFunc = func(ctx context.Context) (milvusclient.ResultSet, error) {
				if called == 0 {
					called++
					return milvusclient.ResultSet{ResultCount: 1}, nil
				}
				return milvusclient.ResultSet{}, io.EOF
			}

			Mock(GetMethod(mockClient, "SearchIterator")).Return(mockIt, nil).Build()

			mockConverter := func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
				return []*schema.Document{{ID: "1"}}, nil
			}
			config.DocumentConverter = mockConverter

			docs, err := iter.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
		})

		PatchConvey("embedding error", func() {
			mockEmb.err = fmt.Errorf("embed error")
			docs, err := iter.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})

		PatchConvey("SearchIterator creation error", func() {
			mockEmb.err = nil
			Mock(GetMethod(mockClient, "SearchIterator")).Return(nil, fmt.Errorf("create iter error")).Build()

			docs, err := iter.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}

type mockSearchIterator struct {
	nextFunc func(ctx context.Context) (milvusclient.ResultSet, error)
}

func (m *mockSearchIterator) Next(ctx context.Context) (milvusclient.ResultSet, error) {
	if m.nextFunc != nil {
		return m.nextFunc(ctx)
	}
	return milvusclient.ResultSet{}, io.EOF
}

// mockIterEmbedding implements embedding.Embedder for testing
type mockIterEmbedding struct {
	err error
}

func (m *mockIterEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return [][]float64{make([]float64, 128)}, nil
}
