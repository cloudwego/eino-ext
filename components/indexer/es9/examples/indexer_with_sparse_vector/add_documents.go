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
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v9"

	"github.com/cloudwego/eino-ext/components/indexer/es9"
)

const (
	indexName                = "eino_example_sparse"
	fieldContent             = "content"
	fieldContentDenseVector  = "content_dense_vector"
	fieldContentSparseVector = "content_sparse_vector"
	fieldExtraLocation       = "location"
	docExtraLocation         = "location"
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

	// create index if needed.
	// comment out the code if index has been created.
	if err = createIndex(ctx, client); err != nil {
		log.Fatalf("createIndex of es9 failed, err=%v", err)
	}

	// load embeddings from local
	emb, err := prepareEmbeddings()
	if err != nil {
		log.Fatalf("prepareEmbeddings failed, err=%v", err)
	}

	// load docs, set sparse vector
	docs := prepareDocs(emb)

	// create es indexer component
	indexer, err := es9.NewIndexer(ctx, &es9.IndexerConfig{
		Client:    client,
		Index:     indexName,
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es9.FieldValue, err error) {
			return map[string]es9.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentDenseVector, // vectorize doc content and save vector to field "content_vector"
				},
				fieldContentSparseVector: {
					Value: doc.SparseVector(), // load sparse vector from doc metadata
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		Embedding: &mockEmbedding{
			denseMap: func() map[string][]float64 {
				m := make(map[string][]float64)
				for i, doc := range docs {
					m[doc.Content] = emb.Dense[i]
				}
				return m
			}(),
		},
	})
	if err != nil {
		log.Fatalf("NewIndexer of es9 failed, err=%v", err)
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Store of es9 failed, err=%v", err)
	}

	fmt.Println(ids) // [1 2 3 4 5 6 7 8 9 10]
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

func prepareDocs(emb *localEmbeddings) []*schema.Document {
	var docs []*schema.Document
	contents := `1. Eiffel Tower: Located in Paris, France, it is one of the most famous landmarks in the world, designed by Gustave Eiffel and built in 1889.
2. The Great Wall: Located in China, it is one of the Seven Wonders of the World, built from the Qin Dynasty to the Ming Dynasty, with a total length of over 20000 kilometers.
3. Grand Canyon National Park: Located in Arizona, USA, it is famous for its deep canyons and magnificent scenery, which are cut by the Colorado River.
4. The Colosseum: Located in Rome, Italy, built between 70-80 AD, it was the largest circular arena in the ancient Roman Empire.
5. Taj Mahal: Located in Agra, India, it was completed by Mughal Emperor Shah Jahan in 1653 to commemorate his wife and is one of the New Seven Wonders of the World.
6. Sydney Opera House: Located in Sydney Harbour, Australia, it is one of the most iconic buildings of the 20th century, renowned for its unique sailboat design.
7. Louvre Museum: Located in Paris, France, it is one of the largest museums in the world with a rich collection, including Leonardo da Vinci's Mona Lisa and Greece's Venus de Milo.
8. Niagara Falls: located at the border of the United States and Canada, consisting of three main waterfalls, its spectacular scenery attracts millions of tourists every year.
9. St. Sophia Cathedral: located in Istanbul, Türkiye, originally built in 537 A.D., it used to be an Orthodox cathedral and mosque, and now it is a museum.
10. Machu Picchu: an ancient Inca site located on the plateau of the Andes Mountains in Peru, one of the New Seven Wonders of the World, with an altitude of over 2400 meters.`
	locations := []string{"France", "China", "USA", "Italy", "India", "Australia", "France", "Border of the United States and Canada", "Turkey", "Peru"}

	for i, content := range strings.Split(contents, "\n") {
		doc := &schema.Document{
			ID:      strconv.FormatInt(int64(i+1), 10),
			Content: content,
			MetaData: map[string]any{
				docExtraLocation: locations[i],
			},
		}
		doc.WithSparseVector(emb.Sparse[i])
		docs = append(docs, doc)
	}

	return docs
}

func of[T any](v T) *T {
	return &v
}

// mockEmbedding returns embeddings with 1024 dimensions
type mockEmbedding struct {
	denseMap map[string][]float64
}

func (m mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vectors := make([][]float64, 0, len(texts))
	for _, text := range texts {
		vectors = append(vectors, m.denseMap[text])
	}
	return vectors, nil
}
