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
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// 创建 Claude 模型
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	fmt.Println("\n=== Basic Chat ===")
	basicChat(ctx, cm)

	fmt.Println("\n=== Streaming Chat ===")
	streamingChat(ctx, cm)

	fmt.Println("\n=== Function Calling ===")
	functionCalling(ctx, cm)

	fmt.Println("\n=== Image Processing ===")
	imageProcessing(ctx, cm)
}

func basicChat(ctx context.Context, cm model.BaseChatModel) {
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful AI assistant. Be concise in your responses.",
		},
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	}

	resp, err := cm.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	thinking, ok := claude.GetThinking(resp)
	fmt.Printf("Thinking(have: %v): %s\n", ok, thinking)
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
	}
}

func streamingChat(ctx context.Context, cm model.BaseChatModel) {
	messages := []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		{
			Role:    schema.User,
			Content: "Write a short poem about spring, word by word.",
		},
	}

	stream, err := cm.Stream(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Stream error: %v", err)
		return
	}
	isFirstThinking := false
	isFirstContent := false

	fmt.Print("Assistant: ----------\n")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}

		thinkingContent, ok := claude.GetThinking(resp)
		if ok {
			if !isFirstThinking {
				isFirstThinking = true
				fmt.Print("\nThinking: ----------\n")
			}
			fmt.Print(thinkingContent)
		}

		if len(resp.Content) > 0 {
			if !isFirstContent {
				isFirstContent = true
				fmt.Print("\nContent: ----------\n")
			}
			fmt.Print(resp.Content)
		}
	}
	fmt.Println("\n----------")
}

func functionCalling(ctx context.Context, cm model.ToolCallingChatModel) {
	toolModel, err := cm.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get current weather information for a city",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "city",
						Value: &jsonschema.Schema{
							Type:        "string",
							Description: "The city name",
						},
					},
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "unit",
						Value: &jsonschema.Schema{
							Type: "string",
							Enum: []interface{}{"celsius", "fahrenheit"},
						},
					},
				)),
				Required: []string{"city"},
			}),
		},
	})
	if err != nil {
		log.Printf("Bind tools error: %v", err)
		return
	}

	cm = toolModel

	streamResp, err := cm.Stream(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		schema.UserMessage("call 'get_weather' to query what's the weather like in Paris today? Please use Celsius."),
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	msgs := make([]*schema.Message, 0)
	for {
		msg, err := streamResp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}
		msgs = append(msgs, msg)
	}
	resp, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("Concat error: %v", err)
	}

	fmt.Printf("assistant content:\n  %v\n----------\n", resp.Content)
	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: %s\n", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)

		weatherResp, err := cm.Generate(ctx, []*schema.Message{
			schema.UserMessage("What's the weather like in Paris today? Please use Celsius."),
			resp,
			schema.ToolMessage(`{"temperature": 18, "condition": "sunny"}`, resp.ToolCalls[0].ID),
		})
		if err != nil {
			log.Printf("Generate error: %v", err)
			return
		}
		fmt.Printf("Final response: %s\n", weatherResp.Content)
	}
}

func imageProcessing(ctx context.Context, cm model.BaseChatModel) {
	imageBinary, err := os.ReadFile("examples/test.jpg")
	if err != nil {
		log.Fatalf("read file failed, err=%v", err)
	}
	base64Str := base64.StdEncoding.EncodeToString(imageBinary)
	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "What do you see in this image?",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &base64Str,
							MIMEType:   "image/jpeg",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}
