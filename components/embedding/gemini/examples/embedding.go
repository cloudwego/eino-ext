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
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	"google.golang.org/genai"
)

func buildClient(ctx context.Context) *genai.Client {
	// set via the GOOGLE_API_KEY or GEMINI_API_KEY environment variable
	cli, err := genai.NewClient(ctx, &genai.ClientConfig{})
	if err != nil {
		log.Fatal("create genai client error: ", err)
	}
	return cli
}

func main() {
	ctx := context.Background()

	cli := buildClient(ctx)

	embedder, err := gemini.NewEmbedder(ctx, &gemini.EmbeddingConfig{
		Client:   cli,
		Model:    "gemini-embedding-001", // Or other models like "text-embedding-004"
		TaskType: "RETRIEVAL_QUERY",      // Or other task types like "EmbeddingTaskTypeImage"
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return
	}

	embedding, err := embedder.EmbedStrings(ctx, []string{"hello world", "你好世界"})
	if err != nil {
		log.Printf("embedding error: %v\n", err)
		return
	}

	log.Printf("embedding: %v\n", embedding)
}
