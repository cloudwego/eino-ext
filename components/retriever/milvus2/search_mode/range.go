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
	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Range implements range search to find vectors within a specified distance or similarity radius.
// Deprecated: Use milvus2.Range instead.
type Range = milvus2.Range

// NewRange creates a new Range search mode.
// Deprecated: Use milvus2.NewRange instead.
func NewRange(metricType milvus2.MetricType, radius float64) *Range {
	return milvus2.NewRange(metricType, radius)
}
