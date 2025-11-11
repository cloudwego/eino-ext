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

	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/smartystreets/goconvey/convey"
)

func TestGetDefaultVectorConverter(t *testing.T) {
	convey.Convey("test getDefaultVectorConverter", t, func() {
		converter := getDefaultVectorConverter()
		convey.So(converter, convey.ShouldNotBeNil)

		convey.Convey("test convert valid vectors", func() {
			vectors := [][]float64{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
			}
			result, err := converter(vectors)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 2)
		})

		convey.Convey("test convert empty vectors", func() {
			vectors := [][]float64{}
			result, err := converter(vectors)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 0)
		})

		convey.Convey("test convert nil vectors", func() {
			result, err := converter(nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 0)
		})
	})
}

func TestGetDefaultDocumentConverter(t *testing.T) {
	convey.Convey("test getDefaultDocumentConverter", t, func() {
		converter := getDefaultDocumentConverter()
		convey.So(converter, convey.ShouldNotBeNil)

		convey.Convey("test convert valid result sets", func() {
				// Create simple ResultSet for testing
				resultSets := []milvusclient.ResultSet{
					{
						ResultCount: 0, // Empty result set
					},
				}
			result, err := converter(resultSets)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 0)
		})

		convey.Convey("test convert empty result sets", func() {
			resultSets := []milvusclient.ResultSet{}
			result, err := converter(resultSets)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 0)
		})

		convey.Convey("test convert nil result sets", func() {
			result, err := converter(nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, 0)
		})


	})
}