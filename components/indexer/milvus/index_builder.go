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
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// IndexBuilder is the interface for building Milvus indexes.
// Use one of the provided builders (NewHNSWIndexBuilder, NewIvfFlatIndexBuilder, etc.)
// to create an IndexBuilder for use in IndexerConfig.
type IndexBuilder interface {
	// Build creates a Milvus entity.Index with the configured parameters and metric type.
	Build(metricType MetricType) (entity.Index, error)
	// IndexType returns the type of index this builder creates.
	IndexType() IndexType
}

// IndexType represents the type of vector index in Milvus.
type IndexType string

const (
	// IndexTypeFlat is a FLAT index (brute force, no compression).
	// Provides 100% recall but O(n) search complexity.
	IndexTypeFlat IndexType = "FLAT"
	// IndexTypeIvfFlat uses inverted file with flat quantizer.
	// Good balance of speed and accuracy for large datasets.
	IndexTypeIvfFlat IndexType = "IVF_FLAT"
	// IndexTypeIvfSQ8 uses inverted file with scalar quantization.
	// Reduces memory usage compared to IVF_FLAT with slight accuracy loss.
	IndexTypeIvfSQ8 IndexType = "IVF_SQ8"
	// IndexTypeIvfPQ uses inverted file with product quantization.
	// Best for very large datasets where memory is a concern.
	IndexTypeIvfPQ IndexType = "IVF_PQ"
	// IndexTypeHNSW uses Hierarchical Navigable Small World graph.
	// Excellent search performance with high recall.
	IndexTypeHNSW IndexType = "HNSW"
	// IndexTypeDiskANN uses disk-based approximate nearest neighbor.
	// Best for datasets that don't fit in memory.
	IndexTypeDiskANN IndexType = "DISKANN"
	// IndexTypeAutoIndex lets Milvus choose the optimal index.
	// Recommended for most use cases.
	IndexTypeAutoIndex IndexType = "AUTOINDEX"
	// IndexTypeBinFlat is FLAT index for binary vectors.
	IndexTypeBinFlat IndexType = "BIN_FLAT"
	// IndexTypeBinIvfFlat is IVF_FLAT for binary vectors.
	IndexTypeBinIvfFlat IndexType = "BIN_IVF_FLAT"
)

// HNSWIndexBuilder builds an HNSW (Hierarchical Navigable Small World) index.
// HNSW provides excellent search performance with high recall, suitable for
// most use cases requiring fast approximate nearest neighbor search.
type HNSWIndexBuilder struct {
	// M is the maximum number of outgoing connections in the graph.
	// Larger M leads to higher accuracy/performance at fixed ef/efConstruction.
	// Range: [4, 64], recommended: 8-32
	M int
	// EfConstruction is the size of the dynamic candidate list during construction.
	// Higher values lead to better index quality but slower index building.
	// Range: [8, 512], recommended: >= M
	EfConstruction int
}

// NewHNSWIndexBuilder creates an HNSW index builder with the given parameters.
// Parameters:
//   - m: Maximum number of outgoing connections in the graph [4, 64]
//   - efConstruction: Build-time search width [8, 512]
func NewHNSWIndexBuilder(m, efConstruction int) (*HNSWIndexBuilder, error) {
	if m < 4 || m > 64 {
		return nil, fmt.Errorf("M must be in range [4, 64], got %d", m)
	}
	if efConstruction < 8 || efConstruction > 512 {
		return nil, fmt.Errorf("efConstruction must be in range [8, 512], got %d", efConstruction)
	}
	return &HNSWIndexBuilder{M: m, EfConstruction: efConstruction}, nil
}

// Build creates an HNSW index with the configured parameters.
func (h *HNSWIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexHNSW(metricType.getMetricType(), h.M, h.EfConstruction)
}

// IndexType returns IndexTypeHNSW.
func (h *HNSWIndexBuilder) IndexType() IndexType {
	return IndexTypeHNSW
}

// IvfFlatIndexBuilder builds an IVF_FLAT (Inverted File with Flat quantizer) index.
// IVF_FLAT divides vectors into clusters and searches only relevant clusters,
// providing a good balance between speed and accuracy for large datasets.
type IvfFlatIndexBuilder struct {
	// NList is the number of cluster units (inverted lists).
	// Range: [1, 65536], recommended: 4*sqrt(n) to 16*sqrt(n) where n is number of vectors
	NList int
}

// NewIvfFlatIndexBuilder creates an IVF_FLAT index builder.
// Parameters:
//   - nlist: Number of cluster centroids [1, 65536]
func NewIvfFlatIndexBuilder(nlist int) (*IvfFlatIndexBuilder, error) {
	if nlist < 1 || nlist > 65536 {
		return nil, fmt.Errorf("nlist must be in range [1, 65536], got %d", nlist)
	}
	return &IvfFlatIndexBuilder{NList: nlist}, nil
}

// Build creates an IVF_FLAT index with the configured parameters.
func (i *IvfFlatIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexIvfFlat(metricType.getMetricType(), i.NList)
}

// IndexType returns IndexTypeIvfFlat.
func (i *IvfFlatIndexBuilder) IndexType() IndexType {
	return IndexTypeIvfFlat
}

// FlatIndexBuilder builds a FLAT index (brute force search).
// FLAT provides 100% recall but has O(n) search complexity.
// Best for small datasets (<10k vectors) or when perfect recall is required.
type FlatIndexBuilder struct{}

// NewFlatIndexBuilder creates a FLAT index builder.
func NewFlatIndexBuilder() *FlatIndexBuilder {
	return &FlatIndexBuilder{}
}

// Build creates a FLAT index.
func (f *FlatIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexFlat(metricType.getMetricType())
}

// IndexType returns IndexTypeFlat.
func (f *FlatIndexBuilder) IndexType() IndexType {
	return IndexTypeFlat
}

// AutoIndexBuilder builds an AUTOINDEX.
// With AUTOINDEX, Milvus automatically chooses the optimal index type
// based on the data characteristics. Recommended for most use cases.
type AutoIndexBuilder struct{}

// NewAutoIndexBuilder creates an AUTOINDEX builder.
func NewAutoIndexBuilder() *AutoIndexBuilder {
	return &AutoIndexBuilder{}
}

// Build creates an AUTOINDEX.
func (a *AutoIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexAUTOINDEX(metricType.getMetricType())
}

// IndexType returns IndexTypeAutoIndex.
func (a *AutoIndexBuilder) IndexType() IndexType {
	return IndexTypeAutoIndex
}

// IvfSQ8IndexBuilder builds an IVF_SQ8 (Inverted File with Scalar Quantization) index.
// IVF_SQ8 reduces memory usage compared to IVF_FLAT by quantizing vectors to 8-bit integers.
type IvfSQ8IndexBuilder struct {
	// NList is the number of cluster units (inverted lists).
	// Range: [1, 65536]
	NList int
}

// NewIvfSQ8IndexBuilder creates an IVF_SQ8 index builder.
func NewIvfSQ8IndexBuilder(nlist int) (*IvfSQ8IndexBuilder, error) {
	if nlist < 1 || nlist > 65536 {
		return nil, fmt.Errorf("nlist must be in range [1, 65536], got %d", nlist)
	}
	return &IvfSQ8IndexBuilder{NList: nlist}, nil
}

// Build creates an IVF_SQ8 index with the configured parameters.
func (i *IvfSQ8IndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexIvfSQ8(metricType.getMetricType(), i.NList)
}

// IndexType returns IndexTypeIvfSQ8.
func (i *IvfSQ8IndexBuilder) IndexType() IndexType {
	return IndexTypeIvfSQ8
}

// BinFlatIndexBuilder builds a BIN_FLAT index for binary vectors.
type BinFlatIndexBuilder struct {
	// NList is the number of cluster units.
	// Range: [1, 65536]
	NList int
}

// NewBinFlatIndexBuilder creates a BIN_FLAT index builder.
func NewBinFlatIndexBuilder(nlist int) (*BinFlatIndexBuilder, error) {
	if nlist < 1 || nlist > 65536 {
		return nil, fmt.Errorf("nlist must be in range [1, 65536], got %d", nlist)
	}
	return &BinFlatIndexBuilder{NList: nlist}, nil
}

// Build creates a BIN_FLAT index.
func (b *BinFlatIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexBinFlat(metricType.getMetricType(), b.NList)
}

// IndexType returns IndexTypeBinFlat.
func (b *BinFlatIndexBuilder) IndexType() IndexType {
	return IndexTypeBinFlat
}

// BinIvfFlatIndexBuilder builds a BIN_IVF_FLAT index for binary vectors.
type BinIvfFlatIndexBuilder struct {
	// NList is the number of cluster units.
	// Range: [1, 65536]
	NList int
}

// NewBinIvfFlatIndexBuilder creates a BIN_IVF_FLAT index builder.
func NewBinIvfFlatIndexBuilder(nlist int) (*BinIvfFlatIndexBuilder, error) {
	if nlist < 1 || nlist > 65536 {
		return nil, fmt.Errorf("nlist must be in range [1, 65536], got %d", nlist)
	}
	return &BinIvfFlatIndexBuilder{NList: nlist}, nil
}

// Build creates a BIN_IVF_FLAT index.
func (b *BinIvfFlatIndexBuilder) Build(metricType MetricType) (entity.Index, error) {
	return entity.NewIndexBinIvfFlat(metricType.getMetricType(), b.NList)
}

// IndexType returns IndexTypeBinIvfFlat.
func (b *BinIvfFlatIndexBuilder) IndexType() IndexType {
	return IndexTypeBinIvfFlat
}
