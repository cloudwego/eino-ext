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

package milvus2

import (
	"testing"

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/smartystreets/goconvey/convey"
)

func TestRetrieverMetricType_toEntity(t *testing.T) {
	convey.Convey("test MetricType.toEntity", t, func() {
		convey.Convey("test L2", func() {
			mt := L2
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("L2"))
		})

		convey.Convey("test IP", func() {
			mt := IP
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("IP"))
		})

		convey.Convey("test COSINE", func() {
			mt := COSINE
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("COSINE"))
		})

		convey.Convey("test HAMMING", func() {
			mt := HAMMING
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("HAMMING"))
		})

		convey.Convey("test JACCARD", func() {
			mt := JACCARD
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("JACCARD"))
		})

		convey.Convey("test TANIMOTO", func() {
			mt := TANIMOTO
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("TANIMOTO"))
		})

		convey.Convey("test SUBSTRUCTURE", func() {
			mt := SUBSTRUCTURE
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("SUBSTRUCTURE"))
		})

		convey.Convey("test SUPERSTRUCTURE", func() {
			mt := SUPERSTRUCTURE
			result := mt.toEntity()
			convey.So(result, convey.ShouldEqual, entity.MetricType("SUPERSTRUCTURE"))
		})
	})
}

func TestRetrieverConsistencyLevel_ToEntity(t *testing.T) {
	convey.Convey("test ConsistencyLevel.ToEntity", t, func() {
		convey.Convey("test ConsistencyLevelStrong", func() {
			level := ConsistencyLevelStrong
			result := level.ToEntity()
			convey.So(result, convey.ShouldEqual, entity.ClStrong)
		})

		convey.Convey("test ConsistencyLevelSession", func() {
			level := ConsistencyLevelSession
			result := level.ToEntity()
			convey.So(result, convey.ShouldEqual, entity.ClSession)
		})

		convey.Convey("test ConsistencyLevelBounded", func() {
			level := ConsistencyLevelBounded
			result := level.ToEntity()
			convey.So(result, convey.ShouldEqual, entity.ClBounded)
		})

		convey.Convey("test ConsistencyLevelEventually", func() {
			level := ConsistencyLevelEventually
			result := level.ToEntity()
			convey.So(result, convey.ShouldEqual, entity.ClEventually)
		})
	})
}
