# Langsmith Callbacks

English | [简体中文](README_zh.md)

A Langsmith callback implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Handler` interface. This enables seamless integration with Eino's application for enhanced observability and tracing.

## Features

- Implements `github.com/cloudwego/eino/internel/callbacks.Handler` interface
- Easy integration with Eino's application

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/langsmith
```

## Quick Start

```go
package main
import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/callbacks/langsmith"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	
)

func main() {

	cfg := &langsmith.Config{
		APIKey: "your api key",
		APIURL: "your api url",
		IDGen: func(ctx context.Context) string { // optional. id generator. default is uuid.NewString
			return uuid.NewString()
		},
	}
	// ft := langsmith.NewFlowTrace(cfg)
	cbh, err := langsmith.NewLangsmithHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Set global callback handler
	callbacks.AppendGlobalHandlers(cbh)
	
	ctx := context.Background()
	ctx = langsmith.SetTrace(ctx,
		langsmith.WithSessionName("your session name"), // Set langsmith project name for reporting
	)

	g := compose.NewGraph[string, string]()
	// ... add nodes and edges to your graph
	// add node and edage to your eino graph, here is an simple example
	g.AddLambdaNode("node1", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}), compose.WithNodeName("node1"))
	g.AddLambdaNode("node2", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return "test output", nil
	}), compose.WithNodeName("node2"))
	g.AddEdge(compose.START, "node1")
	g.AddEdge("node1", "node2")
	g.AddEdge("node2", compose.END)

	runner, err := g.Compile(ctx)
	if err != nil {
		fmt.Println(err)
	}
	// Invoke the runner
	result, err := runner.Invoke(ctx, "test input\n")
	if err != nil {
		fmt.Println(err)
	}
	// Process the result
	log.Printf("Got result: %s", result)
	
}
```

## Examples

See the [examples](./examples/) directory for complete usage examples.

## For More Details

- [Langsmith Documentation](https://www.langchain.com/langsmith)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
