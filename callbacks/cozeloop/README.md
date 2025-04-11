# CozeLoop Callbacks

English | [简体中文](README_zh.md)

A CozeLoop callback implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Handler` interface. This enables seamless integration with Eino's application for enhanced observability.

## Features

- Implements `github.com/cloudwego/eino/internel/callbacks.Handler`
- Easy integration with Eino's application

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/cozeloop
```

## Quick Start

```go
package main
import (
	"context"
	"log"

	ccb "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/cloudwego/eino/callbacks"
	"github.com/coze-dev/cozeloop-go"
)

func main() {
	// 设置相关环境变量
	// COZELOOP_WORKSPACE_ID=your workspace id
	// COZELOOP_API_TOKEN=your token
	client, err := cozeloop.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close(ctx)
	// 在服务 init 时 once 调用
	handler := ccb.NewLoopHandler(client)
	callbacks.AppendGlobalHandlers(handler)
}
```

## For More Details
- [CozeLoop Documentation](https://github.com/coze-dev/cozeloop-go)
