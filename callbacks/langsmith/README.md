# Langsmith 回调

简体中文

这是一个为 [langsmith](https://www.langchain.com/langsmith) 实现的 Trace 回调。该工具实现了 `Handler` 接口，可以与 Eino 的应用无缝集成以提供增强的可观测能力。

## 特性

- 实现了 `github.com/cloudwego/eino/internel/callbacks.Handler` 接口
- 易于与 Eino 应用集成

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/langsmith
```

## 快速开始

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

	// 设置全局上报handler
	callbacks.AppendGlobalHandlers(cbh)
	
	ctx := context.Background()
	ctx = langsmith.SetTrace(ctx,
		langsmith.WithSessionName("your session name"), // 设置langsmith上报项目名称
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