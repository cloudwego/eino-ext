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
	"context"

	"github.com/milvus-io/milvus/client/v2/entity"
)

// MetricType represents the metric type for vector similarity.
type MetricType string

const (
	// L2 represents Euclidean distance (L2 norm).
	// Suitable for dense floating-point vectors.
	L2 MetricType = "L2"

	// IP represents Inner Product similarity.
	// Higher values indicate greater similarity. Suitable for normalized vectors.
	IP MetricType = "IP"

	// COSINE represents Cosine similarity.
	// Measures the cosine of the angle between vectors, ignoring magnitude.
	COSINE MetricType = "COSINE"

	// HAMMING represents Hamming distance for binary vectors.
	// Counts the number of positions where the corresponding bits differ.
	HAMMING MetricType = "HAMMING"

	// JACCARD represents Jaccard distance for binary vectors.
	// Measures dissimilarity between sample sets.
	JACCARD MetricType = "JACCARD"

	// TANIMOTO represents Tanimoto distance for binary vectors.
	// Similar to Jaccard, used for molecular fingerprint comparisons.
	TANIMOTO MetricType = "TANIMOTO"

	// SUBSTRUCTURE represents substructure search for binary vectors.
	// Returns true if A is a subset of B.
	SUBSTRUCTURE MetricType = "SUBSTRUCTURE"

	// SUPERSTRUCTURE represents superstructure search for binary vectors.
	// Returns true if B is a subset of A.
	SUPERSTRUCTURE MetricType = "SUPERSTRUCTURE"
)

// toEntity converts MetricType to the Milvus SDK entity.MetricType.
func (m MetricType) toEntity() entity.MetricType {
	return entity.MetricType(m)
}

// ConsistencyLevel represents the consistency level for Milvus operations.
type ConsistencyLevel int32

const (
	// ConsistencyLevelStrong ensures that the read operation sees all the data written before the read.
	ConsistencyLevelStrong ConsistencyLevel = ConsistencyLevel(entity.ClStrong)

	// ConsistencyLevelSession ensures that the read operation sees all the data written in the same session.
	ConsistencyLevelSession ConsistencyLevel = ConsistencyLevel(entity.ClSession)

	// ConsistencyLevelBounded ensures that the read operation sees the data written before a certain time point.
	ConsistencyLevelBounded ConsistencyLevel = ConsistencyLevel(entity.ClBounded)

	// ConsistencyLevelEventually ensures that the read operation eventually sees the data written.
	ConsistencyLevelEventually ConsistencyLevel = ConsistencyLevel(entity.ClEventually)
)

// ToEntity converts ConsistencyLevel to the Milvus SDK entity.ConsistencyLevel type.
func (c ConsistencyLevel) ToEntity() entity.ConsistencyLevel {
	return entity.ConsistencyLevel(c)
}

// SparseEmbedder defines the interface for generating sparse vector embeddings.
type SparseEmbedder interface {
	// EmbedStrings embeds a list of texts into sparse vectors.
	// Returns a slice of maps where keys are dimension indices and values are weights.
	EmbedStrings(ctx context.Context, texts []string) ([]map[int]float64, error)
}

// VectorType represents the type of vector field.
type VectorType string

const (
	// DenseVector represents standard dense floating-point vectors.
	DenseVector VectorType = "dense"

	// SparseVector represents sparse vectors (map of index to weight).
	SparseVector VectorType = "sparse"
)
