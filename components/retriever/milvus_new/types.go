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

package milvus_new

import "github.com/milvus-io/milvus/client/v2/entity"

// MetricType is the metric type for vector by eino
type MetricType entity.MetricType

// getMetricType returns the metric type
func (t *MetricType) getMetricType() entity.MetricType {
	return entity.MetricType(*t)
}

const (
	L2             = MetricType(entity.L2)
	IP             = MetricType(entity.IP)
	COSINE         = MetricType(entity.COSINE)
	HAMMING        = MetricType(entity.HAMMING)
	JACCARD        = MetricType(entity.JACCARD)
	TANIMOTO       = MetricType(entity.TANIMOTO)
	SUBSTRUCTURE   = MetricType(entity.SUBSTRUCTURE)
	SUPERSTRUCTURE = MetricType(entity.SUPERSTRUCTURE)
)
