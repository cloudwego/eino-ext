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
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino-ext/components/retriever/es7/search_mode"
)

const (
	indexName          = "eino_example"
	fieldContent       = "content"
	fieldContentVector = "content_vector"
	fieldExtraLocation = "location"
	docExtraLocation   = "location"
)

func main() {
	ctx := context.Background()

	// es supports multiple ways to connect
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})
	if err != nil {
		log.Fatalf("NewClient of es7 failed, err=%v", err)
	}

	emb, err := prepareEmbeddings()
	if err != nil {
		log.Fatalf("prepareEmbeddings failed, err=%v", err)
	}

	r, err := es7.NewRetriever(ctx, &es7.RetrieverConfig{
		Client: client,
		Index:  indexName,
		TopK:   5,
		SearchMode: search_mode.DenseVectorSimilarity(
			search_mode.DenseVectorSimilarityTypeCosineSimilarity,
			fieldContentVector,
		),
		ResultParser: func(ctx context.Context, hit map[string]any) (doc *schema.Document, err error) {
			id, _ := hit["_id"].(string)
			score, _ := hit["_score"].(float64)

			source, ok := hit["_source"].(map[string]any)
			if !ok {
				return &schema.Document{
					ID:       id,
					MetaData: map[string]any{"score": score},
				}, nil
			}

			content, _ := source[fieldContent].(string)
			location, _ := source[fieldExtraLocation].(string)

			doc = &schema.Document{
				ID:      id,
				Content: content,
				MetaData: map[string]any{
					docExtraLocation: location,
				},
			}
			return doc.WithScore(score), nil
		},
		Embedding: &mockEmbedding{emb.Dense},
	})
	if err != nil {
		log.Fatalf("NewRetriever of es7 failed, err=%v", err)
	}

	// search without filter
	docs, err := r.Retrieve(ctx, "tourist attraction")
	if err != nil {
		log.Fatalf("Retrieve of es7 failed, err=%v", err)
	}

	fmt.Println("Without Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
	// Without Filters
	// id:1, score=2.05, location:France, content:1. Eiffel Tower: Located in Paris, France...
	// id:2, score=2.03, location:China, content:2. The Great Wall: Located in China...
	// ...

	// search with filter
	docs, err = r.Retrieve(ctx, "tourist attraction",
		es7.WithFilters([]any{
			map[string]any{
				"term": map[string]any{
					fieldExtraLocation: "China",
				},
			},
		}),
	)
	if err != nil {
		log.Fatalf("Retrieve of es7 failed, err=%v", err)
	}

	fmt.Println("With Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
	// With Filters
	// id:2, score=2.03, location:China, content:2. The Great Wall: Located in China...
}

type localEmbeddings struct {
	Dense  [][]float64       `json:"dense"`
	Sparse []map[int]float64 `json:"sparse"`
}

func prepareEmbeddings() (*localEmbeddings, error) {
	b, err := os.ReadFile("./examples/embeddings.json")
	if err != nil {
		return nil, err
	}

	le := &localEmbeddings{}
	if err = json.Unmarshal(b, le); err != nil {
		return nil, err
	}

	return le, nil
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
