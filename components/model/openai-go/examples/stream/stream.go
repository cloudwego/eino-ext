/*
 * Copyright 2026 CloudWeGo Authors
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
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	openaigo "github.com/cloudwego/eino-ext/components/model/openai-go"
)

func main() {
	ctx := context.Background()

	cm, err := openaigo.NewChatModel(ctx, &openaigo.Config{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	stream, err := cm.Stream(ctx, []*schema.Message{
		{Role: schema.User, Content: "Write a short poem about spring."},
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	fmt.Println("Assistant:")
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}

		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
		if chunk.ReasoningContent != "" {
			fmt.Printf("\n[reasoning]\n%s\n", chunk.ReasoningContent)
		}
	}
	fmt.Println()
}
