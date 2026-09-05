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

// Command tls-eino-demo runs a real Eino graph and exports its traces to
// Volcengine TLS through the eino-ext TLS callback handler.
//
// Unlike a handcrafted OnStart/OnEnd sequence, this demo builds an actual
// compose.Chain (prompt template -> chat model) and invokes it. The TLS
// handler is registered as a global Eino callback, so the agent.turn /
// llm.request spans are produced by the framework itself — exactly what a
// production integration looks like.
//
// Configure it with the environment variables below and run it:
//
//	export TLS_EXPORTER_ENABLED=true
//	export TLS_APP_NAME=tls-eino-demo
//	export TLS_LOG_ENDPOINT=tls-cn-beijing2.volces.com   # optional, defaults from region
//	export TLS_LOG_REGION=cn-beijing2
//	export TLS_LOG_TRACE_TOPIC_ID=<your-trace-topic-id>
//	export TLS_LOG_API_KEY=<your-tls-api-key>            # or TLS_LOG_AK + TLS_LOG_SK
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/callbacks/tls"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	log.SetFlags(log.Ltime)
	log.Println("=== Eino TLS Callback Demo (real graph) ===")

	cfg, err := loadConfig()
	if err != nil {
		printUsage()
		log.Fatalf("invalid TLS configuration: %v", err)
	}

	ctx := context.Background()

	// Session attributes are attached to the context and surface on the
	// agent.turn span so the TLS LLM dashboard can group traces per session.
	sessionID := fmt.Sprintf("demo-session-%d", time.Now().UnixMilli())
	userID := "demo-user"
	ctx = tls.SetSession(ctx, tls.WithSessionID(sessionID), tls.WithUserID(userID))
	log.Printf("[demo] session_id=%s user_id=%s", sessionID, userID)

	// Build the TLS handler from the resolved config and register it as a
	// global callback so every node in the graph is traced automatically.
	handler, shutdown, err := tls.NewTLSHandler(cfg)
	if err != nil {
		log.Fatalf("failed to create TLS handler: %v", err)
	}
	defer func() {
		log.Println("[demo] flushing traces to TLS...")
		flushCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := shutdown(flushCtx); err != nil {
			log.Printf("[demo] shutdown error: %v", err)
		}
		log.Println("[demo] done. Check the TLS LLM dashboard for session:", sessionID)
	}()

	callbacks.AppendGlobalHandlers(handler)
	log.Println("[demo] TLS handler registered as global Eino callback")

	// Build a real Eino graph: chat prompt template -> chat model.
	chain, err := buildChain(ctx)
	if err != nil {
		log.Fatalf("failed to build chain: %v", err)
	}

	// --- Demo 1: non-streaming invoke ---
	log.Println("\n--- Demo 1: Chain.Invoke (non-streaming) ---")
	out, err := chain.Invoke(ctx, map[string]any{
		"role":     "helpful assistant",
		"question": "What is Go in one sentence?",
	})
	if err != nil {
		log.Fatalf("chain invoke failed: %v", err)
	}
	log.Printf("[demo] answer: %s", out.Content)

	// --- Demo 2: streaming invoke ---
	log.Println("\n--- Demo 2: Chain.Stream (streaming) ---")
	stream, err := chain.Stream(ctx, map[string]any{
		"role":     "poet",
		"question": "Write one short line about goroutines.",
	})
	if err != nil {
		log.Fatalf("chain stream failed: %v", err)
	}
	streamed := collectStream(stream)
	log.Printf("[demo] streamed answer: %s", streamed)

	// Give the batch span processor a moment before the deferred flush runs.
	time.Sleep(300 * time.Millisecond)
	log.Println("\n[demo] all graph runs submitted")
}

// buildChain wires a chat prompt template to a mock chat model. Both nodes emit
// Eino callbacks, which the global TLS handler turns into spans.
func buildChain(ctx context.Context) (compose.Runnable[map[string]any, *schema.Message], error) {
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("You are a {role}. Answer concisely."),
		schema.UserMessage("{question}"),
	)

	chain := compose.NewChain[map[string]any, *schema.Message]().
		AppendChatTemplate(template).
		AppendChatModel(newMockChatModel("doubao-pro-32k"))

	return chain.Compile(ctx)
}

// loadConfig resolves the TLS config from environment variables and fills in a
// sensible default endpoint when TLS_LOG_ENDPOINT is left empty.
func loadConfig() (*tls.TLSConfig, error) {
	if os.Getenv("TLS_EXPORTER_ENABLED") != "true" {
		return nil, errors.New("TLS_EXPORTER_ENABLED must be 'true'")
	}

	cfg := &tls.TLSConfig{
		AppName:               getenvOr("TLS_APP_NAME", "tls-eino-demo"),
		TLSExporterEnabled:    true,
		TLSLogEndpoint:        os.Getenv("TLS_LOG_ENDPOINT"),
		TLSLogRegion:          os.Getenv("TLS_LOG_REGION"),
		TLSLogTopicID:         os.Getenv("TLS_LOG_TRACE_TOPIC_ID"),
		TLSLogAPIKey:          os.Getenv("TLS_LOG_API_KEY"),
		TLSLogAccessKeyID:     os.Getenv("TLS_LOG_AK"),
		TLSLogAccessKeySecret: os.Getenv("TLS_LOG_SK"),
	}

	// The exporter requires a non-empty endpoint. When only the region is
	// provided, derive the standard TLS endpoint for that region.
	if cfg.TLSLogEndpoint == "" && cfg.TLSLogRegion != "" {
		cfg.TLSLogEndpoint = fmt.Sprintf("tls-%s.volces.com", cfg.TLSLogRegion)
		log.Printf("[demo] TLS_LOG_ENDPOINT empty, defaulting to %s", cfg.TLSLogEndpoint)
	}

	return cfg, tls.ValidateTLSConfig(cfg)
}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func collectStream(stream *schema.StreamReader[*schema.Message]) string {
	defer stream.Close()
	var content string
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Printf("[demo] stream recv error: %v", err)
			break
		}
		content += chunk.Content
	}
	return content
}

func printUsage() {
	log.Println("Set the following environment variables before running:")
	log.Println("  export TLS_EXPORTER_ENABLED=true")
	log.Println("  export TLS_APP_NAME=tls-eino-demo")
	log.Println("  export TLS_LOG_ENDPOINT=tls-cn-beijing2.volces.com   # optional, derived from region")
	log.Println("  export TLS_LOG_REGION=cn-beijing2")
	log.Println("  export TLS_LOG_TRACE_TOPIC_ID=<your-trace-topic-id>")
	log.Println("  export TLS_LOG_API_KEY=<your-tls-api-key>            # or TLS_LOG_AK + TLS_LOG_SK")
}

// mockChatModel is a stand-in ChatModel so the demo runs without real
// credentials for an LLM provider. Swap it for a real model (e.g.
// components/model/ark) and the TLS spans stay identical.
type mockChatModel struct {
	name string
}

func newMockChatModel(name string) model.BaseChatModel {
	return &mockChatModel{name: name}
}

func (m *mockChatModel) reply(input []*schema.Message) *schema.Message {
	var question string
	if len(input) > 0 {
		question = input[len(input)-1].Content
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: fmt.Sprintf("[%s] mock reply to: %s", m.name, question),
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     25,
				CompletionTokens: 12,
				TotalTokens:      37,
			},
		},
	}
}

func (m *mockChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.reply(input), nil
}

func (m *mockChatModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	full := m.reply(input)
	words := []string{"Hello", " from", " the", " mock", " streaming", " model."}

	sr, sw := schema.Pipe[*schema.Message](len(words) + 1)
	go func() {
		defer sw.Close()
		for _, w := range words {
			sw.Send(&schema.Message{Role: schema.Assistant, Content: w}, nil)
		}
		// Final chunk carries token usage, mirroring real providers.
		sw.Send(&schema.Message{Role: schema.Assistant, ResponseMeta: full.ResponseMeta}, nil)
	}()
	return sr, nil
}
