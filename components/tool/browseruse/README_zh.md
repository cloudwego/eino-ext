# BrowserUse Tool

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 BrowserUse Tool 组件，实现了 `Tool` 接口。这使得能够无缝集成 Eino 的 LLM 功能，以增强自然语言处理和生成能力。
> **注意**：此实现受到 [OpenManus](https://github.com/mannaandpoem/OpenManus) 项目的启发和参考。

## 特性

- 实现 `github.com/cloudwego/eino/components/tool.BaseTool`
- 易于与 Eino 的工具系统集成
- 支持执行浏览器操作

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/browseruse@latest
```

## 快速开始

以下是如何使用 browser-use 工具的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/browseruse"
)

func main() {
	ctx := context.Background()
	but, err := browseruse.NewBrowserUseTool(ctx, &browseruse.Config{})
	if err != nil {
		log.Fatal(err)
	}

	url := "https://www.google.com"
	result, err := but.Execute(&browseruse.Param{
		Action: browseruse.ActionGoToURL,
		URL:    &url,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
	time.Sleep(10 * time.Second)
	but.Cleanup()
}

```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
