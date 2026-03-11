# Eino S3 加载器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的 AWS S3 加载器组件，实现了 `Loader` 接口，可无缝集成到 Eino 的文档处理工作流中，用于从 AWS S3 存储桶加载文档。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Loader` 接口
- 从 AWS S3 存储桶加载文档
- 支持使用访问密钥/私密密钥进行 AWS 认证
- 可自定义解析器配置
- 内置回调支持
- 易于集成到 Eino 工作流

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/s3
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/s3"
)

func main() {
	ctx := context.Background()

	region := "us-east-1"
	accessKey := "your-access-key"
	secretKey := "your-secret-key"

	loader, err := s3.NewS3Loader(ctx, &s3.LoaderConfig{
		Region:           &region,
		AWSAccessKey:     &accessKey,
		AWSSecretKey:     &secretKey,
		UseObjectKeyAsID: true,
	})
	if err != nil {
		log.Fatalf("s3.NewS3Loader failed, err=%v", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: "s3://my-bucket/path/to/document.txt",
	})
	if err != nil {
		log.Fatalf("loader.Load failed, err=%v", err)
	}

	log.Printf("加载了 %d 个文档", len(docs))
	log.Printf("文档内容: %v", docs[0].Content)
}
```

## 配置说明

加载器可以通过 `LoaderConfig` 结构体进行配置：

```go
type LoaderConfig struct {
    // Region 是存储桶所在的 AWS 区域 (选填)
    // 例子: "us-east-1"
    Region *string
    
    // AWSAccessKey 是用于认证的 AWS 访问密钥 (选填)
    // 如果不提供，将使用默认的 AWS 凭证
    AWSAccessKey *string
    
    // AWSSecretKey 是用于认证的 AWS 私密密钥 (选填)
    // 必须与 AWSAccessKey 一起提供
    AWSSecretKey *string
    
    // UseObjectKeyAsID 使用 S3 对象键作为文档 ID (选填)
    // 默认值: false
    UseObjectKeyAsID bool
    
    // Parser 指定用于文件内容的解析器 (选填)
    // 默认值: TextParser
    Parser parser.Parser
}
```

## URI 格式

加载器期望使用以下格式的 S3 URI：

```
s3://bucket-name/object-key
```

例如：
- `s3://my-bucket/documents/file.txt`
- `s3://data-bucket/reports/2024/report.pdf`

**注意**：目前不支持使用前缀进行批量加载（例如 `s3://bucket/prefix/`）。

## 认证

加载器支持两种认证方式：

1. **显式凭证**：在配置中提供 `AWSAccessKey` 和 `AWSSecretKey`
2. **默认 AWS 凭证**：如果未提供密钥，加载器将使用默认的 AWS 凭证链（环境变量、共享凭证文件、IAM 角色等）

## 在链中使用

```go
chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, err := chain.Compile(ctx)
if err != nil {
    log.Fatalf("chain.Compile failed, err=%v", err)
}

docs, err := run.Invoke(ctx, document.Source{URI: "s3://my-bucket/file.txt"})
```

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
