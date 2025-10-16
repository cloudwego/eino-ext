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

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/smartystreets/goconvey/convey"
)

func TestWithLimit(t *testing.T) {
	convey.Convey("TestWithLimit", t, func() {
		limitOpt := WithLimit(100)
		convey.So(limitOpt, convey.ShouldNotBeNil)
	})
}

func TestWithHybridSearchOption(t *testing.T) {
	convey.Convey("TestWithHybridSearchOption", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		hybridOpt := WithHybridSearchOption(hybridSearch)
		convey.So(hybridOpt, convey.ShouldNotBeNil)
	})
}

func TestNewHybridSearchOption(t *testing.T) {
	convey.Convey("TestNewHybridSearchOption", t, func() {
		hybridSearch := NewHybridSearchOption("test_field", 20)
		
		convey.So(hybridSearch, convey.ShouldNotBeNil)
		convey.So(hybridSearch.annField, convey.ShouldEqual, "test_field")
		convey.So(hybridSearch.topK, convey.ShouldEqual, 20)
		convey.So(hybridSearch.searchParam, convey.ShouldNotBeNil)
		convey.So(hybridSearch.templateParams, convey.ShouldNotBeNil)
	})
}

func TestHybridSearch_WithANNSField(t *testing.T) {
	convey.Convey("TestHybridSearch_WithANNSField", t, func() {
		hybridSearch := NewHybridSearchOption("original_field", 10)
		result := hybridSearch.WithANNSField("new_field")
		
		convey.So(result, convey.ShouldEqual, hybridSearch) // Returns itself
		convey.So(hybridSearch.annField, convey.ShouldEqual, "new_field")
	})
}

func TestHybridSearch_WithGroupByField(t *testing.T) {
	convey.Convey("TestHybridSearch_WithGroupByField", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithGroupByField("group_field")
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.groupByField, convey.ShouldEqual, "group_field")
	})
}

func TestHybridSearch_WithGroupSize(t *testing.T) {
	convey.Convey("TestHybridSearch_WithGroupSize", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithGroupSize(5)
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.groupSize, convey.ShouldEqual, 5)
	})
}

func TestHybridSearch_WithStrictGroupSize(t *testing.T) {
	convey.Convey("TestHybridSearch_WithStrictGroupSize", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithStrictGroupSize(true)
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.strictGroupSize, convey.ShouldEqual, true)
	})
}

func TestHybridSearch_WithSearchParam(t *testing.T) {
	convey.Convey("TestHybridSearch_WithSearchParam", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithSearchParam("nprobe", "16")
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.searchParam["nprobe"], convey.ShouldEqual, "16")
	})
}

func TestHybridSearch_WithAnnParam(t *testing.T) {
	convey.Convey("TestHybridSearch_WithAnnParam", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		// Create a simple AnnParam for testing
		var annParam index.AnnParam
		result := hybridSearch.WithAnnParam(annParam)
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.annParam, convey.ShouldEqual, annParam)
	})
}

func TestHybridSearch_WithFilter(t *testing.T) {
	convey.Convey("TestHybridSearch_WithFilter", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithFilter("id > 100")
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.expr, convey.ShouldEqual, "id > 100")
	})
}

func TestHybridSearch_WithTemplateParam(t *testing.T) {
	convey.Convey("TestHybridSearch_WithTemplateParam", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithTemplateParam("param1", "value1")
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.templateParams["param1"], convey.ShouldEqual, "value1")
	})
}

func TestHybridSearch_WithOffset(t *testing.T) {
	convey.Convey("TestHybridSearch_WithOffset", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithOffset(5)
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.offset, convey.ShouldEqual, 5)
	})
}

func TestHybridSearch_WithIgnoreGrowing(t *testing.T) {
	convey.Convey("TestHybridSearch_WithIgnoreGrowing", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 10)
		result := hybridSearch.WithIgnoreGrowing(true)
		
		convey.So(result, convey.ShouldEqual, hybridSearch)
		convey.So(hybridSearch.ignoreGrowing, convey.ShouldEqual, true)
	})
}

func TestHybridSearch_getAnnRequest(t *testing.T) {
	convey.Convey("TestHybridSearch_getAnnRequest", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 0) // topK is 0
		hybridSearch.WithFilter("id > 100")
		hybridSearch.WithOffset(5)
		hybridSearch.WithGroupByField("category")
		hybridSearch.WithGroupSize(3)
		hybridSearch.WithStrictGroupSize(true)
		hybridSearch.WithIgnoreGrowing(true)
		hybridSearch.WithSearchParam("nprobe", "16")
		hybridSearch.WithTemplateParam("param1", "value1")
		
		vectors := []entity.Vector{entity.FloatVector([]float32{1.0, 2.0, 3.0})}
		req := hybridSearch.getAnnRequest(10, vectors)
		
		convey.So(req, convey.ShouldNotBeNil)
		convey.So(hybridSearch.topK, convey.ShouldEqual, 10) // Should be set to limit
	})
	
	convey.Convey("TestHybridSearch_getAnnRequest with existing topK", t, func() {
		hybridSearch := NewHybridSearchOption("vector_field", 5) // topK is 5
		
		vectors := []entity.Vector{entity.FloatVector([]float32{1.0, 2.0, 3.0})}
		req := hybridSearch.getAnnRequest(10, vectors)
		
		convey.So(req, convey.ShouldNotBeNil)
		convey.So(hybridSearch.topK, convey.ShouldEqual, 5) // Should keep original value
	})
}