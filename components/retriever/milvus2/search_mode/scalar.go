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

// Scalar implements scalar/metadata search using the Milvus Query API.
// Deprecated: Use milvus2.Scalar instead.
type Scalar = milvus2.Scalar

// NewScalar creates a new Scalar search mode.
// Deprecated: Use milvus2.NewScalar instead.
func NewScalar() *Scalar {
	return milvus2.NewScalar()
}
