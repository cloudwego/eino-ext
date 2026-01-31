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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

// Mock weather function
func getWeather(location string) string {
	// In a real application, this would call a weather API
	return fmt.Sprintf("The weather in %s is sunny, 25°C", location)
}

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		log.Fatal("ZHIPU_API_KEY environment variable not set")
	}

	// Create chat model
	config := &zhipu.ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash", // Free model
	}

	chatModel, err := zhipu.NewChatModel(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create chat model: %v", err)
	}

	// Define tools
	tools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get weather information for a specified city",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"location": {
					Type: "string",
					Desc: "City name, e.g., Beijing, Shanghai",
				},
			}),
		},
	}

	// Bind tools to the model
	chatModelWithTools, err := chatModel.WithTools(tools)
	if err != nil {
		log.Fatalf("Failed to bind tools: %v", err)
	}

	// Prepare messages
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's the weather like in Beijing today?",
		},
	}

	// Generate response
	resp, err := chatModelWithTools.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to generate: %v", err)
	}

	fmt.Println("First Response:")
	fmt.Printf("Content: %s\n", resp.Content)

	// Check if the model wants to call a tool
	if len(resp.ToolCalls) > 0 {
		fmt.Println("\nTool Calls:")
		for _, toolCall := range resp.ToolCalls {
			fmt.Printf("  - Function: %s\n", toolCall.Function.Name)
			fmt.Printf("    Arguments: %s\n", toolCall.Function.Arguments)

			// Execute the tool
			if toolCall.Function.Name == "get_weather" {
				var args map[string]string
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					log.Fatalf("Failed to parse arguments: %v", err)
				}

				result := getWeather(args["location"])
				fmt.Printf("    Result: %s\n", result)

				// Add the assistant's response and tool result to messages
				messages = append(messages, resp)
				messages = append(messages, &schema.Message{
					Role:       schema.Tool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}

		// Get final response after tool execution
		fmt.Println("\nFinal Response:")
		finalResp, err := chatModelWithTools.Generate(ctx, messages)
		if err != nil {
			log.Fatalf("Failed to generate final response: %v", err)
		}

		fmt.Printf("Content: %s\n", finalResp.Content)
	} else {
		fmt.Println("\nNo tool calls were made.")
	}
}
