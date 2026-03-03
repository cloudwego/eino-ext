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
	context "context"
	"fmt"
	"math/rand/v2"

	qq "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	qdrant "github.com/qdrant/go-client/qdrant"
)

const Dimension = 4

func main() {
	ctx := context.Background()

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   "localhost",
		Port:                   6334,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		panic(err)
	}

	r, err := qq.NewRetriever(ctx, &qq.Config{
		Client:     client,
		Collection: "test_collection",
		Embedding:  &mockEmbedding{},
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
	}
}

type mockEmbedding struct {
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, Dimension)
		for j := range vec {
			vec[j] = rand.Float64()
		}
		result[i] = vec
	}
	return result, nil
}
