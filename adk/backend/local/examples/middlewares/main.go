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
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/adk/backend/local"
)

func main() {
	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("Local Backend Middleware Example")
	fmt.Println("========================================")
	fmt.Println()

	// ========================================
	// Step 1: Initialize Local Backend
	// ========================================
	fmt.Println("Step 1: Initializing Local Backend...")

	// The local backend operates directly on the local filesystem
	// Optionally provide ValidateCommand for Execute() method security
	backend, err := local.NewBackend(ctx, &local.Config{
		// ValidateCommand: myCommandValidator, // Optional: for Execute() security
	})
	if err != nil {
		log.Fatalf("✗ Failed to create LocalBackend: %v", err)
	}

	fmt.Println("✓ Local Backend initialized")
	fmt.Println()

	// ========================================
	// Step 2: Prepare Test Data
	// ========================================
	fmt.Println("Step 2: Preparing test data...")

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "local-middleware-example-*")
	if err != nil {
		log.Fatalf("✗ Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFilePath := filepath.Join(tempDir, "example_file.txt")
	testContent := "Hello from Local Backend!\nThis is a test for file operations.\n"

	err = backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: testFilePath,
		Content:  testContent,
	})
	if err != nil {
		log.Fatalf("✗ Failed to write test file: %v", err)
	}

	fmt.Printf("✓ Created test file: %s\n", testFilePath)
	fmt.Println()

	// ========================================
	// Step 3: Initialize Filesystem Middleware
	// ========================================
	fmt.Println("Step 3: Initializing filesystem middleware...")

	// The middleware wraps the backend and provides filesystem tools to the agent
	fileSystemMiddleware, err := filesystem.NewMiddleware(ctx, &filesystem.Config{
		Backend: backend,
	})
	if err != nil {
		log.Fatalf("✗ Failed to create middleware: %v", err)
	}

	fmt.Println("✓ Middleware initialized")
	fmt.Println()

	// ========================================
	// Step 4: Initialize Chat Model
	// ========================================
	fmt.Println("Step 4: Initializing chat model...")

	// IMPORTANT: You must provide a real ChatModel implementation
	// Examples: OpenAI GPT-4, Anthropic Claude, Doubao, etc.
	// The model interprets user requests and decides which tools to call
	chatModel, err := NewChatModel(ctx)
	if err != nil {
		log.Fatalf("✗ Failed to create chat model: %v", err)
	}

	if chatModel == nil {
		log.Fatal("✗ Error: ChatModel is nil. Please implement NewChatModel() with a real model.\n" +
			"See documentation for supported models: OpenAI, Claude, Doubao, etc.")
	}

	fmt.Println("✓ Chat model initialized")
	fmt.Println()

	// ========================================
	// Step 5: Create Agent with Middleware
	// ========================================
	fmt.Println("Step 5: Creating filesystem agent...")

	fileSystemAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "FileSystemAgent",
		Description: "An AI agent capable of performing filesystem operations on the local machine",
		Model:       chatModel,
		Middlewares: []adk.AgentMiddleware{fileSystemMiddleware},
	})
	if err != nil {
		log.Fatalf("✗ Failed to create agent: %v", err)
	}

	fmt.Println("✓ Agent created")
	fmt.Println()

	// ========================================
	// Step 6: Execute Agent with User Input
	// ========================================
	fmt.Println("Step 6: Running agent with user request...")
	fmt.Println()

	userQuery := fmt.Sprintf("List all files in the '%s' directory and show me their details.", tempDir)
	fmt.Printf("User Query: %s\n", userQuery)
	fmt.Println()

	input := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(userQuery),
		},
	}

	fmt.Println("Agent is processing...")
	fmt.Println("─────────────────────────")

	iterator := fileSystemAgent.Run(ctx, input)

	// ========================================
	// Step 7: Process Agent Output
	// ========================================
	eventCount := 0
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		eventCount++
		fmt.Printf("\n[Event %d]\n", eventCount)
		fmt.Printf("Type: %T\n", event)
		fmt.Printf("Details: %+v\n", event)
	}

	fmt.Println("─────────────────────────")
	fmt.Println()
	fmt.Printf("✓ Agent execution finished (%d events processed)\n", eventCount)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("✓ Example completed successfully!")
	fmt.Println("========================================")
}

// NewChatModel creates and returns a ChatModel instance
// IMPORTANT: This is a placeholder. You MUST implement this with a real model.
//
// Example implementations:
//   - OpenAI: Use github.com/cloudwego/eino/components/model/openai
//   - Claude: Use github.com/cloudwego/eino/components/model/anthropic
//   - Doubao: Use github.com/cloudwego/eino/components/model/doubao
//
// Example:
//
//	import "github.com/cloudwego/eino/components/model/openai"
//
//	func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
//	    return openai.NewChatModel(ctx, &openai.ChatModelConfig{
//	        APIKey: os.Getenv("OPENAI_API_KEY"),
//	        Model:  "gpt-4",
//	    })
//	}
func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	// TODO: Implement with a real ChatModel
	// Return nil for now - the example will show an error message
	var chatModel model.ToolCallingChatModel
	return chatModel, nil
}
