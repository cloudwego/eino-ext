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

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// SearchMode defines the interface for executing Milvus search operations.
// It encapsulates the entire search logic, including option building and client execution.
type SearchMode interface {
	// Retrieve performs the search operation using the provided client and configuration.
	Retrieve(ctx context.Context, client *milvusclient.Client, conf *RetrieverConfig, query string, opts ...retriever.Option) ([]*schema.Document, error)
}
