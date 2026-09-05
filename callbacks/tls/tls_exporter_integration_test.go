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

package tls

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// TestTLSExporterE2E sends one complete agent.turn -> llm.request -> tool.call
// trace to a real TLS Trace Topic. It is opt-in so normal package tests never
// require credentials or network access.
func TestTLSExporterE2E(t *testing.T) {
	if os.Getenv("TLS_EXPORTER_E2E") != "1" {
		t.Skip("set TLS_EXPORTER_E2E=1 with TLS_LOG_* credentials to run")
	}

	cfg, err := LoadTLSConfigFromEnv()
	if err != nil {
		t.Fatalf("load TLS exporter config: %v", err)
	}
	handler, shutdown, err := NewTLSCallbackHandler(cfg)
	if err != nil {
		t.Fatalf("create TLS exporter handler: %v", err)
	}
	t.Cleanup(func() {
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	})

	h, ok := handler.(*TLSCallbackHandler)
	if !ok {
		t.Fatalf("handler type = %T, want *TLSCallbackHandler", handler)
	}
	sessionID := fmt.Sprintf("eino-tls-exporter-e2e-%d", time.Now().UnixNano())
	rootInfo := &callbacks.RunInfo{Component: compose.ComponentOfLambda, Name: "eino-e2e-agent"}
	rootCtx := h.OnStart(SetSession(context.Background(), WithSessionID(sessionID)), rootInfo, "TLS exporter e2e input")

	modelInfo := &callbacks.RunInfo{Component: components.ComponentOfChatModel, Name: "eino-e2e-model"}
	modelCtx := h.OnStart(rootCtx, modelInfo, &model.CallbackInput{
		Messages: []*schema.Message{{Role: schema.User, Content: "TLS exporter e2e input"}},
		Config:   &model.Config{Model: "eino-e2e-model"},
	})
	h.OnEnd(modelCtx, modelInfo, &model.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "TLS exporter e2e output"},
		TokenUsage: &model.TokenUsage{
			PromptTokens:     17,
			CompletionTokens: 8,
			TotalTokens:      25,
		},
	})

	toolInfo := &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "eino-e2e-tool"}
	toolCtx := h.OnStart(rootCtx, toolInfo, &tool.CallbackInput{})
	h.OnEnd(toolCtx, toolInfo, &tool.CallbackOutput{})
	h.OnEnd(rootCtx, rootInfo, "TLS exporter e2e done")

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("flush TLS exporter: %v", err)
	}
	shutdown = nil
	t.Logf("exported Eino trace session %s", sessionID)
}

// TestTLSExporterE2EMultimodalMatrix writes one real Trace Topic record set
// through the same handler applications register. It intentionally uses only
// public example URLs: the exporter records media metadata; it does not fetch
// or upload the media itself.
func TestTLSExporterE2EMultimodalMatrix(t *testing.T) {
	if os.Getenv("TLS_EXPORTER_E2E") != "1" {
		t.Skip("set TLS_EXPORTER_E2E=1 with TLS_LOG_* credentials to run")
	}
	cfg, err := LoadTLSConfigFromEnv()
	if err != nil {
		t.Fatalf("load TLS exporter config: %v", err)
	}
	handler, shutdown, err := NewTLSCallbackHandler(cfg)
	if err != nil {
		t.Fatalf("create TLS exporter handler: %v", err)
	}
	t.Cleanup(func() {
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	})
	h, ok := handler.(*TLSCallbackHandler)
	if !ok {
		t.Fatalf("handler type = %T, want *TLSCallbackHandler", handler)
	}

	imageURL := "https://example.com/eino-e2e-image.png"
	videoURL := "https://example.com/eino-e2e-video.mp4"
	sessionID := fmt.Sprintf("eino-tls-matrix-%d", time.Now().UnixNano())
	rootInfo := &callbacks.RunInfo{Component: compose.ComponentOfLambda, Name: "eino-e2e-matrix"}
	rootCtx := h.OnStart(SetSession(context.Background(), WithSessionID(sessionID)), rootInfo, "Eino E2E matrix")
	modelInfo := &callbacks.RunInfo{Component: components.ComponentOfChatModel, Name: "eino-e2e-model"}

	// Text, non-streaming.
	textCtx := h.OnStart(rootCtx, modelInfo, &model.CallbackInput{Messages: []*schema.Message{{Role: schema.User, Content: "E2E text non-stream"}}, Config: &model.Config{Model: "eino-e2e-model"}})
	h.OnEnd(textCtx, modelInfo, &model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "E2E text non-stream result"}, TokenUsage: &model.TokenUsage{PromptTokens: 5, CompletionTokens: 4, TotalTokens: 9}})

	// Image + video + text, non-streaming, followed by a real tool-call-shaped
	// assistant response and the matching tool callback.
	multimodal := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "E2E multimodal non-stream"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imageURL, MIMEType: "image/png"}}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &videoURL, MIMEType: "video/mp4"}}},
	}}
	multiCtx := h.OnStart(rootCtx, modelInfo, &model.CallbackInput{Messages: []*schema.Message{multimodal}, Config: &model.Config{Model: "eino-e2e-model"}})
	h.OnEnd(multiCtx, modelInfo, &model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call-e2e-weather", Type: "function", Function: schema.FunctionCall{Name: "weather", Arguments: `{"city":"Guilin"}`}}}}, TokenUsage: &model.TokenUsage{PromptTokens: 12, CompletionTokens: 5, TotalTokens: 17}})
	toolInfo := &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}
	toolCtx := h.OnStart(rootCtx, toolInfo, &tool.CallbackInput{ArgumentsInJSON: `{"city":"Guilin"}`})
	h.OnEnd(toolCtx, toolInfo, &tool.CallbackOutput{Response: `{"temperature":25,"unit":"C"}`})

	// Image + video + text, streaming.
	inReader, inWriter := schema.Pipe[callbacks.CallbackInput](1)
	streamCtx := h.OnStartWithStreamInput(rootCtx, modelInfo, inReader)
	inWriter.Send(&model.CallbackInput{Messages: []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "E2E multimodal stream"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imageURL}}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &videoURL}}},
	}}}, Config: &model.Config{Model: "eino-e2e-model"}}, nil)
	inWriter.Close()
	outReader, outWriter := schema.Pipe[callbacks.CallbackOutput](2)
	h.OnEndWithStreamOutput(streamCtx, modelInfo, outReader)
	outWriter.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "E2E stream "}}, nil)
	outWriter.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "result"}, TokenUsage: &model.TokenUsage{PromptTokens: 13, CompletionTokens: 6, TotalTokens: 19}}, nil)
	outWriter.Close()

	// Streaming tool input/output gives the exporter fragmented real payloads.
	toolInReader, toolInWriter := schema.Pipe[callbacks.CallbackInput](2)
	streamToolCtx := h.OnStartWithStreamInput(rootCtx, toolInfo, toolInReader)
	toolInWriter.Send(&tool.CallbackInput{ArgumentsInJSON: `{"city":`}, nil)
	toolInWriter.Send(&tool.CallbackInput{ArgumentsInJSON: `"Guilin"}`}, nil)
	toolInWriter.Close()
	toolOutReader, toolOutWriter := schema.Pipe[callbacks.CallbackOutput](2)
	h.OnEndWithStreamOutput(streamToolCtx, toolInfo, toolOutReader)
	toolOutWriter.Send(&tool.CallbackOutput{Response: `{"temperature":`}, nil)
	toolOutWriter.Send(&tool.CallbackOutput{Response: `25}`}, nil)
	toolOutWriter.Close()

	// The callback tracks child stream parsers, so ending the root waits for
	// their final model and tool attributes without a timing-dependent sleep.
	h.OnEnd(rootCtx, rootInfo, "Eino E2E matrix done")
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("flush TLS exporter: %v", err)
	}
	shutdown = nil
	t.Logf("exported Eino multimodal matrix session %s", sessionID)
}
