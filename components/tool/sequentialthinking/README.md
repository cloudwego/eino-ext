# Sequential Thinking Tool

**Sequential Thinking Tool** is a tool for dynamic and reflective problem-solving through a structured thinking process.
Inspired by [@modelcontextprotocol
/sequentialthinking](https://github.com/modelcontextprotocol/servers/tree/HEAD/src/sequentialthinking), it guides LLM
through a series of questions to help them think through problems step-by-step.

## Features

- Guided step-by-step thinking process.
- Dynamic questioning and reflection.
- Enhances problem-solving abilities.

## Usage

The Sequential Thinking tool is designed for:

- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

## Install

```bash
go get github.com/cloudwego/eino-ext/components/tool/sequentialthinking@latest
```

## Quick Start

```go
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

```