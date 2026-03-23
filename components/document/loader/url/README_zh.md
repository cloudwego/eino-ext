# Eino URL 加载器

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的 URL 加载器组件，实现了 `Loader` 接口，可无缝集成到 Eino 的文档处理工作流中，用于从 URL 加载文档。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Loader` 接口
- 易于集成到 Eino 工作流
- 支持从 HTTP/HTTPS URL 加载文档
- 可自定义 HTTP 客户端和请求构建器
- 内置 HTML 解析器，支持可配置的选择器
- 内置回调支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/url
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/url"
)

func main() {
	ctx := context.Background()

	loader, err := url.NewLoader(ctx, &url.LoaderConfig{})
	if err != nil {
		log.Fatalf("NewLoader failed, err=%v", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: "https://example.com/page.html",
	})
	if err != nil {
		log.Fatalf("Load failed, err=%v", err)
	}

	for _, doc := range docs {
		log.Printf("Content: %s\n", doc.Content)
	}
}
```

## 配置说明

加载器可以通过 `LoaderConfig` 结构体进行配置：

```go
type LoaderConfig struct {
    // Parser 指定用于响应内容的解析器
    // 可选。默认值：带 body 选择器的 HTML 解析器
    Parser parser.Parser
    
    // Client 指定要使用的 HTTP 客户端
    // 可选。默认值：http.DefaultClient
    Client *http.Client
    
    // RequestBuilder 自定义 HTTP 请求
    // 可选。默认值：GET 请求构建器
    RequestBuilder func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error)
}
```

## 高级用法

### 使用代理的自定义 HTTP 客户端

```go
proxyURL, _ := url.Parse("http://proxy.example.com:8080")
client := &http.Client{
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    Client: client,
})
```

### 带身份验证的自定义请求构建器

```go
requestBuilder := func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", source.URI, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer YOUR_TOKEN")
    return req, nil
}

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    RequestBuilder: requestBuilder,
})
```

### 自定义解析器

```go
customParser, err := html.NewParser(ctx, &html.Config{
    Selector: &html.ArticleSelector,
})

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    Parser: customParser,
})
```

## 示例

查看以下示例了解更多用法：

- [身份认证](./examples/auth/)
- [目录路径](./examples/dirpath/)
- [HTML 加载](./examples/html/)
- [代理配置](./examples/proxy/)
- [测试数据示例](./examples/testdata/)

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
