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

	rr "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
)

// This example related to example in https://github.com/cloudwego/eino-ext/tree/main/components/indexer/redis/examples/default_indexer

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

	r, err := rr.NewRetriever(ctx, &rr.RetrieverConfig{
		Client:    client,
		Index:     "test_index",          // created index name
		Embedding: &mockEmbedding{dense}, // replace with real embedding.
	})
	if err != nil {
		panic(err)
	}

	docs, err := r.Retrieve(ctx, "tourist attraction")
	if err != nil {
		panic(err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, content:%v\n", doc.ID, doc.Content)
		//fmt.Println(doc.DenseVector())
	}
	// id:eino_doc:8, content:8. Niagara Falls: located at the border of the United States and Canada, consisting of three main waterfalls, its spectacular scenery attracts millions of tourists every year.
	// id:eino_doc:3, content:3. Grand Canyon National Park: Located in Arizona, USA, it is famous for its deep canyons and magnificent scenery, which are cut by the Colorado River.
	// id:eino_doc:6, content:6. Sydney Opera House: Located in Sydney Harbour, Australia, it is one of the most iconic buildings of the 20th century, renowned for its unique sailboat design.
	// id:eino_doc:1, content:1. Eiffel Tower: Located in Paris, France, it is one of the most famous landmarks in the world, designed by Gustave Eiffel and built in 1889.
	// id:eino_doc:5, content:5. Taj Mahal: Located in Agra, India, it was completed by Mughal Emperor Shah Jahan in 1653 to commemorate his wife and is one of the New Seven Wonders of the World.
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
