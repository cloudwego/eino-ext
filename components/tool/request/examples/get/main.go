package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	req "github.com/cloudwego/eino-ext/components/tool/request/get"
)

func main() {
	// Configure the GET tool
	config := &req.Config{
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
		ResponseContentType: "json",
		Timeout:             10 * time.Second,
	}

	ctx := context.Background()

	// Create the GET tool
	tool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Prepare the GET request payload
	request := &req.GetRequest{
		URL: "https://jsonplaceholder.typicode.com/posts",
	}

	jsonReq, err := sonic.Marshal(request)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	// Execute the GET request using the InvokableTool interface
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("GET request failed: %v", err)
	}

	fmt.Println(resp)
}
