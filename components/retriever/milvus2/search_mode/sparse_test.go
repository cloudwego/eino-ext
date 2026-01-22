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
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

func TestNewSparse(t *testing.T) {
	convey.Convey("test NewSparse", t, func() {
		sparse := NewSparse(milvus2.BM25)
		convey.So(sparse, convey.ShouldNotBeNil)
		convey.So(sparse.MetricType, convey.ShouldEqual, milvus2.BM25)

		sparseDefault := NewSparse("")
		convey.So(sparseDefault, convey.ShouldNotBeNil)
		convey.So(sparseDefault.MetricType, convey.ShouldEqual, milvus2.BM25)
	})
}

func TestSparse_BuildSparseSearchOption(t *testing.T) {
	convey.Convey("test Sparse.BuildSparseSearchOption", t, func() {
		ctx := context.Background()
		config := &milvus2.RetrieverConfig{
			Collection:        "test_collection",
			VectorField:       "vector",
			SparseVectorField: "sparse_vector",
			TopK:              10,
		}

		convey.Convey("test with default options", func() {
			sparse := NewSparse(milvus2.BM25)
			opt, err := sparse.BuildSparseSearchOption(ctx, config, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with options", func() {
			sparse := NewSparse(milvus2.BM25)
			opt, err := sparse.BuildSparseSearchOption(ctx, config, "test query",
				retriever.WithTopK(20),
				milvus2.WithFilter("id > 0"),
				milvus2.WithGrouping("group", 5, true),
			)
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with ConsistencyLevel", func() {
			configWithConsistency := &milvus2.RetrieverConfig{
				Collection:        "test_collection",
				VectorField:       "vector",
				SparseVectorField: "sparse_vector",
				TopK:              10,
				ConsistencyLevel:  milvus2.ConsistencyLevelStrong,
			}
			sparse := NewSparse(milvus2.BM25)
			opt, err := sparse.BuildSparseSearchOption(ctx, configWithConsistency, "test query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})
	})
}

func TestSparse_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Sparse implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Sparse)(nil)
	})
}

func TestSparse_Retrieve(t *testing.T) {
	PatchConvey("test Sparse.Retrieve", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		config := &milvus2.RetrieverConfig{
			Collection:        "test_collection",
			SparseVectorField: "sparse",
			TopK:              10,
			OutputFields:      []string{"id", "content"},
		}

		sparse := NewSparse(milvus2.BM25)

		PatchConvey("success", func() {
			Mock(GetMethod(mockClient, "Search")).Return([]milvusclient.ResultSet{
				{
					ResultCount: 1,
				},
			}, nil).Build()

			mockConverter := func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
				return []*schema.Document{{ID: "1"}}, nil
			}
			config.DocumentConverter = mockConverter

			docs, err := sparse.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
		})

		PatchConvey("search error", func() {
			Mock(GetMethod(mockClient, "Search")).Return(nil, fmt.Errorf("search error")).Build()

			docs, err := sparse.Retrieve(ctx, mockClient, config, "query")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}
