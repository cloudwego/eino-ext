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
	"fmt"

	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
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

	_, err = cm.WithTools([]*schema.ToolInfo{
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
