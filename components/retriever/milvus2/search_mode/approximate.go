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

// Package search_mode provides search mode implementations for the milvus2 retriever.
package search_mode

import (
	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
)

// Approximate implements approximate nearest neighbor (ANN) search.
// Deprecated: Use milvus2.Approximate instead.
type Approximate = milvus2.Approximate

// NewApproximate creates a new Approximate search mode with the specified metric type.
// Deprecated: Use milvus2.NewApproximate instead.
func NewApproximate(metricType milvus2.MetricType) *Approximate {
	return milvus2.NewApproximate(metricType)
}
