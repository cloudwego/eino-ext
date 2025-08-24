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
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/smartystreets/goconvey/convey"
)

func TestDefaultDocumentConverter(t *testing.T) {
	convey.Convey("test defaultDocumentConverter", t, func() {
		convey.Convey("successful conversion", func() {
			docs := []*schema.Document{
				{
					ID:      "doc1",
					Content: "test content 1",
					MetaData: map[string]any{
						"key1": "value1",
						"key2": 123,
					},
				},
				{
					ID:      "doc2",
					Content: "test content 2",
					MetaData: nil,
				},
			}
			vectors := [][]float64{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
			}
			dim := 3

			columns, err := defaultDocumentConverter(docs, dim, vectors)

			convey.So(err, convey.ShouldBeNil)
			convey.So(columns, convey.ShouldHaveLength, 4)
			
			// Verify column names and types
			convey.So(columns[0].Name(), convey.ShouldEqual, "doc_id")
			convey.So(columns[1].Name(), convey.ShouldEqual, "content")
			convey.So(columns[2].Name(), convey.ShouldEqual, "vector")
			convey.So(columns[3].Name(), convey.ShouldEqual, "metadata")
		})

		convey.Convey("empty documents", func() {
			docs := []*schema.Document{}
			vectors := [][]float64{}
			dim := 3

			columns, err := defaultDocumentConverter(docs, dim, vectors)

			convey.So(err, convey.ShouldBeNil)
			convey.So(columns, convey.ShouldHaveLength, 4)
		})

		convey.Convey("documents with invalid metadata", func() {
			docs := []*schema.Document{
				{
					ID:      "doc1",
					Content: "test content",
					MetaData: map[string]any{
						"invalid": make(chan int), // Non-serializable type
					},
				},
			}
			vectors := [][]float64{{0.1, 0.2, 0.3}}
			dim := 3

			columns, err := defaultDocumentConverter(docs, dim, vectors)

			convey.So(err, convey.ShouldNotBeNil)
			convey.So(columns, convey.ShouldBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "failed to marshal metadata")
		})

		convey.Convey("mismatched vectors and documents length", func() {
			docs := []*schema.Document{
				{ID: "doc1", Content: "content1"},
				{ID: "doc2", Content: "content2"},
			}
			vectors := [][]float64{{0.1, 0.2, 0.3}} // Only one vector
			dim := 3

			columns, err := defaultDocumentConverter(docs, dim, vectors)

			// Should succeed, but vector array will have different length
			convey.So(err, convey.ShouldBeNil)
			convey.So(columns, convey.ShouldHaveLength, 4)
		})

		convey.Convey("zero dimension", func() {
			docs := []*schema.Document{
				{ID: "doc1", Content: "content1"},
			}
			vectors := [][]float64{{}}
			dim := 0

			columns, err := defaultDocumentConverter(docs, dim, vectors)

			convey.So(err, convey.ShouldBeNil)
			convey.So(columns, convey.ShouldHaveLength, 4)
		})
	})
}

func TestDocumentConverter_Interface(t *testing.T) {
	convey.Convey("test DocumentConverter interface", t, func() {
		// Test custom converter
		customConverter := func(docs []*schema.Document, dim int, vectors [][]float64) ([]column.Column, error) {
			return nil, nil
		}

		convey.So(customConverter, convey.ShouldNotBeNil)

		// Test that default converter conforms to interface
		var converter DocumentConverter = defaultDocumentConverter
		convey.So(converter, convey.ShouldNotBeNil)
	})
}