/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package milvus

import (
	"strconv"
	
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// VectorConverter defines a function type that converts float64 vectors to Milvus entity.Vector format
// This is used to transform embedding vectors into the format required by Milvus search operations
type VectorConverter func(vectors [][]float64) ([]entity.Vector, error)

// DocumentConverter defines a function type that converts Milvus search results to schema.Document objects
// This is used to transform Milvus ResultSet into the standard document format used by the retriever
type DocumentConverter func(result []milvusclient.ResultSet) ([]*schema.Document, error)

// getDefaultVectorConverter returns the default vector converter implementation
// It converts float64 vectors to FloatVector entities compatible with Milvus
func getDefaultVectorConverter() VectorConverter {
	return func(vectors [][]float64) ([]entity.Vector, error) {
		if len(vectors) == 0 {
			return nil, nil
		}
		vecs := make([]entity.Vector, len(vectors))
		for i, v := range vectors {
			vecs[i] = entity.FloatVector(vector2Float32(v))
		}
		return vecs, nil
	}
}

// getDefaultDocumentConverter returns the default document converter implementation
// It converts Milvus ResultSet to schema.Document objects with proper field mapping
func getDefaultDocumentConverter() DocumentConverter {
	return func(result []milvusclient.ResultSet) ([]*schema.Document, error) {
		var errs []error
		if len(result) == 0 {
			return nil, nil
		}
		docs := make([]*schema.Document, result[0].ResultCount)
		if result[0].Fields.Len() >= 1 {
			if err := result[0].Unmarshal(docs); err != nil {
				return nil, err
			}
		} else {
			for i := range docs {
				id, err := result[0].IDs.Get(i)
				if err != nil {
					errs = append(errs, err)
				}
				docs[i] = &schema.Document{
					ID: strconv.FormatInt(id.(int64), 10),
					MetaData: map[string]any{
						"score": result[0].Scores[i],
					},
				}
			}
		}
		return docs, nil
	}
}
