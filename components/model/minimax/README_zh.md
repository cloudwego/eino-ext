# Minimax ChatModel

本模块提供 Minimax ChatModel 在 Eino 框架中的实现。

## 支持的模型

- MiniMax-M2.7
- MiniMax-M2.7-highspeed
- MiniMax-M2.5
- MiniMax-M2.5-highspeed
- MiniMax-M2.1
- MiniMax-M2.1-highspeed
- MiniMax-M2

## 环境要求

- Go 1.22+
- `github.com/cloudwego/eino` v0.7.13+

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/minimax
```

## 使用示例

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/cloudwego/eino-ext/components/model/minimax"
)

func main() {
    ctx := context.Background()
    
    model, err := minimax.NewChatModel(ctx, &minimax.Config{
        APIKey:    "your-api-key",
        Model:     "MiniMax-M2.7",
        MaxTokens: 1024,
    })
    if err != nil {
        panic(err)
    }
    
    resp, err := model.Generate(ctx, []*schema.Message{
        schema.UserMessage("你好，你是谁？"),
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Println(resp.Content)
}
```

## 配置参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| APIKey | string | 是 | 您的 Minimax API 密钥 |
| Model | string | 是 | 模型名称（如 "MiniMax-M2.7"） |
| MaxTokens | int | 是 | 最大生成 token 数 |
| BaseURL | *string | 否 | 自定义 API 端点 |
| Temperature | *float32 | 否 | 采样温度 (0.0-1.0) |
| TopP | *float32 | 否 | 核采样参数 |
| HTTPClient | *http.Client | 否 | 自定义 HTTP 客户端 |
| AdditionalHeaderFields | map[string]string | 否 | 额外的 HTTP 请求头 |
| AdditionalRequestFields | map[string]any | 否 | 额外的请求字段 |

## 工具调用

```go
model, err := minimax.NewChatModel(ctx, &minimax.Config{
    APIKey:    "your-api-key",
    Model:     "MiniMax-M2.7",
    MaxTokens: 1024,
})

toolModel, err := model.WithTools([]*schema.ToolInfo{
    {
        Name: "get_weather",
        Desc: "获取指定位置的当前天气",
        ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
            Type: "object",
            Properties: orderedmap.New[string, *jsonschema.Schema](
                orderedmap.Pair[string, *jsonschema.Schema]{
                    Key: "location",
                    Value: &jsonschema.Schema{Type: "string"},
                },
            ),
            Required: []string{"location"},
        }),
    },
})
```

## 流式输出

```go
stream, err := model.Stream(ctx, []*schema.Message{
    schema.UserMessage("写一个故事"),
})

for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        panic(err)
    }
    fmt.Print(chunk.Content)
}
```

## License

Apache 2.0
