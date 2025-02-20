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
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo"
	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/ddgsearch"
)

func main() {
	ctx := context.Background()

	// Create configuration
	config := &duckduckgo.Config{
		MaxResults: 3, // Limit to return 3 results
		Region:     ddgsearch.RegionCN,
		DDGConfig: &ddgsearch.Config{
			Timeout:    10,
			Cache:      true,
			MaxRetries: 5,
		},
	}

	// Create search client
	tool, err := duckduckgo.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("NewTool of duckduckgo failed, err=%v", err)
	}

	// Create search request
	searchReq := &duckduckgo.SearchRequest{
		Query: "Golang programming development",
		Page:  1,
	}

	jsonReq, err := json.Marshal(searchReq)
	if err != nil {
		log.Fatalf("Marshal of search request failed, err=%v", err)
	}

	// Execute search
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search of duckduckgo failed, err=%v", err)
	}

	var searchResp duckduckgo.SearchResponse
	if err := json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Unmarshal of search response failed, err=%v", err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	for i, result := range searchResp.Results {
		fmt.Printf("\n%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.Link)
		fmt.Printf("   Description: %s\n", result.Description)
	}
	fmt.Println("")
	fmt.Println("==============")
}
