/*
 * Copyright 2026 CloudWeGo Authors
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

	"github.com/cloudwego/eino-ext/components/tool/serply"
)

func main() {
	ctx := context.Background()

	// Get an API key at https://serply.io
	apiKey := os.Getenv("SERPLY_API_KEY")
	if apiKey == "" {
		log.Fatal("SERPLY_API_KEY must be set")
	}

	// Create the search tool. SearchType can also be serply.SearchTypeNews or serply.SearchTypeScholar.
	searchTool, err := serply.NewTool(ctx, &serply.Config{
		APIKey:     apiKey,
		SearchType: serply.SearchTypeWeb,
		Num:        5,
	})
	if err != nil {
		log.Fatal(err)
	}

	req := serply.SearchRequest{
		Query: "CloudWeGo Eino",
	}

	args, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	// Execute search
	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp serply.SearchResponse
	if err = json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatal(err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	fmt.Printf("Query: %s\n\n", searchResp.Query)

	for i, result := range searchResp.Results {
		fmt.Printf("%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.Link)
		fmt.Printf("   Description: %s\n\n", result.Description)
	}
	fmt.Println("==============")
}
