# Eino HTML 解析器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的 HTML 解析器组件，实现了 `Parser` 接口，可无缝集成到 Eino 的文档处理工作流中，用于将 HTML 内容解析为结构化文档。

## 特性

- 实现 `github.com/cloudwego/eino/components/document/parser.Parser` 接口
- 将 HTML 内容解析为纯文本
- 从 HTML 中提取元数据（标题、描述、语言、字符集）
- 使用 CSS 选择器语法自定义内容选择器
- 使用 bluemonday 进行 HTML 清理
- 易于集成到 Eino 工作流

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/parser/html
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"strings"

	"github.com/cloudwego/eino-ext/components/document/parser/html"
)

func main() {
	ctx := context.Background()

	parser, err := html.NewParser(ctx, &html.Config{})
	if err != nil {
		log.Fatalf("html.NewParser failed, err=%v", err)
	}

	htmlContent := `
		<!DOCTYPE html>
		<html lang="zh-CN">
		<head>
			<meta charset="UTF-8">
			<meta name="description" content="示例页面">
			<title>示例页面</title>
		</head>
		<body>
			<h1>你好世界</h1>
			<p>这是一个示例 HTML 文档。</p>
		</body>
		</html>
	`

	reader := strings.NewReader(htmlContent)
	docs, err := parser.Parse(ctx, reader)
	if err != nil {
		log.Fatalf("parser.Parse failed, err=%v", err)
	}

	log.Printf("内容: %s", docs[0].Content)
	log.Printf("标题: %s", docs[0].MetaData[html.MetaKeyTitle])
	log.Printf("描述: %s", docs[0].MetaData[html.MetaKeyDesc])
	log.Printf("语言: %s", docs[0].MetaData[html.MetaKeyLang])
}
```

## 配置说明

解析器可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // Selector 是用于提取特定内容的 CSS 选择器 (选填)
    // 例子: "body" 用于 <body>, "#content" 用于 <div id="content">
    // 默认值: 整个文档
    Selector *string
}
```

## 元数据

解析器会自动提取并为解析的文档添加以下元数据：

- `_title`: 来自 `<title>` 标签的文档标题
- `_description`: 来自 `<meta name="description">` 标签的描述
- `_language`: 来自 `<html lang="">` 属性的语言
- `_charset`: 来自 `<meta charset="">` 标签的字符编码
- `_source`: 源 URI（如果通过解析器选项提供）

## 使用自定义选择器

您可以从 HTML 的特定部分提取内容：

```go
bodySelector := "body"
parser, err := html.NewParser(ctx, &html.Config{
    Selector: &bodySelector,
})

contentSelector := "#main-content"
parser, err := html.NewParser(ctx, &html.Config{
    Selector: &contentSelector,
})
```

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/components/document"
    htmlParser "github.com/cloudwego/eino-ext/components/document/parser/html"
)

parser, _ := htmlParser.NewParser(ctx, &htmlParser.Config{})
loader, _ := urlLoader.NewURLLoader(ctx, &urlLoader.LoaderConfig{
    Parser: parser,
})

chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, document.Source{URI: "https://example.com"})
```

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
