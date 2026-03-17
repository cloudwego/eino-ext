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

	"github.com/cloudwego/eino-ext/components/tool/sougousearch"
)

func main() {
	ctx := context.Background()

	tencentCloudSecretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	tencentCloudSecretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")

	if tencentCloudSecretID == "" || tencentCloudSecretKey == "" {
		log.Fatal("[TENCENTCLOUD_SECRET_ID] and [TENCENTCLOUD_SECRET_KEY] must set")
	}

	// create tool
	searchTool, err := sougousearch.NewTool(ctx, &sougousearch.Config{
		SecretID:  tencentCloudSecretID,
		SecretKey: tencentCloudSecretKey,
		Cnt:       5,
		Mode:      0, // natural search
	})
	if err != nil {
		log.Fatal(err)
	}

	// prepare params
	cnt := uint64(3)
	req := sougousearch.SearchRequest{
		Query: "Golang concurrent programming",
		Cnt:   &cnt,
	}

	args, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	// do search
	resp, err := searchTool.InvokableRun(ctx, string(args))
	if err != nil {
		log.Fatal(err)
	}

	var searchResp sougousearch.SearchResult
	if err := json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatal(err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	for i, result := range searchResp.Items {
		fmt.Printf("\n%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.URL)
		fmt.Printf("   Desc: %s\n", result.Passage)
	}
	fmt.Println("")
	fmt.Println("==============")

	// seems like:
	// Search Results:
	// ==============
	// 1. Title: Go Concurrency Patterns - The Go Programming Language
	//    Link: https://go.dev/blog/pipelines
	//    Desc: Go's concurrency primitives make it easy to construct streaming data pipelines that make efficient use of I/O and multiple CPUs...
	//
	// 2. Title: A Tour of Go - Concurrency
	//    Link: https://go.dev/tour/concurrency/1
	//    Desc: Go provides concurrency constructions as part of the core language...
	// ...
	// ==============
}
