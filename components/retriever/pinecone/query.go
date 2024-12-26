/*
 * Copyright 2024 CloudWeGo Authors
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

package pinecone

import "encoding/json"

// Query support additional query fields over simple text
// see: https://docs.pinecone.io/guides/data/query-data
type Query struct {
	Text           string                 `json:"text"`             // required
	DenseVector    []float64              `json:"dense_vector"`     // optional, embedding method from retriever config will be used if not provided
	SparseVector   map[int]float64        `json:"sparse_vector"`    // optional, indices -> vector
	MetaDataFilter map[string]interface{} `json:"meta_data_filter"` // optional,
}

func (q *Query) ToQuery() (string, error) {
	b, err := json.Marshal(q)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
