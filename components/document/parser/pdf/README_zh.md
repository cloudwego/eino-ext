# Eino PDF 解析器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的 PDF 解析器组件，实现了 `Parser` 接口，可无缝集成到 Eino 的文档处理工作流中，用于将 PDF 文件解析为纯文本文档。

## 特性

- 实现 `github.com/cloudwego/eino/components/document/parser.Parser` 接口
- 将 PDF 内容解析为纯文本
- 支持逐页解析或整个文档解析
- 字体缓存以提高性能
- 易于集成到 Eino 工作流

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/parser/pdf
```

## 快速开始

### 将整个 PDF 解析为单个文档

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
)

func main() {
	ctx := context.Background()

	parser, err := pdf.NewPDFParser(ctx, &pdf.Config{})
	if err != nil {
		log.Fatalf("pdf.NewPDFParser failed, err=%v", err)
	}

	file, err := os.Open("document.pdf")
	if err != nil {
		log.Fatalf("os.Open failed, err=%v", err)
	}
	defer file.Close()

	docs, err := parser.Parse(ctx, file)
	if err != nil {
		log.Fatalf("parser.Parse failed, err=%v", err)
	}

	log.Printf("解析了 %d 个文档", len(docs))
	log.Printf("内容: %s", docs[0].Content)
}
```

### 逐页解析 PDF

```go
parser, err := pdf.NewPDFParser(ctx, &pdf.Config{
	ToPages: true,
})

docs, err := parser.Parse(ctx, file)

for i, doc := range docs {
	log.Printf("第 %d 页: %s", i+1, doc.Content)
}
```

## 配置说明

解析器可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // ToPages 决定是否逐页解析 PDF (选填)
    // 如果为 true，每一页都会成为一个单独的文档
    // 如果为 false，整个 PDF 被解析为一个单独的文档
    // 默认值: false
    ToPages bool
}
```

## 解析器选项

您也可以使用解析器选项来配置解析器行为：

```go
docs, err := parser.Parse(ctx, file, 
    pdf.WithToPages(true),
)
```

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/components/document"
    pdfParser "github.com/cloudwego/eino-ext/components/document/parser/pdf"
)

parser, _ := pdfParser.NewPDFParser(ctx, &pdfParser.Config{})
loader, _ := fileLoader.NewFileLoader(ctx, &fileLoader.FileLoaderConfig{
    Parser: parser,
})

chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, document.Source{URI: "document.pdf"})
```

## 重要说明

⚠️ **Alpha 阶段**：此解析器处于 alpha 阶段，可能无法完美支持所有 PDF 用例。

当前限制：
- 可能无法在所有情况下保留空白和换行符
- 复杂的 PDF 布局可能无法最佳解析
- 某些 PDF 功能可能不完全支持

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
