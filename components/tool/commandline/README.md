# CommandLine Tool

A CommandLine Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.
> **Note**: This implementation is inspired by and references the [OpenManus](https://github.com/mannaandpoem/OpenManus) project.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Support executing command-line instructions in Docker containers

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/commandline@latest
```

## Quick Start

Here's a quick example of how to use the commandline tool:

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/commandline"
	"github.com/cloudwego/eino-ext/components/tool/commandline/sandbox"
)

func main() {
	ctx := context.Background()

	op, err := sandbox.NewDockerSandbox(ctx, &sandbox.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// you should ensure that docker has been started before create a docker container
	err = op.Create(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer op.Cleanup(ctx)

	sre, err := commandline.NewStrReplaceEditor(ctx, &commandline.Config{Operator: op})
	if err != nil {
		log.Fatal(err)
	}

	info, err := sre.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s, tool desc: %s", info.Name, info.Desc)

	content := "hello world"

	log.Println("create file[test.txt]...")
	result, err := sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command:  commandline.CreateCommand,
		Path:     "./test.txt",
		FileText: &content,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("create file result: ", result)

	log.Println("view file[test.txt]...")
	result, err = sre.Execute(ctx, &commandline.StrReplaceEditorParams{
		Command: commandline.ViewCommand,
		Path:    "./test.txt",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("view file result: ", result)
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)