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

	"github.com/smartystreets/goconvey/convey"
)

func TestAutoIndexBuilder(t *testing.T) {
	convey.Convey("test AutoIndexBuilder", t, func() {
		convey.Convey("test NewAutoIndexBuilder", func() {
			builder := NewAutoIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with L2", func() {
			builder := NewAutoIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with IP", func() {
			builder := NewAutoIndexBuilder()
			idx := builder.Build(IP)
			convey.So(idx, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with COSINE", func() {
			builder := NewAutoIndexBuilder()
			idx := builder.Build(COSINE)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestFlatIndexBuilder(t *testing.T) {
	convey.Convey("test FlatIndexBuilder", t, func() {
		convey.Convey("test NewFlatIndexBuilder", func() {
			builder := NewFlatIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with L2", func() {
			builder := NewFlatIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with IP", func() {
			builder := NewFlatIndexBuilder()
			idx := builder.Build(IP)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestHNSWIndexBuilder(t *testing.T) {
	convey.Convey("test HNSWIndexBuilder", t, func() {
		convey.Convey("test NewHNSWIndexBuilder default values", func() {
			builder := NewHNSWIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.M, convey.ShouldEqual, 16)
			convey.So(builder.EfConstruction, convey.ShouldEqual, 200)
		})

		convey.Convey("test WithM", func() {
			builder := NewHNSWIndexBuilder().WithM(32)
			convey.So(builder.M, convey.ShouldEqual, 32)
		})

		convey.Convey("test WithEfConstruction", func() {
			builder := NewHNSWIndexBuilder().WithEfConstruction(500)
			convey.So(builder.EfConstruction, convey.ShouldEqual, 500)
		})

		convey.Convey("test chained methods", func() {
			builder := NewHNSWIndexBuilder().WithM(8).WithEfConstruction(100)
			convey.So(builder.M, convey.ShouldEqual, 8)
			convey.So(builder.EfConstruction, convey.ShouldEqual, 100)
		})

		convey.Convey("test Build", func() {
			builder := NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200)
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestIVFFlatIndexBuilder(t *testing.T) {
	convey.Convey("test IVFFlatIndexBuilder", t, func() {
		convey.Convey("test NewIVFFlatIndexBuilder default values", func() {
			builder := NewIVFFlatIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test WithNList", func() {
			builder := NewIVFFlatIndexBuilder().WithNList(256)
			convey.So(builder.NList, convey.ShouldEqual, 256)
		})

		convey.Convey("test Build", func() {
			builder := NewIVFFlatIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestIVFPQIndexBuilder(t *testing.T) {
	convey.Convey("test IVFPQIndexBuilder", t, func() {
		convey.Convey("test NewIVFPQIndexBuilder default values", func() {
			builder := NewIVFPQIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
			convey.So(builder.M, convey.ShouldEqual, 16)
			convey.So(builder.NBits, convey.ShouldEqual, 8)
		})

		convey.Convey("test WithNList", func() {
			builder := NewIVFPQIndexBuilder().WithNList(256)
			convey.So(builder.NList, convey.ShouldEqual, 256)
		})

		convey.Convey("test WithM", func() {
			builder := NewIVFPQIndexBuilder().WithM(32)
			convey.So(builder.M, convey.ShouldEqual, 32)
		})

		convey.Convey("test WithNBits", func() {
			builder := NewIVFPQIndexBuilder().WithNBits(4)
			convey.So(builder.NBits, convey.ShouldEqual, 4)
		})

		convey.Convey("test chained methods", func() {
			builder := NewIVFPQIndexBuilder().WithNList(64).WithM(8).WithNBits(16)
			convey.So(builder.NList, convey.ShouldEqual, 64)
			convey.So(builder.M, convey.ShouldEqual, 8)
			convey.So(builder.NBits, convey.ShouldEqual, 16)
		})

		convey.Convey("test Build", func() {
			builder := NewIVFPQIndexBuilder()
			idx := builder.Build(IP)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestIVFSQ8IndexBuilder(t *testing.T) {
	convey.Convey("test IVFSQ8IndexBuilder", t, func() {
		convey.Convey("test NewIVFSQ8IndexBuilder default values", func() {
			builder := NewIVFSQ8IndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test WithNList", func() {
			builder := NewIVFSQ8IndexBuilder().WithNList(64)
			convey.So(builder.NList, convey.ShouldEqual, 64)
		})

		convey.Convey("test Build", func() {
			builder := NewIVFSQ8IndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestDiskANNIndexBuilder(t *testing.T) {
	convey.Convey("test DiskANNIndexBuilder", t, func() {
		convey.Convey("test NewDiskANNIndexBuilder", func() {
			builder := NewDiskANNIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build", func() {
			builder := NewDiskANNIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestSCANNIndexBuilder(t *testing.T) {
	convey.Convey("test SCANNIndexBuilder", t, func() {
		convey.Convey("test NewSCANNIndexBuilder default values", func() {
			builder := NewSCANNIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
			convey.So(builder.WithRawData, convey.ShouldBeTrue)
		})

		convey.Convey("test WithNList", func() {
			builder := NewSCANNIndexBuilder().WithNList(256)
			convey.So(builder.NList, convey.ShouldEqual, 256)
		})

		convey.Convey("test WithRawDataEnabled", func() {
			builder := NewSCANNIndexBuilder().WithRawDataEnabled(false)
			convey.So(builder.WithRawData, convey.ShouldBeFalse)
		})

		convey.Convey("test Build", func() {
			builder := NewSCANNIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestBinFlatIndexBuilder(t *testing.T) {
	convey.Convey("test BinFlatIndexBuilder", t, func() {
		convey.Convey("test NewBinFlatIndexBuilder", func() {
			builder := NewBinFlatIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with HAMMING", func() {
			builder := NewBinFlatIndexBuilder()
			idx := builder.Build(HAMMING)
			convey.So(idx, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build with JACCARD", func() {
			builder := NewBinFlatIndexBuilder()
			idx := builder.Build(JACCARD)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestBinIVFFlatIndexBuilder(t *testing.T) {
	convey.Convey("test BinIVFFlatIndexBuilder", t, func() {
		convey.Convey("test NewBinIVFFlatIndexBuilder default values", func() {
			builder := NewBinIVFFlatIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test WithNList", func() {
			builder := NewBinIVFFlatIndexBuilder().WithNList(64)
			convey.So(builder.NList, convey.ShouldEqual, 64)
		})

		convey.Convey("test Build", func() {
			builder := NewBinIVFFlatIndexBuilder()
			idx := builder.Build(HAMMING)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestGPUBruteForceIndexBuilder(t *testing.T) {
	convey.Convey("test GPUBruteForceIndexBuilder", t, func() {
		convey.Convey("test NewGPUBruteForceIndexBuilder", func() {
			builder := NewGPUBruteForceIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build", func() {
			builder := NewGPUBruteForceIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestGPUIVFFlatIndexBuilder(t *testing.T) {
	convey.Convey("test GPUIVFFlatIndexBuilder", t, func() {
		convey.Convey("test NewGPUIVFFlatIndexBuilder", func() {
			builder := NewGPUIVFFlatIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build", func() {
			builder := NewGPUIVFFlatIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestGPUIVFPQIndexBuilder(t *testing.T) {
	convey.Convey("test GPUIVFPQIndexBuilder", t, func() {
		convey.Convey("test NewGPUIVFPQIndexBuilder", func() {
			builder := NewGPUIVFPQIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
		})

		convey.Convey("test Build", func() {
			builder := NewGPUIVFPQIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestGPUCagraIndexBuilder(t *testing.T) {
	convey.Convey("test GPUCagraIndexBuilder", t, func() {
		convey.Convey("test NewGPUCagraIndexBuilder default values", func() {
			builder := NewGPUCagraIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.IntermediateGraphDegree, convey.ShouldEqual, 128)
			convey.So(builder.GraphDegree, convey.ShouldEqual, 64)
		})

		convey.Convey("test WithIntermediateGraphDegree", func() {
			builder := NewGPUCagraIndexBuilder().WithIntermediateGraphDegree(256)
			convey.So(builder.IntermediateGraphDegree, convey.ShouldEqual, 256)
		})

		convey.Convey("test WithGraphDegree", func() {
			builder := NewGPUCagraIndexBuilder().WithGraphDegree(32)
			convey.So(builder.GraphDegree, convey.ShouldEqual, 32)
		})

		convey.Convey("test chained methods", func() {
			builder := NewGPUCagraIndexBuilder().WithIntermediateGraphDegree(64).WithGraphDegree(16)
			convey.So(builder.IntermediateGraphDegree, convey.ShouldEqual, 64)
			convey.So(builder.GraphDegree, convey.ShouldEqual, 16)
		})

		convey.Convey("test Build", func() {
			builder := NewGPUCagraIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}

func TestIVFRabitQIndexBuilder(t *testing.T) {
	convey.Convey("test IVFRabitQIndexBuilder", t, func() {
		convey.Convey("test NewIVFRabitQIndexBuilder default values", func() {
			builder := NewIVFRabitQIndexBuilder()
			convey.So(builder, convey.ShouldNotBeNil)
			convey.So(builder.NList, convey.ShouldEqual, 128)
		})

		convey.Convey("test WithNList", func() {
			builder := NewIVFRabitQIndexBuilder().WithNList(256)
			convey.So(builder.NList, convey.ShouldEqual, 256)
		})

		convey.Convey("test Build", func() {
			builder := NewIVFRabitQIndexBuilder()
			idx := builder.Build(L2)
			convey.So(idx, convey.ShouldNotBeNil)
		})
	})
}
