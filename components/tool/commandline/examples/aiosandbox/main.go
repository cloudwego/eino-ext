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
	"os"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino-ext/components/tool/commandline/aiosandbox"
)

func main() {
	ctx := context.Background()

	// Get configuration from environment variables
	baseURL := os.Getenv("AIO_SANDBOX_BASE_URL")
	token := os.Getenv("AIO_SANDBOX_TOKEN")

	if baseURL == "" || token == "" {
		log.Fatal("Please set AIO_SANDBOX_BASE_URL and AIO_SANDBOX_TOKEN environment variables")
	}

	// Create AIO Sandbox
	sandbox, err := aiosandbox.NewAIOSandbox(ctx, &aiosandbox.Config{
		BaseURL:     baseURL,
		Token:       token,
		WorkDir:     "/tmp",
		Timeout:     120,
		KeepSession: true, // Enable session persistence for stateful operations
	})
	if err != nil {
		log.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Close(ctx)

	fmt.Println("=== AIO Sandbox Example ===")
	fmt.Printf("Session ID: %s\n\n", sandbox.GetSessionID())

	// Example 1: Execute a simple command
	fmt.Println("1. Executing 'echo hello'...")
	output, err := sandbox.RunCommand(ctx, []string{"echo", "hello"})
	if err != nil {
		log.Fatalf("RunCommand failed: %v", err)
	}
	fmt.Printf("   Output: %s", output.Stdout)
	fmt.Printf("   Exit Code: %d\n\n", output.ExitCode)

	// Example 2: Write a Python script
	fmt.Println("2. Writing a Python script...")
	pythonCode := `#!/usr/bin/env python3
import sys
print(f"Hello from Python {sys.version_info.major}.{sys.version_info.minor}!")
print("Arguments:", sys.argv[1:])
`
	err = sandbox.WriteFile(ctx, "/workspace/hello.py", pythonCode)
	if err != nil {
		log.Fatalf("WriteFile failed: %v", err)
	}
	fmt.Println("   Script written to /workspace/hello.py\n")

	// Example 3: Execute the Python script
	fmt.Println("3. Executing Python script...")
	output, err = sandbox.RunCommand(ctx, []string{"python3", "/workspace/hello.py", "arg1", "arg2"})
	if err != nil {
		log.Fatalf("RunCommand failed: %v", err)
	}
	fmt.Printf("   Output: %s\n", output.Stdout)

	// Example 4: Read file content
	fmt.Println("4. Reading file content...")
	content, err := sandbox.ReadFile(ctx, "/workspace/hello.py")
	if err != nil {
		log.Fatalf("ReadFile failed: %v", err)
	}
	fmt.Printf("   Content:\n%s\n", content)

	// Example 5: Check file existence
	fmt.Println("5. Checking file existence...")
	exists, err := sandbox.Exists(ctx, "/workspace/hello.py")
	if err != nil {
		log.Fatalf("Exists failed: %v", err)
	}
	fmt.Printf("   /workspace/hello.py exists: %v\n\n", exists)

	// Example 6: Check if path is directory
	fmt.Println("6. Checking directory...")
	isDir, err := sandbox.IsDirectory(ctx, "/workspace")
	if err != nil {
		log.Fatalf("IsDirectory failed: %v", err)
	}
	fmt.Printf("   /workspace is directory: %v\n\n", isDir)

	// Example 7: Use with eino tools
	fmt.Println("7. Creating eino tools with AIO Sandbox...")

	// StrReplaceEditor
	editor, err := commandline.NewStrReplaceEditor(ctx, &commandline.EditorConfig{
		Operator: sandbox,
	})
	if err != nil {
		log.Fatalf("Failed to create editor: %v", err)
	}
	editorInfo, _ := editor.Info(ctx)
	fmt.Printf("   Editor tool: %s\n", editorInfo.Name)

	// PyExecutor
	pyExecutor, err := commandline.NewPyExecutor(ctx, &commandline.PyExecutorConfig{
		Command:  "python3",
		Operator: sandbox,
	})
	if err != nil {
		log.Fatalf("Failed to create PyExecutor: %v", err)
	}
	pyInfo, _ := pyExecutor.Info(ctx)
	fmt.Printf("   PyExecutor tool: %s\n", pyInfo.Name)

	fmt.Println("\n=== Example completed successfully ===")
}
