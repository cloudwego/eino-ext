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
	fieldContent     = "content"
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

	// Create retriever with ExactMatch search mode
	// This performs a simple text match query on the specified field
	r, err := es8.NewRetriever(ctx, &es8.RetrieverConfig{
		Client:     client,
		Index:      indexName,
		TopK:       5,
		SearchMode: search_mode.SearchModeExactMatch(fieldContent),
		// Use default result parser
	})
	if err != nil {
		log.Fatalf("NewRetriever of es8 failed, err=%v", err)
	}

	// Search for documents containing "France"
	fmt.Println("=== ExactMatch: Search for 'France' ===")
	docs, err := r.Retrieve(ctx, "France")
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%v, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}

	// Search for documents containing "Wonders of the World"
	fmt.Println("\n=== ExactMatch: Search for 'Wonders of the World' ===")
	docs, err = r.Retrieve(ctx, "Wonders of the World")
	if err != nil {
		log.Fatalf("Retrieve of es8 failed, err=%v", err)
	}

	for _, doc := range docs {
		fmt.Printf("id:%s, score=%.2f, location:%v, content:%v\n",
			doc.ID, doc.Score(), doc.MetaData[docExtraLocation], doc.Content)
	}
}
