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

func TestNewScalar(t *testing.T) {
	convey.Convey("test NewScalar", t, func() {
		scalar := NewScalar()
		convey.So(scalar, convey.ShouldNotBeNil)
	})
}

func TestScalar_BuildQueryOption(t *testing.T) {
	convey.Convey("test Scalar.BuildQueryOption", t, func() {
		ctx := context.Background()

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		convey.Convey("test with query expression", func() {
			scalar := NewScalar()
			opt, err := scalar.BuildQueryOption(ctx, config, "id > 10")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with empty query", func() {
			scalar := NewScalar()
			opt, err := scalar.BuildQueryOption(ctx, config, "")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with query and additional filter", func() {
			scalar := NewScalar()
			opt, err := scalar.BuildQueryOption(ctx, config, "id > 10",
				milvus2.WithFilter("category == 'A'"))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with only filter (empty query)", func() {
			scalar := NewScalar()
			opt, err := scalar.BuildQueryOption(ctx, config, "",
				milvus2.WithFilter("status == 'active'"))
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
			scalar := NewScalar()
			opt, err := scalar.BuildQueryOption(ctx, configWithPartitions, "id > 10")
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

		convey.Convey("test with custom TopK", func() {
			scalar := NewScalar()
			customTopK := 50
			opt, err := scalar.BuildQueryOption(ctx, config, "id > 10",
				retriever.WithTopK(customTopK))
			convey.So(err, convey.ShouldBeNil)
			convey.So(opt, convey.ShouldNotBeNil)
		})

	})
}

// Verify interface implementation
func TestScalar_ImplementsSearchMode(t *testing.T) {
	convey.Convey("test Scalar implements SearchMode", t, func() {
		var _ milvus2.SearchMode = (*Scalar)(nil)
	})
}

func TestScalar_Retrieve(t *testing.T) {
	PatchConvey("test Scalar.Retrieve", t, func() {
		ctx := context.Background()
		mockClient := &milvusclient.Client{}

		config := &milvus2.RetrieverConfig{
			Collection:   "test_collection",
			VectorField:  "vector",
			TopK:         10,
			OutputFields: []string{"id", "content"},
		}

		scalar := NewScalar()

		PatchConvey("success", func() {
			Mock(GetMethod(mockClient, "Query")).Return(milvusclient.ResultSet{
				ResultCount: 1,
			}, nil).Build()

			mockConverter := func(ctx context.Context, result milvusclient.ResultSet) ([]*schema.Document, error) {
				return []*schema.Document{{ID: "1"}}, nil
			}
			config.DocumentConverter = mockConverter

			docs, err := scalar.Retrieve(ctx, mockClient, config, "id > 10")
			convey.So(err, convey.ShouldBeNil)
			convey.So(len(docs), convey.ShouldEqual, 1)
		})

		PatchConvey("query error", func() {
			Mock(GetMethod(mockClient, "Query")).Return(nil, fmt.Errorf("query error")).Build()

			docs, err := scalar.Retrieve(ctx, mockClient, config, "id > 10")
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(docs, convey.ShouldBeNil)
		})
	})
}
