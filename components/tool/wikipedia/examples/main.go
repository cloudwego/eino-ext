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
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
)

func main() {
	ctx := context.Background()

	// Create configuration
	config := &wikipedia.Config{
		UserAgent:   "eino",
		DocMaxChars: 2000,
		Timeout:     15 * time.Second,
		TopK:        3,
		MaxRedirect: 3,
		Language:    "en",
	}

	// Create wikipedia tool
	tool, err := wikipedia.NewTool(ctx, config)
	if err != nil {
		log.Fatal("Failed to create tool:", err)
	}

	// Create search request
	m, err := sonic.MarshalString(wikipedia.SearchRequest{"bytedance"})
	if err != nil {
		log.Fatal("Failed to marshal search request:", err)
	}

	// Execute search
	resp, err := tool.InvokableRun(ctx, m)
	if err != nil {
		log.Fatal("Search failed:", err)
	}

	var searchResponse wikipedia.SearchResponse
	if err = sonic.Unmarshal([]byte(resp), &searchResponse); err != nil {
		log.Fatal("Failed to unmarshal search response:", err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	for _, r := range searchResponse.Results {
		fmt.Printf("Title: %s\n", r.Title)
		fmt.Printf("URL: %s\n", r.URL)
		fmt.Printf("Summary: %s\n", r.Extract)
		fmt.Printf("Snippet: %s\n", r.Snippet)
	}
	fmt.Println("")
	fmt.Println("==============")

}
