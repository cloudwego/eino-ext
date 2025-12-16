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
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino-ext/components/retriever/es7/search_mode"
)

const (
	indexName          = "eino_example"
	fieldContent       = "content"
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

	// Create retriever with ExactMatch search mode
	// This performs a simple text match query on the specified field
	r, err := es7.NewRetriever(ctx, &es7.RetrieverConfig{
		Client:     client,
		Index:      indexName,
		TopK:       5,
		SearchMode: search_mode.ExactMatch(fieldContent),
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
	})
	if err != nil {
		log.Fatalf("NewRetriever of es7 failed, err=%v", err)
	}

	// Search for documents containing "France"
	fmt.Println("=== ExactMatch: Search for 'France' ===")
	docs, err := r.Retrieve(ctx, "France")
	if err != nil {
		log.Fatalf("Retrieve of es7 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}

	// Search for documents containing "Wonders of the World"
	fmt.Println("\n=== ExactMatch: Search for 'Wonders of the World' ===")
	docs, err = r.Retrieve(ctx, "Wonders of the World")
	if err != nil {
		log.Fatalf("Retrieve of es7 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%s, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
}
