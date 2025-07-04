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
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
)

func main() {
	ctx := context.Background()

	// Create configuration
	config := &duckduckgo.Config{
		MaxResults: 20, // Limit to return 20 results
		Region:     duckduckgo.RegionWT,
		Timeout:    10 * time.Second,
	}

	// Create search client
	tool, err := duckduckgo.NewTextSearchTool(ctx, config)
	if err != nil {
		log.Fatalf("NewTextSearchTool of duckduckgo failed, err=%v", err)
	}

	results := make([]*duckduckgo.TextSearchResult, 0, config.MaxResults)

	searchReq := &duckduckgo.TextSearchRequest{
		Query: "eino",
	}
	jsonReq, err := json.Marshal(searchReq)
	if err != nil {
		log.Fatalf("Marshal of search request failed, err=%v", err)
	}

	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search of duckduckgo failed, err=%v", err)
	}

	var searchResp duckduckgo.TextSearchResponse
	if err = json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Unmarshal of search response failed, err=%v", err)
	}

	results = append(results, searchResp.Results...)

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	fmt.Printf("%s\n", searchResp.Message)
	for i, result := range results {
		fmt.Printf("\n%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   URL: %s\n", result.URL)
		fmt.Printf("   Summary: %s\n", result.Summary)
	}
	fmt.Println("")
	fmt.Println("==============")
}
