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
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/smartystreets/goconvey/convey"
)

// Helper functions to create options for testing
func withRadius(radius float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Radius = &radius
	})
}

func withRangeFilter(rangeFilter float64) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.RangeFilter = &rangeFilter
	})
}

func TestSearchModeAuto(t *testing.T) {
	convey.Convey("test SearchModeAuto", t, func() {
		ctx := context.Background()

		convey.Convey("test nil config uses defaults", func() {
			mode := SearchModeAuto(nil)
			convey.So(mode, convey.ShouldNotBeNil)
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)

			sp, err := mode.BuildSearchParam(ctx)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})

		convey.Convey("test level out of range defaults to 1", func() {
			// Level too low
			mode := SearchModeAuto(&AutoConfig{Level: 0})
			convey.So(mode, convey.ShouldNotBeNil)

			// Level too high
			mode = SearchModeAuto(&AutoConfig{Level: 6})
			convey.So(mode, convey.ShouldNotBeNil)
		})

		convey.Convey("test valid level values", func() {
			for level := 1; level <= 5; level++ {
				mode := SearchModeAuto(&AutoConfig{Level: level, Metric: entity.L2})
				convey.So(mode, convey.ShouldNotBeNil)
				convey.So(mode.MetricType(), convey.ShouldEqual, entity.L2)
			}
		})

		convey.Convey("test empty metric defaults to COSINE", func() {
			mode := SearchModeAuto(&AutoConfig{Level: 1})
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)
		})

		convey.Convey("test BuildSearchParam success", func() {
			mode := SearchModeAuto(&AutoConfig{Level: 3, Metric: entity.IP})
			sp, err := mode.BuildSearchParam(ctx)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})

		convey.Convey("test BuildSearchParam with radius and range filter", func() {
			mode := SearchModeAuto(&AutoConfig{Level: 1, Metric: entity.L2})
			sp, err := mode.BuildSearchParam(ctx, withRadius(0.5), withRangeFilter(0.1))
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})
	})
}

func TestSearchModeFlat(t *testing.T) {
	convey.Convey("test SearchModeFlat", t, func() {
		ctx := context.Background()

		convey.Convey("test nil config uses defaults", func() {
			mode := SearchModeFlat(nil)
			convey.So(mode, convey.ShouldNotBeNil)
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)
		})

		convey.Convey("test empty metric defaults to COSINE", func() {
			mode := SearchModeFlat(&FlatConfig{})
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)
		})

		convey.Convey("test custom metric type", func() {
			mode := SearchModeFlat(&FlatConfig{Metric: entity.L2})
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.L2)
		})

		convey.Convey("test BuildSearchParam success", func() {
			mode := SearchModeFlat(&FlatConfig{Metric: entity.IP})
			sp, err := mode.BuildSearchParam(ctx)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})

		convey.Convey("test BuildSearchParam with radius and range filter", func() {
			mode := SearchModeFlat(&FlatConfig{Metric: entity.L2})
			sp, err := mode.BuildSearchParam(ctx, withRadius(0.8), withRangeFilter(0.2))
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})
	})
}

func TestSearchModeHNSW(t *testing.T) {
	convey.Convey("test SearchModeHNSW", t, func() {
		ctx := context.Background()

		convey.Convey("test nil config returns error", func() {
			mode, err := SearchModeHNSW(nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "config cannot be nil")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test ef too low", func() {
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 0})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "ef must be in range [1, 32768]")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test ef too high", func() {
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 32769})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "ef must be in range [1, 32768]")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test valid ef values", func() {
			// Min value
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 1, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode, convey.ShouldNotBeNil)

			// Max value
			mode, err = SearchModeHNSW(&HNSWConfig{Ef: 32768, Metric: entity.IP})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode, convey.ShouldNotBeNil)
		})

		convey.Convey("test empty metric defaults to COSINE", func() {
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 64})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)
		})

		convey.Convey("test BuildSearchParam success", func() {
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 64, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)

			sp, err := mode.BuildSearchParam(ctx)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})

		convey.Convey("test BuildSearchParam with radius and range filter", func() {
			mode, err := SearchModeHNSW(&HNSWConfig{Ef: 64, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)

			sp, err := mode.BuildSearchParam(ctx, withRadius(0.7), withRangeFilter(0.3))
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})
	})
}

func TestSearchModeIvfFlat(t *testing.T) {
	convey.Convey("test SearchModeIvfFlat", t, func() {
		ctx := context.Background()

		convey.Convey("test nil config returns error", func() {
			mode, err := SearchModeIvfFlat(nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "config cannot be nil")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test nprobe too low", func() {
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 0})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nprobe must be in range [1, 65536]")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test nprobe too high", func() {
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 65537})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nprobe must be in range [1, 65536]")
			convey.So(mode, convey.ShouldBeNil)
		})

		convey.Convey("test valid nprobe values", func() {
			// Min value
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 1, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode, convey.ShouldNotBeNil)

			// Max value
			mode, err = SearchModeIvfFlat(&IvfFlatConfig{NProbe: 65536, Metric: entity.IP})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode, convey.ShouldNotBeNil)
		})

		convey.Convey("test empty metric defaults to COSINE", func() {
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 16})
			convey.So(err, convey.ShouldBeNil)
			convey.So(mode.MetricType(), convey.ShouldEqual, entity.COSINE)
		})

		convey.Convey("test BuildSearchParam success", func() {
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 16, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)

			sp, err := mode.BuildSearchParam(ctx)
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})

		convey.Convey("test BuildSearchParam with radius and range filter", func() {
			mode, err := SearchModeIvfFlat(&IvfFlatConfig{NProbe: 16, Metric: entity.L2})
			convey.So(err, convey.ShouldBeNil)

			sp, err := mode.BuildSearchParam(ctx, withRadius(0.6), withRangeFilter(0.1))
			convey.So(err, convey.ShouldBeNil)
			convey.So(sp, convey.ShouldNotBeNil)
		})
	})
}

func TestMakeEmbeddingCtx(t *testing.T) {
	convey.Convey("test MakeEmbeddingCtx", t, func() {
		ctx := context.Background()

		convey.Convey("test creates context with nil embedder", func() {
			newCtx := MakeEmbeddingCtx(ctx, nil)
			convey.So(newCtx, convey.ShouldNotBeNil)
		})
	})
}
