package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/atlascloud"
)

func main() {
	apiKey := os.Getenv("ATLASCLOUD_API_KEY")
	modelName := os.Getenv("ATLASCLOUD_MODEL")
	if modelName == "" {
		modelName = "deepseek-ai/DeepSeek-V3-0324"
	}

	if apiKey == "" {
		log.Fatal("ATLASCLOUD_API_KEY is required")
	}

	ctx := context.Background()
	chatModel, err := atlascloud.NewChatModel(ctx, &atlascloud.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelName,
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a concise assistant."),
		schema.UserMessage("Reply with exactly one short sentence introducing Atlas Cloud in Chinese."),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	fmt.Println(resp.Content)
}
