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
	
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
)

// DocumentConverter is a function type that converts documents and vectors into Milvus columns.
type DocumentConverter func(docs []*schema.Document, dim int, vectors [][]float64) ([]column.Column, error)

// defaultDocumentConverter is the default implementation of DocumentConverter.
// It converts documents into Milvus columns with the following schema:
// - doc_id: string
// - content: string
// - vector: float_vector (dimension specified by dim)
// - metadata: json_bytes (optional, marshaled from Document.MetaData)
func defaultDocumentConverter(docs []*schema.Document, dim int, vectors [][]float64) ([]column.Column, error) {
	var errs []error
	// Convert documents to Milvus columns
	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	metadata := make([][]byte, len(docs))
	for i, doc := range docs {
		ids[i] = doc.ID
		contents[i] = doc.Content
		if doc.MetaData != nil {
			md, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				errs = append(errs, err)
			} else {
				metadata[i] = md
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("[Indexer.DocumentConverter] failed to marshal metadata: %v", errs)
	}
	floats := make([][]float32, 0, len(vectors))
	for _, vector := range vectors {
		floats = append(floats, vector2Float32(vector))
	}
	columns := []column.Column{
		column.NewColumnVarChar("doc_id", ids),
		column.NewColumnVarChar("content", contents),
		column.NewColumnFloatVector("vector", dim, floats),
		column.NewColumnJSONBytes("metadata", metadata),
	}
	return columns, nil
}
