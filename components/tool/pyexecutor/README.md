# PyExecutor Tool

A PyExecutor Tool implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Tool` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.
> **Note**: This implementation is inspired by and references the [OpenManus](https://github.com/mannaandpoem/OpenManus) project.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Easy integration with Eino's tool system
- Support for execute python code

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/tool/pyexecutor@latest
```

## Quick Start

Here's a quick example of how to use the pyexecutor tool:

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/tool/pyexecutor"
)

func main() {
	ctx := context.Background()
	exec, err := pyexecutor.NewPyExecutor(ctx, &pyexecutor.Config{}) // use python3 by default
	if err != nil {
		log.Fatal(err)
	}

	info, err := exec.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool name: %s,  tool desc: %s", info.Name, info.Desc)

	code := "print('hello world')"
	log.Printf("execute code:\n%s", code)
	result, err := exec.Execute(ctx, &pyexecutor.Input{Code: code})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("result:\n", result)
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)