# Langsmith 回调

简体中文

这是一个为 [langsmith](https://github.com/cloudwego/eino-ext) 实现的 Trace 回调。该工具实现了 `Handler` 接口，可以与 Eino 的应用无缝集成以提供增强的可观测能力。

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
	"log"

	"github.com/google/uuid"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino-ext/callbacks/langsmith"
	
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
}
```