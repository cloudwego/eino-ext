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
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
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
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")

	cert, err := os.ReadFile(httpCACertPath)
	if err != nil {
		log.Fatalf("read file failed, err=%v", err)
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})
	if err != nil {
		log.Fatalf("NewClient of es8 failed, err=%v", err)
	}

	emb, err := prepareEmbeddings()
	if err != nil {
		log.Fatalf("prepareEmbeddings failed, err=%v", err)
	}

	r, err := es8.NewRetriever(ctx, &es8.RetrieverConfig{
		Client: client,
		Index:  indexName,
		TopK:   5,
		SearchMode: search_mode.SearchModeDenseVectorSimilarity(
			search_mode.DenseVectorSimilarityTypeCosineSimilarity,
			fieldContentVector,
		),
		ResultParser: func(ctx context.Context, hit types.Hit) (doc *schema.Document, err error) {
			doc = &schema.Document{
				ID:       *hit.Id_,
				Content:  "",
				MetaData: map[string]any{},
			}

			var src map[string]any
			if err = json.Unmarshal(hit.Source_, &src); err != nil {
				return nil, err
			}

			for field, val := range src {
				switch field {
				case fieldContent:
					doc.Content = val.(string)

				case fieldContentVector:
					var v []float64
					for _, item := range val.([]any) {
						v = append(v, item.(float64))
					}
					doc.WithDenseVector(v)

				case fieldExtraLocation:
					doc.MetaData[docExtraLocation] = val.(string)

				default:
					return nil, fmt.Errorf("unexpected field=%s, val=%v", field, val)
				}
			}

			if hit.Score_ != nil {
				doc.WithScore(float64(*hit.Score_))
			}

			return doc, nil
		},
		Embedding: &mockEmbedding{emb.Dense},
	})

	// search without filter
	docs, err := r.Retrieve(ctx, "tourist attraction")
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	fmt.Println("Without Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
		// fmt.Println(doc.DenseVector())
	}
	// Without Filters
	// id:1, score=1.05, location:France, content:1. Eiffel Tower: Located in Paris, France, it is one of the most famous landmarks in the world, designed by Gustave Eiffel and built in 1889.
	// id:2, score=1.03, location:China, content:2. The Great Wall: Located in China, it is one of the Seven Wonders of the World, built from the Qin Dynasty to the Ming Dynasty, with a total length of over 20000 kilometers.
	// id:5, score=1.02, location:India, content:5. Taj Mahal: Located in Agra, India, it was completed by Mughal Emperor Shah Jahan in 1653 to commemorate his wife and is one of the New Seven Wonders of the World.
	// id:7, score=1.02, location:France, content:7. Louvre Museum: Located in Paris, France, it is one of the largest museums in the world with a rich collection, including Leonardo da Vinci's Mona Lisa and Greece's Venus de Milo.
	// id:6, score=1.02, location:Australia, content:6. Sydney Opera House: Located in Sydney Harbour, Australia, it is one of the most iconic buildings of the 20th century, renowned for its unique sailboat design.

	// search with filter
	docs, err = r.Retrieve(ctx, "tourist attraction",
		es8.WithFilters([]types.Query{
			{
				Term: map[string]types.TermQuery{
					fieldExtraLocation: {
						CaseInsensitive: of(true),
						Value:           "China",
					},
				},
			},
		}),
	)
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	fmt.Println("With Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
		// fmt.Println(doc.DenseVector())
	}
	// With Filters
	// id:2, score=1.03, location:China, content:2. The Great Wall: Located in China, it is one of the Seven Wonders of the World, built from the Qin Dynasty to the Ming Dynasty, with a total length of over 20000 kilometers.
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

func of[T any](t T) *T {
	return &t
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	dense [][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	return m.dense, nil
}
