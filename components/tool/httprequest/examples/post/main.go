package main

import (
	"context"
	"fmt"
	"log"

	"github.com/bytedance/sonic"
	post "github.com/cloudwego/eino-ext/components/tool/httprequest/post"
)

func main() {
	config := &post.Config{
		Headers: map[string]string{
			"User-Agent":   "MyCustomAgent",
			"Content-Type": "application/json; charset=UTF-8",
		},
	}

	ctx := context.Background()

	tool, err := post.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	request := &post.PostRequest{
		URL:  "https://jsonplaceholder.typicode.com/posts",
		Body: `{"title": "my title","body": "my body","userId": 1}`,
	}

	jsonReq, err := sonic.Marshal(request)

	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Post failed: %v", err)
	}

	fmt.Println(resp)
}
