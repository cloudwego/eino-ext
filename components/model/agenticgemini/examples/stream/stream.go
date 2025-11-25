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

	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	stream, err := cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Write a short poem about spring."),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	var messages []*schema.AgenticMessage

	fmt.Println("\nAssistant: ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}

		fmt.Printf("frame:\n%s\n\n", resp.String())
		messages = append(messages, resp)
	}

	m, err := schema.ConcatAgenticMessages(messages)
	if err != nil {
		log.Fatalf("ConcatAgenticMessages failed, err=%v", err)
	}
	fmt.Printf("concat message:\n%s\n", m.String())

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Write a short poem about spring."),
		m,
		schema.UserAgenticMessage("Please analyze this poem"),
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	messages = []*schema.AgenticMessage{}
	fmt.Println("\nAssistant: ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}

		fmt.Printf("frame:\n%s\n\n", resp.String())
		messages = append(messages, resp)
	}

	m, err = schema.ConcatAgenticMessages(messages)
	if err != nil {
		log.Fatalf("ConcatAgenticMessages failed, err=%v", err)
	}
	fmt.Printf("concat message:\n%s\n", m.String())
}
