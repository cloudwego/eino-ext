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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	rr "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "eino_doc_customized:"  // keyPrefix should be the prefix of keys you write to redis and want to retrieve.
	indexName = "test_index_customized" // indexName should be used in redis retriever.

	customContentFieldName       = "my_content_field"
	customContentVectorFieldName = "my_vector_content_field"
	customExtraFieldName         = "extra_field_number"
)

// This example related to example in https://github.com/cloudwego/eino-ext/tree/main/components/indexer/redis/examples/customized_indexer
func main() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr:          "localhost:6379",
		Protocol:      2,
		UnstableResp3: true,
	})

	b, err := os.ReadFile("./examples/embeddings.json")
	if err != nil {
		panic(err)
	}

	var dense [][]float64
	if err = json.Unmarshal(b, &dense); err != nil {
		panic(err)
	}

	// customize with your own index structure.
	r, err := rr.NewRetriever(ctx, &rr.RetrieverConfig{
		Client:      client,
		Index:       indexName,
		VectorField: customContentVectorFieldName,
		Dialect:     2,
		ReturnFields: []string{
			customContentFieldName,
			customContentVectorFieldName,
			customExtraFieldName,
		},
		DocumentConverter: func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
			resp := &schema.Document{
				ID:       strings.TrimPrefix(doc.ID, keyPrefix),
				MetaData: map[string]any{},
			}
			for k, v := range doc.Fields {
				switch k {
				case customContentVectorFieldName:
					resp.WithDenseVector(rr.Bytes2Vector([]byte(v)))
				case customContentFieldName:
					resp.Content = v
				case customExtraFieldName:
					i, err := strconv.ParseInt(v, 10, 64)
					if err != nil {
						return nil, err
					}
					resp.MetaData["ext"] = i
				default:
					return nil, fmt.Errorf("unexpected field=%s", k)
				}
			}
			return resp, nil
		},
		TopK:      5,
		Embedding: &mockEmbedding{dense},
	})
	if err != nil {
		panic(err)
	}

	docs, err := r.Retrieve(ctx, "tourist attraction")
	if err != nil {
		panic(err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, ext_number:%d, content:%v\n", doc.ID, doc.MetaData["ext"], doc.Content)
		//fmt.Println(doc.DenseVector())
	}
	// id:8, ext_number:10008, content:8. Niagara Falls: located at the border of the United States and Canada, consisting of three main waterfalls, its spectacular scenery attracts millions of tourists every year.
	// id:3, ext_number:10003, content:3. Grand Canyon National Park: Located in Arizona, USA, it is famous for its deep canyons and magnificent scenery, which are cut by the Colorado River.
	// id:6, ext_number:10006, content:6. Sydney Opera House: Located in Sydney Harbour, Australia, it is one of the most iconic buildings of the 20th century, renowned for its unique sailboat design.
	// id:1, ext_number:10001, content:1. Eiffel Tower: Located in Paris, France, it is one of the most famous landmarks in the world, designed by Gustave Eiffel and built in 1889.
	// id:5, ext_number:10005, content:5. Taj Mahal: Located in Agra, India, it was completed by Mughal Emperor Shah Jahan in 1653 to commemorate his wife and is one of the New Seven Wonders of the World.
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
