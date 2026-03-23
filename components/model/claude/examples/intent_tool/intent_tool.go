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
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

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

	// Create a Claude model
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

	fmt.Printf("\n==========text tool call==========\n")
	textToolCall(ctx, cm)
	fmt.Printf("\n==========image tool call==========\n")
	imageToolCall(ctx, cm)
}

func textToolCall(ctx context.Context, chatModel model.ToolCallingChatModel) {
	chatModel, err := chatModel.WithTools([]*schema.ToolInfo{
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
		log.Fatalf("Bind tools error: %v", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		schema.UserMessage("call 'get_weather' to query what's the weather like in Paris today? Please use Celsius."),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
		return
	}
	fmt.Printf("output: %v", resp)
}

func imageToolCall(ctx context.Context, chatModel model.ToolCallingChatModel) {
	image, err := os.ReadFile("./examples/intent_tool/img.png")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	imageStr := base64.StdEncoding.EncodeToString(image)

	chatModel, err = chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "image_generator",
			Desc: "a tool can generate images",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"description": {
						Type: "string",
						Desc: "The description of the image",
					},
				}),
		},
	})
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
		return
	}

	query := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful assistant. If the user needs to generate an image, call the image_generator tool to generate the image.",
		},
		{
			Role:    schema.User,
			Content: "Generator a cat image",
		},
	}

	resp, err := chatModel.Generate(ctx, query)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	if len(resp.ToolCalls) == 0 {
		log.Fatalf("No tool calls found")
		return
	}

	fmt.Printf("output: \n%v", resp)

	resp, err = chatModel.Generate(ctx, append(query, resp, &schema.Message{
		Role:       schema.Tool,
		ToolCallID: resp.ToolCalls[0].ID,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Image generation successful.",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &imageStr,
						MIMEType:   "image/png",
					},
				},
			},
		},
	}))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)
}
