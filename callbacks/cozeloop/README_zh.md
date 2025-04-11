# CozeLoop 回调

[English](README.md) | 简体中文

这是一个为 [CozeLoop](https://github.com/cloudwego/eino) 实现的 Trace 回调。该工具实现了 `Handler` 接口，可以与 Eino 的应用无缝集成以提供增强的可观测能力。

## 特性

- 实现了 `github.com/cloudwego/eino/internel/callbacks.Handler` 接口
- 易于与 Eino 应用集成

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/cozeloop
```

## 快速开始

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

## 更多详情
- [CozeLoop 文档](https://github.com/coze-dev/cozeloop-go) 