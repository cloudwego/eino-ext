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
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
)

func main() {
	// Set the Bing Search API key
	bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")

	// Create a context
	ctx := context.Background()

	// Create the Bing Search tool
	bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
		APIKey: bingSearchAPIKey,
		Cache:  5 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Create a search request
	request := &bingsearch.SearchRequest{
		Query:  "Eino",
		Offset: 0,
	}

	jsonReq, err := sonic.Marshal(request)

	// Execute the search
	resp, err := bingSearchTool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	// Unmarshal the search response
	var searchResp bingsearch.SearchResponse
	if err := sonic.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Failed to unmarshal search response: %v", err)
	}

	// Print the search results
	fmt.Println("Search Results:")
	for i, result := range searchResp.Results {
		fmt.Printf("Title %d.     %s\n", i+1, result.Title)
		fmt.Printf("Link:          %s\n", result.URL)
		fmt.Printf("Description:   %s\n", result.Description)
	}
}
