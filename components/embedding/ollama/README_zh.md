# Ollama Embedding for Eino

## 简介
这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Ollama Embedding 组件，实现了 `Embedder` 接口，可无缝集成到 Eino 的 embedding 系统中，提供文本向量化能力。

## 特性
- 实现 `github.com/cloudwego/eino/components/embedding.Embedder` 接口
- 易于集成到 Eino的工作流
- 支持自定义 Ollama 服务端点和模型
- Eino内置回调支持

## 安装
```bash
  go get github.com/cloudwego/eino-ext/components/embedding/ollama
```


## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ollama"
)

func main() {
	ctx := context.Background()

	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_EMBED_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}

	embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		BaseURL: baseURL,
		Model:   model,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ollama error: %v", err)
		return
	}

	log.Printf("===== call Embedder directly =====")

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of Ollama failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## 配置说明

embedder 可以通过 `EmbeddingConfig` 结构体进行配置：

```go
type EmbeddingConfig struct {
    // Timeout 指定等待 API 响应的最大持续时间
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认值：无超时
    Timeout time.Duration `json:"timeout"`
    
    // HTTPClient 指定用于发送 HTTP 请求的客户端
    // 如果设置了 HTTPClient，则不会使用 Timeout
    // 可选。默认值：&http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // BaseURL 指定 Ollama 服务端点 URL
    // 格式：http(s)://host:port
    // 可选。默认值："http://localhost:11434"
    BaseURL string `json:"base_url"`
    
    // Model 指定用于生成嵌入的模型 ID
    // 必需
    Model string `json:"model"`
    
    // Truncate 指定是否将文本截断到模型的最大上下文长度
    // 当设置为 true 时，如果要嵌入的文本超过模型的最大上下文长度，
    // 调用 EmbedStrings 将返回错误
    // 可选。
    Truncate *bool `json:"truncate,omitempty"`
    
    // KeepAlive 控制此请求后模型在内存中保持加载的时间
    // 可选。默认值：5 分钟
    KeepAlive *time.Duration `json:"keep_alive,omitempty"`
    
    // Options 列出特定于模型的选项
    // 可选
    Options map[string]any `json:"options,omitempty"`
}
```

## 示例

查看 [examples](./examples/) 目录获取完整的使用示例。
