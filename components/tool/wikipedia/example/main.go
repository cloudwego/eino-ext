package main

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
	"log"
	"time"
)

func main() {
	ctx := context.Background()

	// Create configuration
	config := &wikipedia.Config{
		UserAgent:   "eino",
		DocMaxChars: 2000,
		Timeout:     15 * time.Second,
		TopK:        4,
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
	if err := sonic.Unmarshal([]byte(resp), &searchResponse); err != nil {
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
