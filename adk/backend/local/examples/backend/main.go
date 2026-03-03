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

	"github.com/cloudwego/eino/adk/middlewares/filesystem"

	"github.com/cloudwego/eino-ext/adk/backend/local"
)

func main() {
	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("Local Backend Example")
	fmt.Println("========================================")
	fmt.Println()

	// ========================================
	// Step 1: Create Temporary Directory
	// ========================================
	fmt.Println("Step 1: Setting up temporary directory...")

	tempDir, err := os.MkdirTemp("", "local-backend-example-*")
	if err != nil {
		log.Fatalf("✗ Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("✓ Created temporary directory: %s\n", tempDir)
	fmt.Println()

	// ========================================
	// Step 2: Initialize Local Backend
	// ========================================
	fmt.Println("Step 2: Initializing Local Backend...")

	// The local backend requires minimal configuration
	// You can optionally provide a ValidateCommand function for security
	backend, err := local.NewBackend(ctx, &local.Config{
		// ValidateCommand: myCommandValidator, // Optional: for Execute() security
	})
	if err != nil {
		log.Fatalf("✗ Failed to create local backend: %v", err)
	}

	fmt.Println("✓ Local Backend initialized")
	fmt.Println()

	// Test file path
	filePath := filepath.Join(tempDir, "test.txt")

	// ========================================
	// Example 1: Write a file
	// ========================================
	fmt.Println("Example 1: Write a file")
	fmt.Println("-----------------------")
	fmt.Printf("Writing to: %s\n", filePath)

	err = backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: filePath,
		Content:  "Hello, Local Backend!",
	})
	if err != nil {
		log.Fatalf("✗ Failed to write file: %v", err)
	}

	fmt.Println("✓ File written successfully")
	fmt.Println()

	// ========================================
	// Example 2: Read a file
	// ========================================
	fmt.Println("Example 2: Read a file")
	fmt.Println("----------------------")
	fmt.Printf("Reading from: %s\n", filePath)

	content, err := backend.Read(ctx, &filesystem.ReadRequest{
		FilePath: filePath,
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
	fmt.Printf("Listing: %s\n", tempDir)

	files, err := backend.LsInfo(ctx, &filesystem.LsInfoRequest{
		Path: tempDir,
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
	// Example 4: Edit a file
	// ========================================
	fmt.Println("Example 4: Edit a file")
	fmt.Println("----------------------")
	fmt.Println("Replacing 'Hello' with 'Hi'")

	err = backend.Edit(ctx, &filesystem.EditRequest{
		FilePath:  filePath,
		OldString: "Hello",
		NewString: "Hi",
	})
	if err != nil {
		log.Fatalf("✗ Failed to edit file: %v", err)
	}

	fmt.Println("✓ File edited successfully")
	fmt.Println()

	// ========================================
	// Example 5: Read edited file
	// ========================================
	fmt.Println("Example 5: Read edited file")
	fmt.Println("---------------------------")
	fmt.Printf("Reading from: %s\n", filePath)

	editedContent, err := backend.Read(ctx, &filesystem.ReadRequest{
		FilePath: filePath,
	})
	if err != nil {
		log.Fatalf("✗ Failed to read edited file: %v", err)
	}

	fmt.Println("Updated file content:")
	fmt.Println("─────────────────────────")
	fmt.Print(editedContent)
	fmt.Println("─────────────────────────")
	fmt.Println()

	// ========================================
	// Example 6: Search file content (Grep)
	// ========================================
	fmt.Println("Example 6: Search file content (Grep)")
	fmt.Println("--------------------------------------")
	fmt.Println("Searching for: 'Local' in *.txt files")

	matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{
		Path:    tempDir,
		Pattern: "Local",
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
			fmt.Printf("  • %s:%d - %s\n", filepath.Base(match.Path), match.Line, match.Content)
		}
	}
	fmt.Println()

	// ========================================
	// Example 7: Find files by pattern (Glob)
	// ========================================
	fmt.Println("Example 7: Find files by pattern (Glob)")
	fmt.Println("----------------------------------------")
	fmt.Println("Pattern: *.txt")

	globFiles, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{
		Path:    tempDir,
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
