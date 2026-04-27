//go:build integration

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

package minimax

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func getAPIKey(t *testing.T) string {
	key := os.Getenv("MINIMAX_API_KEY")
	if key == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping integration test")
	}
	return key
}

func TestIntegrationGenerate(t *testing.T) {
	apiKey := getAPIKey(t)
	ctx := context.Background()

	m, err := NewChatModel(ctx, &Config{
		APIKey:  apiKey,
		Model:   "MiniMax-M2.7",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful assistant. Reply concisely."),
		{
			Role:    schema.User,
			Content: "What is 2+2? Reply with just the number.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Role != schema.Assistant {
		t.Fatalf("expected assistant role, got %s", result.Role)
	}
	if result.Content == "" {
		t.Fatal("expected non-empty content")
	}
	t.Logf("Generate response: %s", result.Content)

	if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
		t.Logf("Token usage - prompt: %d, completion: %d, total: %d",
			result.ResponseMeta.Usage.PromptTokens,
			result.ResponseMeta.Usage.CompletionTokens,
			result.ResponseMeta.Usage.TotalTokens)
	}
}

func TestIntegrationStream(t *testing.T) {
	apiKey := getAPIKey(t)
	ctx := context.Background()

	m, err := NewChatModel(ctx, &Config{
		APIKey:  apiKey,
		Model:   "MiniMax-M2.7",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := m.Stream(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful assistant. Reply concisely."),
		{
			Role:    schema.User,
			Content: "Count from 1 to 5.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	chunks := 0
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		chunks++
		t.Logf("Stream chunk %d: %s", chunks, msg.Content)
	}

	if chunks == 0 {
		t.Fatal("expected at least one stream chunk")
	}
	t.Logf("Received %d stream chunks", chunks)
}

func TestIntegrationHighspeedModel(t *testing.T) {
	apiKey := getAPIKey(t)
	ctx := context.Background()

	m, err := NewChatModel(ctx, &Config{
		APIKey:  apiKey,
		Model:   "MiniMax-M2.7-highspeed",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := m.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Say hello in one word.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Content == "" {
		t.Fatal("expected non-empty content from highspeed model")
	}
	t.Logf("Highspeed model response: %s", result.Content)
}
