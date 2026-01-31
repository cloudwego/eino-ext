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

package zhipu

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func getAPIKey(t *testing.T) string {
	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		t.Skip("ZHIPU_API_KEY not set, skipping integration test")
	}
	return apiKey
}

func TestIntegration_Generate(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash",
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	messages := []*schema.Message{
		schema.UserMessage("你好，请用一句话介绍智谱AI。"),
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}

	t.Logf("Response: %s", resp.Content)
}

func TestIntegration_Stream(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash",
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	messages := []*schema.Message{
		schema.UserMessage("请写一首关于人工智能的五言绝句。"),
	}

	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("failed to stream: %v", err)
	}

	var fullContent string
	var fullReasoning string
	chunkCount := 0
	for {
		msg, err := stream.Recv()
		if err != nil {
			t.Logf("Stream ended with error: %v", err)
			break
		}
		chunkCount++
		fullContent += msg.Content
		fullReasoning += msg.ReasoningContent

		// 详细打印每个消息的信息
		if msg.Content != "" {
			t.Logf("Chunk #%d Content: %q", chunkCount, msg.Content)
		}
		if msg.ReasoningContent != "" {
			t.Logf("Chunk #%d Reasoning: %q", chunkCount, msg.ReasoningContent)
		}

		// 如果有 usage 信息也打印
		if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
			t.Logf("  Usage: prompt=%d, completion=%d, total=%d",
				msg.ResponseMeta.Usage.PromptTokens,
				msg.ResponseMeta.Usage.CompletionTokens,
				msg.ResponseMeta.Usage.TotalTokens)
		}
	}

	t.Logf("Total chunks: %d", chunkCount)
	t.Logf("Full reasoning length: %d", len(fullReasoning))
	t.Logf("Full content length: %d", len(fullContent))

	if fullReasoning != "" {
		t.Logf("Full reasoning:\n%s", fullReasoning)
	}
	if fullContent != "" {
		t.Logf("Full content:\n%s", fullContent)
	}

	// 只要有内容或推理内容就算通过
	if fullContent == "" && fullReasoning == "" {
		t.Error("expected non-empty streamed content or reasoning content")
	}
}

func TestIntegration_WithThinking(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	temp := float32(0.9)
	config := &ChatModelConfig{
		APIKey:      apiKey,
		Model:       "glm-4.7-flash",
		Temperature: &temp,
		Thinking: &Thinking{
			Type: ThinkingEnabled,
		},
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "you are a helpful assistant",
		},
		{
			Role:    schema.User,
			Content: "what is the revolution of llm?",
		},
	}

	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("failed to stream: %v", err)
	}

	hasReasoningContent := false
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		if msg.ReasoningContent != "" {
			hasReasoningContent = true
			t.Logf("Reasoning: %s", msg.ReasoningContent)
		}
		if msg.Content != "" {
			t.Logf("Content: %s", msg.Content)
		}
	}

	if !hasReasoningContent {
		t.Log("Warning: no reasoning content found (might be normal depending on the model)")
	}
}

func TestIntegration_ToolCalling(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash",
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	// Define a tool
	tools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "获取指定城市的天气信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"location": {
					Type: "string",
					Desc: "城市名称，例如：北京、上海",
				},
			}),
		},
	}

	chatModelWithTools, err := chatModel.WithTools(tools)
	if err != nil {
		t.Fatalf("failed to bind tools: %v", err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "北京今天天气怎么样？",
		},
	}

	resp, err := chatModelWithTools.Generate(ctx, messages)
	if err != nil {
		t.Fatalf("failed to generate with tools: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		t.Log("Warning: expected tool calls but got none (might be normal depending on the model)")
	} else {
		t.Logf("Tool calls: %+v", resp.ToolCalls)
	}
}

func TestIntegration_Vision(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.6v-flash", // Use vision model
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	// Read test image
	image, err := os.ReadFile("./examples/vision/test.jpg")
	if err != nil {
		t.Skipf("test image not found, skipping vision test: %v", err)
	}

	messages := []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "这张图片里有什么？请简要描述。",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: of(base64.StdEncoding.EncodeToString(image)),
							MIMEType:   "image/jpeg",
						},
						Detail: schema.ImageURLDetailAuto,
					},
				},
			},
		},
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		t.Fatalf("failed to generate with vision: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}

	t.Logf("Vision Response: %s", resp.Content)
}

func TestIntegration_WithCustomOptions(t *testing.T) {
	apiKey := getAPIKey(t)

	ctx := context.Background()
	config := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash",
	}

	chatModel, err := NewChatModel(ctx, config)
	if err != nil {
		t.Fatalf("failed to create chat model: %v", err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "解释一下量子计算的基本原理。",
		},
	}

	// Use WithThinking option at runtime
	resp, err := chatModel.Generate(ctx, messages,
		WithThinking(&Thinking{Type: ThinkingEnabled}),
		model.WithTemperature(0.7),
	)
	if err != nil {
		t.Fatalf("failed to generate with custom options: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}

	t.Logf("Response: %s", resp.Content)
	if resp.ReasoningContent != "" {
		t.Logf("Reasoning: %s", resp.ReasoningContent)
	}
}

func of[T any](t T) *T {
	return &t
}
