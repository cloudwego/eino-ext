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

// Iterator implements search iterator mode for traversing large result sets.
// Deprecated: Use milvus2.Iterator instead.
type Iterator = milvus2.Iterator

// NewIterator creates a new Iterator search mode.
// Deprecated: Use milvus2.NewIterator instead.
func NewIterator(metricType milvus2.MetricType, batchSize int) *Iterator {
	return milvus2.NewIterator(metricType, batchSize)
}
