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
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/prompt/cozeloop"
	cozeloopgo "github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/entity"
)

func main() {
	// Set the following environment variables first:
	// COZELOOP_WORKSPACE_ID=your workspace id
	// COZELOOP_API_TOKEN=your token (for testing)
	// Or use JWT OAuth with:
	// COZELOOP_JWT_OAUTH_CLIENT_ID=your client id
	// COZELOOP_JWT_OAUTH_PRIVATE_KEY=your private key
	// COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID=your public key id

	ctx := context.Background()

	// Initialize CozeLoop client
	client, err := cozeloopgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create CozeLoop client: %v", err)
	}
	defer client.Close(ctx)

	// Create PromptHub component
	ph, err := cozeloop.NewPromptHub(ctx, &cozeloop.Config{
		Key:            "your.prompt.key", // Replace with your prompt key
		Version:        "",                // Empty for latest version, or specify like "1.0.0"
		CozeLoopClient: client,
	})
	if err != nil {
		log.Fatalf("Failed to create PromptHub: %v", err)
	}

	// Format the prompt with variables
	// For multi_part variables, use []*entity.ContentPart
	testMultiPart := []*entity.ContentPart{
		{
			Type: entity.ContentTypeText,
			Text: strPtr("Example text content "),
		},
		{
			Type:     entity.ContentTypeImageURL,
			ImageURL: strPtr("https://example.com/image.png"),
		},
		{
			Type: entity.ContentTypeText,
			Text: strPtr(" with additional description"),
		},
	}

	variables := map[string]any{
		"user_name": "John Doe",
		"topic":     "example topic",
		"lang":      "Go",
		"test":      testMultiPart,
	}

	messages, err := ph.Format(ctx, variables)
	if err != nil {
		log.Fatalf("Failed to format prompt: %v", err)
	}

	// Print the formatted messages
	fmt.Println("Formatted messages:")
	for i, msg := range messages {
		fmt.Printf("\nMessage %d:\n", i+1)
		fmt.Printf("  Role: %s\n", msg.Role)
		fmt.Printf("  Content: %s\n", msg.Content)

		if msg.Name != "" {
			fmt.Printf("  Name: %s\n", msg.Name)
		}

		if len(msg.UserInputMultiContent) > 0 {
			fmt.Printf("  UserInputMultiContent (%d parts):\n", len(msg.UserInputMultiContent))
			for j, part := range msg.UserInputMultiContent {
				fmt.Printf("    Part %d - Type: %s\n", j+1, part.Type)
				if part.Text != "" {
					fmt.Printf("      Text: %s\n", part.Text)
				}
				if part.Image != nil {
					if part.Image.URL != nil {
						fmt.Printf("      Image URL: %s\n", *part.Image.URL)
					}
					if part.Image.Base64Data != nil {
						fmt.Printf("      Image Base64Data: [%d bytes]\n", len(*part.Image.Base64Data))
					}
				}
			}
		}

		if len(msg.AssistantGenMultiContent) > 0 {
			fmt.Printf("  AssistantGenMultiContent (%d parts):\n", len(msg.AssistantGenMultiContent))
			for j, part := range msg.AssistantGenMultiContent {
				fmt.Printf("    Part %d - Type: %s\n", j+1, part.Type)
				if part.Text != "" {
					fmt.Printf("      Text: %s\n", part.Text)
				}
				if part.Image != nil {
					if part.Image.URL != nil {
						fmt.Printf("      Image URL: %s\n", *part.Image.URL)
					}
					if part.Image.Base64Data != nil {
						fmt.Printf("      Image Base64Data: [%d bytes]\n", len(*part.Image.Base64Data))
					}
				}
			}
		}

		if msg.ResponseMeta != nil {
			fmt.Printf("  ResponseMeta: %+v\n", msg.ResponseMeta)
		}

		if len(msg.Extra) > 0 {
			fmt.Printf("  Extra: %+v\n", msg.Extra)
		}
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
