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
	
	"github.com/bytedance/sonic"
	
	"github.com/cloudwego/eino-ext/components/tool/sequentialthinking"
)

func main() {
	ctx := context.Background()
	
	// Instantiate the tool
	tool, err := sequentialthinking.NewTool()
	if err != nil {
		panic(err)
	}
	
	args := &sequentialthinking.ThoughtRequest{
		Thought:           "This is a test thought",
		ThoughtNumber:     1,
		TotalThoughts:     3,
		NextThoughtNeeded: true,
	}
	
	argsStr, _ := sonic.Marshal(args)
	
	// Use the tool
	// (This is just a placeholder; actual usage will depend on the tool's functionality)
	result, err := tool.InvokableRun(ctx, string(argsStr))
	if err != nil {
		panic(err)
	}
	
	// Process the result
	// (This is just a placeholder; actual processing will depend on the tool's output)
	fmt.Println(result)
}
