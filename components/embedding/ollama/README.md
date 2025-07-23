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
		baseURL = "http://localhost:11434" // 默认本地
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
    // Timeout specifies the maximum duration to wait for API responses
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: no timeout
    Timeout time.Duration `json:"timeout"`
    
    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // BaseURL specifies the Ollama service endpoint URL
    // Format: http(s)://host:port
    // Optional. Default: "http://localhost:11434"
    BaseURL string `json:"base_url"`
    
    // Model specifies the ID of the model to use for embedding generation
    // Required
    Model string `json:"model"`
    
    // Truncate specifies whether to truncate text to model's maximum context length
    // When set to true, if text to embed exceeds the model's maximum context length,
    // a call to EmbedStrings will return an error
    // Optional.
    Truncate *bool `json:"truncate,omitempty"`
    
    // KeepAlive controls how long the model will stay loaded in memory following this request.
    // Optional. Default 5 minutes
    KeepAlive *time.Duration `json:"keep_alive,omitempty"`
    
    // Options lists model-specific options.
    // Optional
    Options map[string]any `json:"options,omitempty"`
}
```
