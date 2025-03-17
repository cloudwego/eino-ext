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

package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func createIndex() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// below use FT.CREATE to create an index.
	// see: https://redis.io/docs/latest/commands/ft.create/

	keyPrefix := "eino_doc:"  // keyPrefix should be the prefix of keys you write to redis and want to retrieve.
	indexName := "test_index" // indexName should be used in redis retriever.

	// schemas should match DocumentToHashes configured in IndexerConfig.
	schemas := []*redis.FieldSchema{
		{
			FieldName: "content",
			FieldType: redis.SearchFieldTypeText,
			Weight:    1,
		},
		{
			FieldName: "vector_content",
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{
				// FLAT index: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#flat-index
				// Choose the FLAT index when you have small datasets (< 1M vectors) or when perfect search accuracy is more important than search latency.
				FlatOptions: &redis.FTFlatOptions{
					Type:           "FLOAT32", // BFLOAT16 / FLOAT16 / FLOAT32 / FLOAT64. BFLOAT16 and FLOAT16 require v2.10 or later.
					Dim:            1024,      // keeps same with dimensions of Embedding
					DistanceMetric: "COSINE",  // L2 / IP / COSINE
				},
				// HNSW index: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#hnsw-index
				// HNSW, or hierarchical navigable small world, is an approximate nearest neighbors algorithm that uses a multi-layered graph to make vector search more scalable.
				HNSWOptions: nil,
			},
		},
		{
			FieldName: "extra_field_number",
			FieldType: redis.SearchFieldTypeNumeric,
		},
	}

	options := &redis.FTCreateOptions{
		OnHash: true,
		Prefix: []any{keyPrefix},
	}

	result, err := client.FTCreate(ctx, indexName, options, schemas...).Result()
	if err != nil {
		panic(err)
	}

	fmt.Println(result) // OK
}
