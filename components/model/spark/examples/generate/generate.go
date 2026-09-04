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
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/spark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("SPARK_API_KEY")
	if apiKey == "" {
		log.Fatal("SPARK_API_KEY environment variable is not set")
	}

	// Model defaults to 4.0Ultra; override via SPARK_MODEL if desired
	// (valid HTTP models: 4.0Ultra, generalv3.5, max-32k, generalv3, pro-128k, lite).
	cm, err := spark.NewChatModel(ctx, &spark.ChatModelConfig{
		APIKey: apiKey,
		Model:  os.Getenv("SPARK_MODEL"),
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		schema.UserMessage("What is the capital of France?"),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	log.Printf("Response: %s", resp.Content)
}
