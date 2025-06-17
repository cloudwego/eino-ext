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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/retriever/pinecone"
	"github.com/cloudwego/eino/components/embedding"
)

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("./examples/ds_query.json")
	if err != nil {
		panic(err)
	}

	var vs struct {
		Dense  [][]float64       `json:"dense"`
		Sparse []map[int]float64 `json:"sparse"`
	}

	if err := json.Unmarshal(b, &vs); err != nil {
		panic(err)
	}

	apiKey := os.Getenv("PINECONE_API_KEY")
	indexName := "eino-index-test"

	retriever, err := pinecone.NewRetriever(ctx, &pinecone.RetrieverConfig{
		ApiKey:    apiKey,
		IndexName: indexName,
		Namespace: "", // default name space
		Embedding: mockEmbedding{nil},
	})
	if err != nil {
		panic(err)
	}

	/*
		Pinecone sparse-dense vectors have the following limitations:

		Records with sparse vector values must also contain dense vector values.

		Sparse vector values can contain up to 1000 non-zero values and 4.2 billion dimensions.

		Only indexes using the dotproduct distance metric support querying sparse-dense vectors.

		Upserting, updating, and fetching sparse-dense vectors in indexes with a different distance metric will succeed, but querying will return an error.

		Indexes created before February 22, 2023 do not support sparse vectors.
	*/

	// query is plain text, provide additional info by options
	resp, err := retriever.Retrieve(ctx, "tourist attraction",
		pinecone.WithQueryDenseVector(vs.Dense[0]),                                 // (optional) dense vector here will be used preferentially and will not trigger the embedding operation.
		pinecone.WithQuerySparseVector(vs.Sparse[0]),                               // (optional) set sparse vector for retrieve
		pinecone.WithQueryMetadataFilter(map[string]interface{}{"location": "中国"}), // (optional) meta data filter
	)
	if err != nil {
		panic(err)
	}

	for _, doc := range resp {
		fmt.Printf("id=%s content=%s loc=%s, score=%v\n", doc.ID, doc.Content, doc.MetaData["location"], doc.Score())
		// fmt.Printf("dense=%v, sparse=%v\n", doc.DenseVector(), doc.SparseVector())
	}

	// id=2 content=2. 长城：位于中国，是世界七大奇迹之一，从秦至明代修筑而成，全长超过 2 万公里。 loc=中国, score=0.5461838841438293
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
