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
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/adk/backend/agentkit"
)

func main() {
	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("Ark Sandbox Middleware Example")
	fmt.Println("========================================")
	fmt.Println()

	// ========================================
	// Step 1: Load Configuration
	// ========================================
	fmt.Println("Step 1: Loading configuration...")

	// Load credentials from environment variables for security
	// Set these before running:
	//   export VOLC_ACCESS_KEY_ID="your_access_key"
	//   export VOLC_SECRET_ACCESS_KEY="your_secret_key"
	//   export VOLC_TOOL_ID="your_tool_id"
	accessKeyID := os.Getenv("VOLC_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("VOLC_SECRET_ACCESS_KEY")
	toolID := os.Getenv("VOLC_TOOL_ID")

	if accessKeyID == "" || secretAccessKey == "" || toolID == "" {
		log.Fatal("✗ Error: Missing required environment variables.\n" +
			"Please set: VOLC_ACCESS_KEY_ID, VOLC_SECRET_ACCESS_KEY, and VOLC_TOOL_ID")
	}

	fmt.Println("✓ Configuration loaded")
	fmt.Println()

	// ========================================
	// Step 2: Initialize Ark Sandbox Backend
	// ========================================
	fmt.Println("Step 2: Initializing Ark Sandbox backend...")

	config := &agentkit.Config{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		ToolID:          toolID,
		UserSessionID:   "middleware-example-" + time.Now().Format("20060102-150405"),
		Region:          agentkit.RegionOfBeijing,
	}

	backend, err := agentkit.NewSandboxToolBackend(config)
	if err != nil {
		log.Fatalf("✗ Failed to create backend: %v", err)
	}

	fmt.Println("✓ Backend initialized")
	fmt.Println()

	// Prepare test file
	testFilePath := "/home/gem/example_file.txt"
	testContent := "Hello from ArkSandbox!\nThis is a test for file operations.\n"

	err = backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: testFilePath,
		Content:  testContent,
	})
	if err != nil {
		log.Printf("⚠ Note: Could not write test file (may already exist): %v\n", err)
	}

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
		Description: "An AI agent capable of performing filesystem operations using Ark Sandbox",
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

	userQuery := "List all files in the '/home/gem' directory and show me their details."
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
