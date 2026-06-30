# MiniMax - ChatModel

[English](README.md) | 中文

## 概述

[Eino](https://github.com/cloudwego/eino) 框架的 MiniMax ChatModel 组件，通过 OpenAI 兼容 API 接入 [MiniMax](https://www.minimaxi.com/) 大语言模型。

## 支持的模型

| 模型 ID | 说明 |
|---------|------|
| `MiniMax-M2.7` | 巅峰性能，极致性价比，攻克复杂任务。(默认) |
| `MiniMax-M2.7-highspeed` | 同等性能，更快更敏捷 |

两个模型均支持 204,800 tokens 上下文窗口，最大输出 192K tokens。

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/model/minimax"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    model, err := minimax.NewChatModel(ctx, &minimax.Config{
        APIKey: "your-minimax-api-key", // 或设置 MINIMAX_API_KEY 环境变量
        Model:  "MiniMax-M2.7",
    })
    if err != nil {
        log.Fatal(err)
    }

    result, err := model.Generate(ctx, []*schema.Message{
        schema.SystemMessage("你是一个有帮助的助手。"),
        {Role: schema.User, Content: "你好！"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Content)
}
```

## 配置项

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `APIKey` | string | 是 | - | MiniMax API 密钥 |
| `Model` | string | 否 | `MiniMax-M2.7` | 模型 ID |
| `BaseURL` | string | 否 | `https://api.minimax.io/v1` | API 地址。国内用户使用 `https://api.minimaxi.com/v1` |
| `Temperature` | *float32 | 否 | `1.0` | 采样温度，范围 (0.0, 1.0] |
| `TopP` | *float32 | 否 | `1.0` | 核采样参数 |
| `MaxTokens` | *int | 否 | 模型最大值 | 最大输出 tokens |
| `Stop` | []string | 否 | - | 停止序列 |
| `Timeout` | time.Duration | 否 | 无超时 | 请求超时时间 |

## 功能特性

- 对话生成 (Generate)
- 流式输出 (Stream)
- 工具调用 / 函数调用
- 回调集成
- 温度自动钳位 (自动约束到 MiniMax 允许的范围)

## 相关链接

- [MiniMax 开放平台](https://platform.minimax.io/)
- [MiniMax API 文档](https://platform.minimax.io/docs/api-reference/text-openai-api)
- [Eino 框架](https://github.com/cloudwego/eino)
