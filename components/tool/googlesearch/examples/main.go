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

	"github.com/cloudwego/eino-ext/components/tool/googlesearch"
)

func main() {
	ctx := context.Background()

	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	googleSearchEngineID := os.Getenv("GOOGLE_SEARCH_ENGINE_ID")

	if googleAPIKey == "" || googleSearchEngineID == "" {
		panic("[GOOGLE_API_KEY] and [GOOGLE_SEARCH_ENGINE_ID] must set")
	}

	// create tool
	searchTool, err := googlesearch.NewTool(ctx, &googlesearch.Config{
		APIKey:         googleAPIKey,
		SearchEngineID: googleSearchEngineID,
		Lang:           "zh-CN",
		Num:            5,
	})
	if err != nil {
		panic(err)
	}

	// prepare params
	req := googlesearch.SearchRequest{
		Query: "Golang concurrent programming",
		Num:   3,
		Lang:  "en",
	}

	args, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	// do search
	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		panic(err)
	}

	var searchResp googlesearch.SearchResult
	if err := json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Unmarshal of search response failed, err=%v", err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	for i, result := range searchResp.Items {
		fmt.Printf("\n%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.Link)
		fmt.Printf("   Desc: %s\n", result.Desc)
	}
	fmt.Println("")
	fmt.Println("==============")

	// seems like:
	// Search Results:
	// ==============
	// 1. Title: My Concurrent Programming book is finally PUBLISHED!!! : r/golang
	//    Link: https://www.reddit.com/r/golang/comments/18b86aa/my_concurrent_programming_book_is_finally/
	//    Desc: Posted by u/channelselectcase - 398 votes and 46 comments
	// 2. Title: Concurrency — An Introduction to Programming in Go | Go Resources
	//    Link: https://www.golang-book.com/books/intro/10
	//    Desc:
	// 3. Title: The Comprehensive Guide to Concurrency in Golang | by Brandon ...
	//    Link: https://bwoff.medium.com/the-comprehensive-guide-to-concurrency-in-golang-aaa99f8bccf6
	//    Desc: Update (November 20, 2023) — This article has undergone a comprehensive revision for enhanced clarity and conciseness. I’ve streamlined the…

	// ==============
}
