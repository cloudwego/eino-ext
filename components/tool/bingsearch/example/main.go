package main

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch/bingcore"
	"log"
	"os"
)

func main() {
	// Set the Bing Search API key
	bingSearchAPIKey := os.Getenv("BING_SEARCH_API_KEY")

	// Create a context
	ctx := context.Background()

	// Create the Bing Search tool
	bingSearchTool, err := bingsearch.NewTool(ctx, &bingsearch.Config{
		APIKey: bingSearchAPIKey,
		BingConfig: &bingcore.Config{
			Cache: true,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Create a search request
	request := &bingsearch.SearchRequest{
		Query: "Eino",
		Page:  1,
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
