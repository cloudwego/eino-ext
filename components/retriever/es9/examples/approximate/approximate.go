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
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/cloudwego/eino-ext/components/retriever/es9"
	"github.com/cloudwego/eino-ext/components/retriever/es9/search_mode"
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
		log.Fatalf("NewClient of es9 failed, err=%v", err)
	}

	emb, err := prepareEmbeddings()
	if err != nil {
		log.Fatalf("prepareEmbeddings failed, err=%v", err)
	}

	r, err := es9.NewRetriever(ctx, &es9.RetrieverConfig{
		Client: client,
		Index:  indexName,
		TopK:   5,
		SearchMode: search_mode.SearchModeApproximate(&search_mode.ApproximateConfig{
			QueryFieldName:  fieldContent,
			VectorFieldName: fieldContentVector,
			Hybrid:          true,
			// RRF only available with specific licenses
			// see: https://www.elastic.co/subscriptions
			RRF:             false,
			RRFRankConstant: nil,
			RRFWindowSize:   nil,
			K:               of(10),
			NumCandidates:   of(100),
		}),
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
		log.Fatalf("Retrieve of es9 failed, err=%v", err)
	}

	fmt.Println("Without Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}

	// search with filter
	docs, err = r.Retrieve(ctx, "tourist attraction",
		es9.WithFilters([]types.Query{
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
		log.Fatalf("Retrieve of es9 failed, err=%v", err)
	}

	fmt.Println("With Filters")
	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
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
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = m.dense[0]
	}
	return vectors, nil
}
