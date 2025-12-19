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
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// ConsistencyLevel constants define the available consistency levels for Milvus collections.
const (
	// ConsistencyLevelStrong ensures the latest data is always returned.
	ConsistencyLevelStrong ConsistencyLevel = 1
	// ConsistencyLevelSession ensures reads reflect all writes from the current session.
	ConsistencyLevelSession ConsistencyLevel = 2
	// ConsistencyLevelBounded allows reads to lag behind writes by a bounded time (default 5 seconds).
	ConsistencyLevelBounded ConsistencyLevel = 3
	// ConsistencyLevelEventually provides the weakest consistency with eventual convergence.
	ConsistencyLevelEventually ConsistencyLevel = 4
	// ConsistencyLevelCustomized allows custom consistency level configuration.
	ConsistencyLevelCustomized ConsistencyLevel = 5
)

// MetricType constants define the available distance metrics for vector similarity.
const (
	// L2 represents Euclidean distance metric for FloatVector fields.
	L2 = MetricType(entity.L2)
	// IP represents Inner Product metric for FloatVector fields.
	IP = MetricType(entity.IP)
	// COSINE represents Cosine similarity metric for FloatVector fields.
	COSINE = MetricType(entity.COSINE)
	// HAMMING represents Hamming distance metric for BinaryVector fields.
	HAMMING = MetricType(entity.HAMMING)
	// JACCARD represents Jaccard distance metric for BinaryVector fields.
	JACCARD = MetricType(entity.JACCARD)
	// TANIMOTO represents Tanimoto distance metric.
	TANIMOTO = MetricType(entity.TANIMOTO)
	// SUBSTRUCTURE is a metric for chemical structure matching.
	SUBSTRUCTURE = MetricType(entity.SUBSTRUCTURE)
	// SUPERSTRUCTURE is a metric for chemical structure matching.
	SUPERSTRUCTURE = MetricType(entity.SUPERSTRUCTURE)

	// Deprecated: CONSINE has a typo; use COSINE instead.
	CONSINE = MetricType(entity.COSINE)
)

// defaultSchema represents the default row structure for storing documents in Milvus.
type defaultSchema struct {
	ID       string `json:"id" milvus:"name:id"`
	Content  string `json:"content" milvus:"name:content"`
	Vector   []byte `json:"vector" milvus:"name:vector"`
	Metadata []byte `json:"metadata" milvus:"name:metadata"`
}

// getDefaultFields returns the default collection schema fields.
// The schema includes id (primary key), vector (binary), content, and metadata fields.
func getDefaultFields() []*entity.Field {
	return []*entity.Field{
		entity.NewField().
			WithName(defaultCollectionID).
			WithDescription(defaultCollectionIDDesc).
			WithIsPrimaryKey(true).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(255),
		entity.NewField().
			WithName(defaultCollectionVector).
			WithDescription(defaultCollectionVectorDesc).
			WithIsPrimaryKey(false).
			WithDataType(entity.FieldTypeBinaryVector).
			WithDim(defaultDim),
		entity.NewField().
			WithName(defaultCollectionContent).
			WithDescription(defaultCollectionContentDesc).
			WithIsPrimaryKey(false).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(1024),
		entity.NewField().
			WithName(defaultCollectionMetadata).
			WithDescription(defaultCollectionMetadataDesc).
			WithIsPrimaryKey(false).
			WithDataType(entity.FieldTypeJSON),
	}
}

// ConsistencyLevel represents the consistency level for Milvus operations.
type ConsistencyLevel entity.ConsistencyLevel

// getConsistencyLevel converts the ConsistencyLevel to its Milvus entity equivalent.
func (c *ConsistencyLevel) getConsistencyLevel() entity.ConsistencyLevel {
	return entity.ConsistencyLevel(*c - 1)
}

// MetricType represents the distance metric type for vector similarity calculations.
type MetricType entity.MetricType

// getMetricType converts the MetricType to its Milvus entity equivalent.
func (t *MetricType) getMetricType() entity.MetricType {
	return entity.MetricType(*t)
}
