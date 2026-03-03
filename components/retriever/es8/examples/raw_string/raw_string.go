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
	"fmt"
	"log"
	"os"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
)

const (
	indexName        = "eino_example"
	docExtraLocation = "location"
)

func main() {
	ctx := context.Background()

	// es supports multiple ways to connect
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")

	var client *elasticsearch.Client
	var err error

	if httpCACertPath != "" {
		cert, err := os.ReadFile(httpCACertPath)
		if err != nil {
			log.Fatalf("read file failed, err=%v", err)
		}
		client, err = elasticsearch.NewClient(elasticsearch.Config{
			Addresses: []string{"https://localhost:9200"},
			Username:  username,
			Password:  password,
			CACert:    cert,
		})
	} else {
		client, err = elasticsearch.NewClient(elasticsearch.Config{
			Addresses: []string{"https://localhost:9200"},
			Username:  username,
			Password:  password,
		})
	}

	if err != nil {
		log.Fatalf("NewClient of es8 failed, err=%v", err)
	}

	// Create retriever with RawStringRequest search mode
	// This allows you to pass a raw JSON query string directly
	r, err := es8.NewRetriever(ctx, &es8.RetrieverConfig{
		Client:     client,
		Index:      indexName,
		TopK:       5,
		SearchMode: search_mode.SearchModeRawStringRequest(),
		// Use default result parser
	})
	if err != nil {
		log.Fatalf("NewRetriever of es8 failed, err=%v", err)
	}

	// Example 1: Simple match query using raw JSON
	// Note: in es8 typed api, raw string request should be a complete valid json request body
	fmt.Println("=== RawStringRequest: Simple match query ===")
	query1 := `{
		"query": {
			"match": {
				"content": "museum"
			}
		}
	}`
	docs, err := r.Retrieve(ctx, query1)
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%v, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}

	// Example 2: Bool query with must and filter
	fmt.Println("\n=== RawStringRequest: Bool query with filter ===")
	query2 := `{
		"query": {
			"bool": {
				"must": [
					{"match": {"content": "World"}}
				],
				"filter": [
					{"term": {"location": "China"}}
				]
			}
		}
	}`
	docs, err = r.Retrieve(ctx, query2)
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%v, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}

	// Example 3: Wildcard query
	fmt.Println("\n=== RawStringRequest: Wildcard query ===")
	query3 := `{
		"query": {
			"wildcard": {
				"content": "*ancient*"
			}
		}
	}`
	docs, err = r.Retrieve(ctx, query3)
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%v, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
}
