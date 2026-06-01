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

import "github.com/cloudwego/eino/components/indexer"

// ImplOptions contains implementation-specific options for indexing operations.
type ImplOptions struct {
	// Partition specifies the target partition for document insertion.
	// If empty, documents are inserted into the default partition.
	Partition string
}

// WithPartition returns an option that sets the target partition for insertion.
func WithPartition(partition string) indexer.Option {
	return indexer.WrapImplSpecificOptFn(func(o *ImplOptions) {
		o.Partition = partition
	})
}
