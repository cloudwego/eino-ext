//go:build boxlite

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
	"log"
	"strings"

	"github.com/cloudwego/eino/adk/filesystem"

	"github.com/cloudwego/eino-ext/adk/backend/boxlite"
)

func main() {
	ctx := context.Background()

	backend, err := boxlite.NewBackend(ctx, &boxlite.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// Create boots a microVM. It needs the BoxLite native library installed
	// (see the package README) and, on macOS, the Hypervisor entitlement.
	if err := backend.Create(ctx); err != nil {
		log.Fatal(err)
	}
	defer backend.Cleanup(ctx)

	// The agent workspace: write, edit, read, run — all inside the microVM.
	if err := backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: "hello.py",
		Content:  "print('hello from a boxlite microVM')\n",
	}); err != nil {
		log.Fatal(err)
	}

	if err := backend.Edit(ctx, &filesystem.EditRequest{
		FilePath:  "hello.py",
		OldString: "hello from",
		NewString: "greetings from",
	}); err != nil {
		log.Fatal(err)
	}

	content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "hello.py"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("file content: %s", content.Content)

	out, err := backend.Execute(ctx, &filesystem.ExecuteRequest{Command: "python3 hello.py"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("execute output: %s", strings.TrimSpace(out.Output))
}
