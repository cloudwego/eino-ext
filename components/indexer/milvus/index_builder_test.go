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

	"github.com/smartystreets/goconvey/convey"
)

func TestNewHNSWIndexBuilder(t *testing.T) {
	convey.Convey("test NewHNSWIndexBuilder", t, func() {
		convey.Convey("test valid parameters", func() {
			builder, err := NewHNSWIndexBuilder(16, 128)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.M, convey.ShouldEqual, 16)
			convey.So(builder.EfConstruction, convey.ShouldEqual, 128)
		})

		convey.Convey("test M too low", func() {
			builder, err := NewHNSWIndexBuilder(3, 128)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "M must be in range [4, 64]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test M too high", func() {
			builder, err := NewHNSWIndexBuilder(65, 128)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "M must be in range [4, 64]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test efConstruction too low", func() {
			builder, err := NewHNSWIndexBuilder(16, 7)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "efConstruction must be in range [8, 512]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test efConstruction too high", func() {
			builder, err := NewHNSWIndexBuilder(16, 513)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "efConstruction must be in range [8, 512]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test boundary values", func() {
			// Min values
			builder, err := NewHNSWIndexBuilder(4, 8)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)

			// Max values
			builder, err = NewHNSWIndexBuilder(64, 512)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
		})
	})
}

func TestHNSWIndexBuilder_Build(t *testing.T) {
	convey.Convey("test HNSWIndexBuilder Build", t, func() {
		builder, _ := NewHNSWIndexBuilder(16, 128)

		convey.Convey("test build with L2 metric", func() {
			index, err := builder.Build(L2)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test build with IP metric", func() {
			index, err := builder.Build(IP)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test build with COSINE metric", func() {
			index, err := builder.Build(COSINE)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns HNSW", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeHNSW)
		})
	})
}

func TestNewIvfFlatIndexBuilder(t *testing.T) {
	convey.Convey("test NewIvfFlatIndexBuilder", t, func() {
		convey.Convey("test valid parameters", func() {
			builder, err := NewIvfFlatIndexBuilder(128)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test nlist too low", func() {
			builder, err := NewIvfFlatIndexBuilder(0)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test nlist too high", func() {
			builder, err := NewIvfFlatIndexBuilder(65537)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test boundary values", func() {
			// Min value
			builder, err := NewIvfFlatIndexBuilder(1)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)

			// Max value
			builder, err = NewIvfFlatIndexBuilder(65536)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
		})
	})
}

func TestIvfFlatIndexBuilder_Build(t *testing.T) {
	convey.Convey("test IvfFlatIndexBuilder Build", t, func() {
		builder, _ := NewIvfFlatIndexBuilder(128)

		convey.Convey("test build with L2 metric", func() {
			index, err := builder.Build(L2)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns IVF_FLAT", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeIvfFlat)
		})
	})
}

func TestNewFlatIndexBuilder(t *testing.T) {
	convey.Convey("test NewFlatIndexBuilder", t, func() {
		builder := NewFlatIndexBuilder()
		convey.So(builder, convey.ShouldNotBeNil)
	})
}

func TestFlatIndexBuilder_Build(t *testing.T) {
	convey.Convey("test FlatIndexBuilder Build", t, func() {
		builder := NewFlatIndexBuilder()

		convey.Convey("test build with L2 metric", func() {
			index, err := builder.Build(L2)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test build with COSINE metric", func() {
			index, err := builder.Build(COSINE)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns FLAT", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeFlat)
		})
	})
}

func TestNewAutoIndexBuilder(t *testing.T) {
	convey.Convey("test NewAutoIndexBuilder", t, func() {
		builder := NewAutoIndexBuilder()
		convey.So(builder, convey.ShouldNotBeNil)
	})
}

func TestAutoIndexBuilder_Build(t *testing.T) {
	convey.Convey("test AutoIndexBuilder Build", t, func() {
		builder := NewAutoIndexBuilder()

		convey.Convey("test build with L2 metric", func() {
			index, err := builder.Build(L2)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test build with IP metric", func() {
			index, err := builder.Build(IP)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns AUTOINDEX", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeAutoIndex)
		})
	})
}

func TestNewIvfSQ8IndexBuilder(t *testing.T) {
	convey.Convey("test NewIvfSQ8IndexBuilder", t, func() {
		convey.Convey("test valid parameters", func() {
			builder, err := NewIvfSQ8IndexBuilder(128)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test nlist too low", func() {
			builder, err := NewIvfSQ8IndexBuilder(0)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test nlist too high", func() {
			builder, err := NewIvfSQ8IndexBuilder(65537)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})
	})
}

func TestIvfSQ8IndexBuilder_Build(t *testing.T) {
	convey.Convey("test IvfSQ8IndexBuilder Build", t, func() {
		builder, _ := NewIvfSQ8IndexBuilder(128)

		convey.Convey("test build with L2 metric", func() {
			index, err := builder.Build(L2)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns IVF_SQ8", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeIvfSQ8)
		})
	})
}

func TestNewBinFlatIndexBuilder(t *testing.T) {
	convey.Convey("test NewBinFlatIndexBuilder", t, func() {
		convey.Convey("test valid parameters", func() {
			builder, err := NewBinFlatIndexBuilder(128)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test nlist too low", func() {
			builder, err := NewBinFlatIndexBuilder(0)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test nlist too high", func() {
			builder, err := NewBinFlatIndexBuilder(65537)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})
	})
}

func TestBinFlatIndexBuilder_Build(t *testing.T) {
	convey.Convey("test BinFlatIndexBuilder Build", t, func() {
		builder, _ := NewBinFlatIndexBuilder(128)

		convey.Convey("test build with HAMMING metric", func() {
			index, err := builder.Build(HAMMING)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns BIN_FLAT", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeBinFlat)
		})
	})
}

func TestNewBinIvfFlatIndexBuilder(t *testing.T) {
	convey.Convey("test NewBinIvfFlatIndexBuilder", t, func() {
		convey.Convey("test valid parameters", func() {
			builder, err := NewBinIvfFlatIndexBuilder(128)
			convey.So(err, convey.ShouldBeNil)
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test nlist too low", func() {
			builder, err := NewBinIvfFlatIndexBuilder(0)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})

		convey.Convey("test nlist too high", func() {
			builder, err := NewBinIvfFlatIndexBuilder(65537)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "nlist must be in range [1, 65536]")
			convey.So(builder, convey.ShouldBeNil)
		})
	})
}

func TestBinIvfFlatIndexBuilder_Build(t *testing.T) {
	convey.Convey("test BinIvfFlatIndexBuilder Build", t, func() {
		builder, _ := NewBinIvfFlatIndexBuilder(128)

		convey.Convey("test build with HAMMING metric", func() {
			index, err := builder.Build(HAMMING)
			convey.So(err, convey.ShouldBeNil)
			convey.So(index, convey.ShouldNotBeNil)
		})

		convey.Convey("test IndexType returns BIN_IVF_FLAT", func() {
			convey.So(builder.IndexType(), convey.ShouldEqual, IndexTypeBinIvfFlat)
		})
	})
}
