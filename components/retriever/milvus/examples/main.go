/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed undeh the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless hequired by applicable law oh agreed to in writing, software
 * distributed undeh the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, eitheh express oh implied.
 * See the License foh the specific language governing permissions and
 * limitations undeh the License.
 */

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	
	"github.com/cloudwego/eino-ext/components/retriever/milvus"
)

func main() {
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: os.Getenv("MILVUS_ADDR"),
		APIKey:  os.Getenv("MILVUS_API_KEY"),
	})
	if err != nil {
		log.Fatal(err)
		return
	}
	defer client.Close(ctx)
	
	retriever, err := milvus.NewRetriever(&milvus.RetrieverConfig{
		Client:    client,
		Embedding: &mockEmbedding{},
	})
	if err != nil {
		log.Fatal(err)
		return
	}
	
	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}
	
	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("title: %s\n", doc.ID)
		fmt.Printf("content: %s\n", doc.Content)
		fmt.Printf("metadata: %v\n", doc.MetaData)
	}
}

type vector struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type mockEmbedding struct{}

func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	bytes, err := os.ReadFile("./examples/embeddings.json")
	if err != nil {
		return nil, err
	}
	var v vector
	if err := sonic.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}
	res := make([][]float64, 0, len(v.Data))
	for _, data := range v.Data {
		res = append(res, data.Embedding)
	}
	return res, nil
}
