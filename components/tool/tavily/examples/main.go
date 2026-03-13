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
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/tavily"
)

func main() {
	// Get the Tavily API key from environment variable
	tavilyAPIKey := os.Getenv("TAVILY_API_KEY")
	if tavilyAPIKey == "" {
		log.Fatal("TAVILY_API_KEY environment variable is not set")
	}

	// Create a context
	ctx := context.Background()

	// Create the Tavily Search tool with advanced options
	tavilyTool, err := tavily.NewTool(ctx, &tavily.Config{
		APIKey:        tavilyAPIKey,
		SearchDepth:   tavily.SearchDepthBasic,
		Topic:         tavily.TopicGeneral,
		MaxResults:    5,
		IncludeAnswer: true,
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Create a search request
	request := &tavily.SearchRequest{
		Query: "What is CloudWeGo Eino framework?",
	}

	jsonReq, err := sonic.Marshal(request)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	// Execute the search
	resp, err := tavilyTool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	// Unmarshal the search response
	var searchResp tavily.SearchResponse
	if err := sonic.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Failed to unmarshal search response: %v", err)
	}

	// Print the AI-generated answer if available
	if searchResp.Answer != "" {
		fmt.Println("AI Answer:")
		fmt.Println("==========")
		fmt.Println(searchResp.Answer)
		fmt.Println()
	}

	// Print the search results
	fmt.Println("Search Results:")
	fmt.Println("===============")
	for i, result := range searchResp.Results {
		fmt.Printf("\n%d. %s\n", i+1, result.Title)
		fmt.Printf("   URL:     %s\n", result.URL)
		fmt.Printf("   Score:   %.2f\n", result.Score)
		fmt.Printf("   Content: %s\n", truncateString(result.Content, 200))
	}
}

// truncateString truncates a string to a maximum length and adds ellipsis if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
