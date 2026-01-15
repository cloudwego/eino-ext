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

	"github.com/cloudwego/eino/adk/middlewares/filesystem"

	"github.com/cloudwego/eino-ext/adk/backend/agentkit"
)

func main() {
	ctx := context.Background()

	// Step 1: Load configuration from environment variables
	// For security reasons, never hardcode credentials in your source code.
	// Set these environment variables before running:
	//   export VOLC_ACCESS_KEY_ID="your_access_key"
	//   export VOLC_SECRET_ACCESS_KEY="your_secret_key"
	//   export VOLC_TOOL_ID="your_tool_id"
	accessKeyID := os.Getenv("VOLC_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("VOLC_SECRET_ACCESS_KEY")
	toolID := os.Getenv("VOLC_TOOL_ID")

	if accessKeyID == "" || secretAccessKey == "" || toolID == "" {
		log.Fatal("Error: Missing required environment variables.\n" +
			"Please set: VOLC_ACCESS_KEY_ID, VOLC_SECRET_ACCESS_KEY, and VOLC_TOOL_ID")
	}

	// Step 2: Configure the Ark Sandbox backend
	// UserSessionID should be unique for each execution context to ensure isolation
	config := &agentkit.Config{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		ToolID:          toolID,
		UserSessionID:   "example-session-" + time.Now().Format("20060102-150405"),
		Region:          agentkit.RegionOfBeijing,
	}

	// Step 3: Initialize the Ark Sandbox backend
	backend, err := agentkit.NewSandboxToolBackend(config)
	if err != nil {
		log.Fatalf("Failed to create agentKitSandboxToolBackend: %v", err)
	}
	fmt.Println("✓ agentKitSandboxToolBackend initialized successfully")
	fmt.Println()

	// Test file configuration
	// Note: Use /home/gem directory as it's writable by the default 'gem' user
	testFilePath := "/home/gem/example_file.txt"
	testContent := "Hello from ArkSandbox!\nThis is a test for file operations.\n"

	// ========================================
	// Example 1: Write a file
	// ========================================
	fmt.Println("Example 1: Write a file")
	fmt.Println("------------------------")
	fmt.Printf("Writing to: %s\n", testFilePath)

	err = backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: testFilePath,
		Content:  testContent,
	})
	if err != nil {
		// Note: Write fails if the file already exists (safety feature)
		// If you need to overwrite, delete the file first or use Edit
		log.Printf("⚠ Warning: Could not write file (may already exist): %v\n", err)
	} else {
		fmt.Println("✓ File written successfully")
	}
	fmt.Println()

	// ========================================
	// Example 2: Read a file
	// ========================================
	fmt.Println("Example 2: Read a file")
	fmt.Println("----------------------")
	fmt.Printf("Reading from: %s\n", testFilePath)

	content, err := backend.Read(ctx, &filesystem.ReadRequest{
		FilePath: testFilePath,
	})
	if err != nil {
		log.Fatalf("✗ Failed to read file: %v", err)
	}

	fmt.Println("File content:")
	fmt.Println("─────────────────────────")
	fmt.Print(content)
	fmt.Println("─────────────────────────")
	fmt.Println()

	// ========================================
	// Example 3: List directory contents
	// ========================================
	fmt.Println("Example 3: List directory contents")
	fmt.Println("-----------------------------------")
	fmt.Println("Listing: /home/gem")

	files, err := backend.LsInfo(ctx, &filesystem.LsInfoRequest{
		Path: "/home/gem",
	})
	if err != nil {
		log.Fatalf("✗ Failed to list files: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("(empty directory)")
	} else {
		fmt.Printf("Found %d item(s):\n", len(files))
		for i, f := range files {
			fmt.Printf("  %d. %s\n", i+1, f.Path)
		}
	}
	fmt.Println()

	// ========================================
	// Example 4: Search file content (Grep)
	// ========================================
	fmt.Println("Example 4: Search file content (Grep)")
	fmt.Println("--------------------------------------")
	fmt.Println("Searching for: 'ArkSandbox' in *.txt files")

	matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{
		Path:    "/home/gem",
		Pattern: "ArkSandbox",
		Glob:    "*.txt",
	})
	if err != nil {
		log.Fatalf("✗ Failed to grep: %v", err)
	}

	if len(matches) == 0 {
		fmt.Println("No matches found")
	} else {
		fmt.Printf("✓ Found %d match(es):\n", len(matches))
		for _, match := range matches {
			fmt.Printf("  • %s:%d - %s\n", match.Path, match.Line, match.Content)
		}
	}
	fmt.Println()

	// ========================================
	// Example 5: Find files by pattern (Glob)
	// ========================================
	fmt.Println("Example 5: Find files by pattern (Glob)")
	fmt.Println("----------------------------------------")
	fmt.Println("Pattern: *.txt in /home/gem")

	globFiles, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{
		Path:    "/home/gem",
		Pattern: "*.txt",
	})
	if err != nil {
		log.Fatalf("✗ Failed to glob: %v", err)
	}

	if len(globFiles) == 0 {
		fmt.Println("No matching files found")
	} else {
		fmt.Printf("✓ Found %d file(s):\n", len(globFiles))
		for i, f := range globFiles {
			fmt.Printf("  %d. %s\n", i+1, f.Path)
		}
	}
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("✓ All examples completed successfully!")
	fmt.Println("========================================")
}
