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
	"github.com/milvus-io/milvus/client/v2/index"
)

// IndexBuilder defines the interface for building Milvus vector indexes.
// It provides different index types with their specific parameters.
type IndexBuilder interface {
	// Build creates a Milvus index configured with the specified metric type.
	Build(metricType MetricType) index.Index
}

// SparseIndexBuilder defines the interface for building Milvus sparse vector indexes.
type SparseIndexBuilder interface {
	// Build creates a Milvus sparse index configured with the specified metric type.
	Build(metricType MetricType) index.Index
}

// AutoIndexBuilder creates an AUTOINDEX that allows Milvus to automatically
// select the optimal index type based on data characteristics.
type AutoIndexBuilder struct{}

// NewAutoIndexBuilder creates a new AutoIndexBuilder instance.
func NewAutoIndexBuilder() *AutoIndexBuilder {
	return &AutoIndexBuilder{}
}

// Build creates an AUTOINDEX using the specified metric type.
func (b *AutoIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewAutoIndex(metricType.toEntity())
}

// FlatIndexBuilder creates a FLAT index for brute-force exact search.
// It guarantees 100% recall accuracy but is slowest for large datasets.
type FlatIndexBuilder struct{}

// NewFlatIndexBuilder creates a new FlatIndexBuilder instance.
func NewFlatIndexBuilder() *FlatIndexBuilder {
	return &FlatIndexBuilder{}
}

// Build creates a FLAT index using the specified metric type.
func (b *FlatIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewFlatIndex(metricType.toEntity())
}

// HNSWIndexBuilder creates an HNSW (Hierarchical Navigable Small World) index.
// It provides a graph-based index with excellent query performance.
type HNSWIndexBuilder struct {
	// M specifies the maximum degree of nodes in graph layers (recommended: 4-64).
	M int
	// EfConstruction specifies the search width during index building (recommended: 8-512).
	EfConstruction int
}

// NewHNSWIndexBuilder creates a new HNSWIndexBuilder with default parameters.
// Default: M=16, EfConstruction=200
func NewHNSWIndexBuilder() *HNSWIndexBuilder {
	return &HNSWIndexBuilder{
		M:              16,
		EfConstruction: 200,
	}
}

// WithM sets the maximum degree of nodes in graph layers.
func (b *HNSWIndexBuilder) WithM(m int) *HNSWIndexBuilder {
	b.M = m
	return b
}

// WithEfConstruction sets the search width during index building.
func (b *HNSWIndexBuilder) WithEfConstruction(ef int) *HNSWIndexBuilder {
	b.EfConstruction = ef
	return b
}

// Build creates an HNSW index using the specified metric type.
func (b *HNSWIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewHNSWIndex(metricType.toEntity(), b.M, b.EfConstruction)
}

// IVFFlatIndexBuilder creates an IVF_FLAT (Inverted File with Flat) index.
// It partitions data into clusters for faster search.
type IVFFlatIndexBuilder struct {
	// NList is the number of cluster units (recommended: 1-65536).
	NList int
}

// NewIVFFlatIndexBuilder creates a new IVFFlatIndexBuilder with default parameters.
// Default: NList=128
func NewIVFFlatIndexBuilder() *IVFFlatIndexBuilder {
	return &IVFFlatIndexBuilder{
		NList: 128,
	}
}

// WithNList sets the NList parameter.
func (b *IVFFlatIndexBuilder) WithNList(nlist int) *IVFFlatIndexBuilder {
	b.NList = nlist
	return b
}

// Build creates an IVF_FLAT index.
func (b *IVFFlatIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewIvfFlatIndex(metricType.toEntity(), b.NList)
}

// IVFPQIndexBuilder creates an IVF_PQ (Inverted File with Product Quantization) index.
// It provides significant memory savings with good search quality.
type IVFPQIndexBuilder struct {
	// NList is the number of cluster units (recommended: 1-65536).
	NList int
	// M is the number of subquantizers (recommended: 1-div).
	M int
	// NBits is the number of bits for each subquantizer (recommended: 1-16).
	NBits int
}

// NewIVFPQIndexBuilder creates a new IVFPQIndexBuilder with default parameters.
// Default: NList=128, M=16, NBits=8
func NewIVFPQIndexBuilder() *IVFPQIndexBuilder {
	return &IVFPQIndexBuilder{
		NList: 128,
		M:     16,
		NBits: 8,
	}
}

// WithNList sets the NList parameter.
func (b *IVFPQIndexBuilder) WithNList(nlist int) *IVFPQIndexBuilder {
	b.NList = nlist
	return b
}

// WithM sets the M parameter.
func (b *IVFPQIndexBuilder) WithM(m int) *IVFPQIndexBuilder {
	b.M = m
	return b
}

// WithNBits sets the NBits parameter.
func (b *IVFPQIndexBuilder) WithNBits(nbits int) *IVFPQIndexBuilder {
	b.NBits = nbits
	return b
}

// Build creates an IVF_PQ index.
func (b *IVFPQIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewIvfPQIndex(metricType.toEntity(), b.NList, b.M, b.NBits)
}

// IVFSQ8IndexBuilder creates an IVF_SQ8 (Inverted File with Scalar Quantization) index.
// It uses scalar quantization for memory efficiency.
type IVFSQ8IndexBuilder struct {
	// NList is the number of cluster units (recommended: 1-65536).
	NList int
}

// NewIVFSQ8IndexBuilder creates a new IVFSQ8IndexBuilder with default parameters.
// Default: NList=128
func NewIVFSQ8IndexBuilder() *IVFSQ8IndexBuilder {
	return &IVFSQ8IndexBuilder{
		NList: 128,
	}
}

// WithNList sets the NList parameter.
func (b *IVFSQ8IndexBuilder) WithNList(nlist int) *IVFSQ8IndexBuilder {
	b.NList = nlist
	return b
}

// Build creates an IVF_SQ8 index.
func (b *IVFSQ8IndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewIvfSQ8Index(metricType.toEntity(), b.NList)
}

// DiskANNIndexBuilder creates a DISKANN index for disk-based ANN search.
// It is optimized for large datasets that don't fit in memory.
type DiskANNIndexBuilder struct{}

// NewDiskANNIndexBuilder creates a new DiskANNIndexBuilder.
func NewDiskANNIndexBuilder() *DiskANNIndexBuilder {
	return &DiskANNIndexBuilder{}
}

// Build creates a DISKANN index.
func (b *DiskANNIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewDiskANNIndex(metricType.toEntity())
}

// SCANNIndexBuilder creates a SCANN (Scalable Approximate Nearest Neighbors) index.
type SCANNIndexBuilder struct {
	// NList is the number of cluster units.
	NList int
	// WithRawData determines whether to include raw data for reranking.
	WithRawData bool
}

// NewSCANNIndexBuilder creates a new SCANNIndexBuilder with default parameters.
// Default: NList=128, WithRawData=true
func NewSCANNIndexBuilder() *SCANNIndexBuilder {
	return &SCANNIndexBuilder{
		NList:       128,
		WithRawData: true,
	}
}

// WithNList sets the NList parameter.
func (b *SCANNIndexBuilder) WithNList(nlist int) *SCANNIndexBuilder {
	b.NList = nlist
	return b
}

// WithRawDataEnabled sets whether to include raw data for reranking.
func (b *SCANNIndexBuilder) WithRawDataEnabled(enabled bool) *SCANNIndexBuilder {
	b.WithRawData = enabled
	return b
}

// Build creates a SCANN index.
func (b *SCANNIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewSCANNIndex(metricType.toEntity(), b.NList, b.WithRawData)
}

// BinFlatIndexBuilder creates a BIN_FLAT index for binary vectors.
type BinFlatIndexBuilder struct{}

// NewBinFlatIndexBuilder creates a new BinFlatIndexBuilder.
func NewBinFlatIndexBuilder() *BinFlatIndexBuilder {
	return &BinFlatIndexBuilder{}
}

// Build creates a BIN_FLAT index.
func (b *BinFlatIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewBinFlatIndex(metricType.toEntity())
}

// BinIVFFlatIndexBuilder creates a BIN_IVF_FLAT index for binary vectors.
type BinIVFFlatIndexBuilder struct {
	// NList is the number of cluster units.
	NList int
}

// NewBinIVFFlatIndexBuilder creates a new BinIVFFlatIndexBuilder with default parameters.
// Default: NList=128
func NewBinIVFFlatIndexBuilder() *BinIVFFlatIndexBuilder {
	return &BinIVFFlatIndexBuilder{
		NList: 128,
	}
}

// WithNList sets the NList parameter.
func (b *BinIVFFlatIndexBuilder) WithNList(nlist int) *BinIVFFlatIndexBuilder {
	b.NList = nlist
	return b
}

// Build creates a BIN_IVF_FLAT index.
func (b *BinIVFFlatIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewBinIvfFlatIndex(metricType.toEntity(), b.NList)
}

// GPUBruteForceIndexBuilder creates a GPU-accelerated brute-force index.
type GPUBruteForceIndexBuilder struct{}

// NewGPUBruteForceIndexBuilder creates a new GPUBruteForceIndexBuilder.
func NewGPUBruteForceIndexBuilder() *GPUBruteForceIndexBuilder {
	return &GPUBruteForceIndexBuilder{}
}

// Build creates a GPU brute-force index.
func (b *GPUBruteForceIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewGPUBruteForceIndex(metricType.toEntity())
}

// GPUIVFFlatIndexBuilder creates a GPU-accelerated IVF_FLAT index.
type GPUIVFFlatIndexBuilder struct{}

// NewGPUIVFFlatIndexBuilder creates a new GPUIVFFlatIndexBuilder.
func NewGPUIVFFlatIndexBuilder() *GPUIVFFlatIndexBuilder {
	return &GPUIVFFlatIndexBuilder{}
}

// Build creates a GPU IVF_FLAT index.
func (b *GPUIVFFlatIndexBuilder) Build(metricType MetricType) index.Index {
	// NewGPUIVPFlatIndex is a typo in Milvus SDK (should be NewGPUIVFFlatIndex).
	// We keep using it until SDK fixes it.
	return index.NewGPUIVPFlatIndex(metricType.toEntity())
}

// GPUIVFPQIndexBuilder creates a GPU-accelerated IVF_PQ index.
// It provides high-speed approximate nearest neighbor search with memory efficiency.
// Note: The current Milvus SDK implementation does not expose NList, M, NBits parameters.
type GPUIVFPQIndexBuilder struct{}

// NewGPUIVFPQIndexBuilder creates a new GPUIVFPQIndexBuilder.
func NewGPUIVFPQIndexBuilder() *GPUIVFPQIndexBuilder {
	return &GPUIVFPQIndexBuilder{}
}

// Build creates a GPU IVF_PQ index.
func (b *GPUIVFPQIndexBuilder) Build(metricType MetricType) index.Index {
	// NewGPUIVPPQIndex is a typo in Milvus SDK (should be NewGPUIVFPQIndex).
	// We keep using it until SDK fixes it.
	return index.NewGPUIVPPQIndex(metricType.toEntity())
}

// GPUCagraIndexBuilder creates a GPU CAGRA index.
type GPUCagraIndexBuilder struct {
	// IntermediateGraphDegree is the degree of the intermediate graph.
	IntermediateGraphDegree int
	// GraphDegree is the degree of the final graph.
	GraphDegree int
}

// NewGPUCagraIndexBuilder creates a new GPUCagraIndexBuilder with default parameters.
// Default: IntermediateGraphDegree=128, GraphDegree=64
func NewGPUCagraIndexBuilder() *GPUCagraIndexBuilder {
	return &GPUCagraIndexBuilder{
		IntermediateGraphDegree: 128,
		GraphDegree:             64,
	}
}

// WithIntermediateGraphDegree sets the intermediate graph degree.
func (b *GPUCagraIndexBuilder) WithIntermediateGraphDegree(degree int) *GPUCagraIndexBuilder {
	b.IntermediateGraphDegree = degree
	return b
}

// WithGraphDegree sets the final graph degree.
func (b *GPUCagraIndexBuilder) WithGraphDegree(degree int) *GPUCagraIndexBuilder {
	b.GraphDegree = degree
	return b
}

// Build creates a GPU CAGRA index.
func (b *GPUCagraIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewGPUCagraIndex(metricType.toEntity(), b.IntermediateGraphDegree, b.GraphDegree)
}

// IVFRabitQIndexBuilder creates an IVF_RABITQ index (Milvus 2.6+).
// It combines IVF clustering with RaBitQ binary quantization for storage efficiency.
type IVFRabitQIndexBuilder struct {
	// NList is the number of cluster units (recommended: 32-4096).
	NList int
}

// NewIVFRabitQIndexBuilder creates a new IVFRabitQIndexBuilder with default parameters.
// Default: NList=128
func NewIVFRabitQIndexBuilder() *IVFRabitQIndexBuilder {
	return &IVFRabitQIndexBuilder{
		NList: 128,
	}
}

// WithNList sets the number of cluster units.
func (b *IVFRabitQIndexBuilder) WithNList(nlist int) *IVFRabitQIndexBuilder {
	b.NList = nlist
	return b
}

// Build creates an IVF_RABITQ index.
func (b *IVFRabitQIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewIvfRabitQIndex(metricType.toEntity(), b.NList)
}

// SparseInvertedIndexBuilder creates a SPARSE_INVERTED_INDEX.
type SparseInvertedIndexBuilder struct {
	DropRatioBuild float64
}

// NewSparseInvertedIndexBuilder creates a new SparseInvertedIndexBuilder.
func NewSparseInvertedIndexBuilder() *SparseInvertedIndexBuilder {
	return &SparseInvertedIndexBuilder{
		DropRatioBuild: 0.2,
	}
}

// WithDropRatioBuild sets the drop ratio for building the index.
func (b *SparseInvertedIndexBuilder) WithDropRatioBuild(ratio float64) *SparseInvertedIndexBuilder {
	b.DropRatioBuild = ratio
	return b
}

// Build creates a SPARSE_INVERTED_INDEX.
func (b *SparseInvertedIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewSparseInvertedIndex(metricType.toEntity(), b.DropRatioBuild)
}

// SparseWANDIndexBuilder creates a SPARSE_WAND index.
type SparseWANDIndexBuilder struct {
	DropRatioBuild float64
}

// NewSparseWANDIndexBuilder creates a new SparseWANDIndexBuilder.
func NewSparseWANDIndexBuilder() *SparseWANDIndexBuilder {
	return &SparseWANDIndexBuilder{
		DropRatioBuild: 0.2,
	}
}

// WithDropRatioBuild sets the drop ratio for building the index.
func (b *SparseWANDIndexBuilder) WithDropRatioBuild(ratio float64) *SparseWANDIndexBuilder {
	b.DropRatioBuild = ratio
	return b
}

// Build creates a SPARSE_WAND index.
func (b *SparseWANDIndexBuilder) Build(metricType MetricType) index.Index {
	return index.NewSparseWANDIndex(metricType.toEntity(), b.DropRatioBuild)
}

// Ensure all builders implement IndexBuilder or SparseIndexBuilder
var (
	_ IndexBuilder = (*AutoIndexBuilder)(nil)
	_ IndexBuilder = (*FlatIndexBuilder)(nil)
	_ IndexBuilder = (*HNSWIndexBuilder)(nil)
	_ IndexBuilder = (*IVFFlatIndexBuilder)(nil)
	_ IndexBuilder = (*IVFPQIndexBuilder)(nil)
	_ IndexBuilder = (*IVFSQ8IndexBuilder)(nil)
	_ IndexBuilder = (*DiskANNIndexBuilder)(nil)
	_ IndexBuilder = (*SCANNIndexBuilder)(nil)
	_ IndexBuilder = (*BinFlatIndexBuilder)(nil)
	_ IndexBuilder = (*BinIVFFlatIndexBuilder)(nil)
	_ IndexBuilder = (*GPUBruteForceIndexBuilder)(nil)
	_ IndexBuilder = (*GPUIVFFlatIndexBuilder)(nil)
	_ IndexBuilder = (*GPUCagraIndexBuilder)(nil)
	_ IndexBuilder = (*GPUIVFPQIndexBuilder)(nil)
	_ IndexBuilder = (*IVFRabitQIndexBuilder)(nil)

	_ SparseIndexBuilder = (*SparseInvertedIndexBuilder)(nil)
	_ SparseIndexBuilder = (*SparseWANDIndexBuilder)(nil)
)
