# Eino 文件加载器

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的文件加载器组件，实现了 `Loader` 接口，可无缝集成到 Eino 的文档处理工作流中，用于加载本地文件。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Loader` 接口
- 易于集成到 Eino 工作流
- 支持根据文件扩展名自动解析
- 可自定义解析器配置
- 内置回调支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/file
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
)

func main() {
	ctx := context.Background()

	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
	})
	if err != nil {
		log.Fatalf("file.NewFileLoader failed, err=%v", err)
	}

	filePath := "./document.txt"
	docs, err := loader.Load(ctx, document.Source{
		URI: filePath,
	})
	if err != nil {
		log.Fatalf("loader.Load failed, err=%v", err)
	}

	log.Printf("doc content: %v", docs[0].Content)
	log.Printf("Extension: %s\n", docs[0].MetaData[file.MetaKeyExtension])
	log.Printf("Source: %s\n", docs[0].MetaData[file.MetaKeySource])
}
```

## 配置说明

加载器可以通过 `FileLoaderConfig` 结构体进行配置：

```go
type FileLoaderConfig struct {
    // UseNameAsID 使用文件名作为文档 ID
    // 可选。默认值：false
    UseNameAsID bool
    
    // Parser 指定用于文件内容的解析器
    // 可选。默认值：带 TextParser 后备的 ExtParser
    Parser parser.Parser
}
```

## 元数据

加载器会自动为加载的文档添加以下元数据：

- `_file_name`: 文件名（基本名称）
- `_extension`: 文件扩展名
- `_source`: 文件路径（URI）

## 在链中使用

```go
chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, err := chain.Compile(ctx)
if err != nil {
    log.Fatalf("chain.Compile failed, err=%v", err)
}

docs, err := run.Invoke(ctx, document.Source{URI: filePath})
```

## 示例

更多示例请参考 [examples](./examples) 目录：

- [fileloader](./examples/fileloader) - 基本文件加载用法
- [customloader](./examples/customloader) - 自定义加载器实现

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
