# Eino HTML 标题分割器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的基于 HTML 标题的分割器组件，实现了 `Transformer` 接口，根据标题标签（h1、h2、h3 等）分割 HTML 文档，并将标题层次结构保留为元数据。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Transformer` 接口
- 基于标题标签（h1-h6）分割 HTML 内容
- 在文档元数据中保留标题层次结构
- 可自定义标题到元数据键的映射
- 可选的自定义 ID 生成器用于分割块
- 维护标题之间的父子关系
- 易于集成到 Eino 工作流

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/html
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
)

func main() {
	ctx := context.Background()

	splitter, err := html.NewHeaderSplitter(ctx, &html.HeaderConfig{
		Headers: map[string]string{
			"h1": "章节",
			"h2": "小节",
		},
	})
	if err != nil {
		log.Fatalf("html.NewHeaderSplitter failed, err=%v", err)
	}

	htmlContent := `
		<h1>第一章</h1>
		<p>第一章的介绍文本。</p>
		<h2>第 1.1 节</h2>
		<p>第 1.1 节的内容。</p>
		<h2>第 1.2 节</h2>
		<p>第 1.2 节的内容。</p>
		<h1>第二章</h1>
		<p>第二章的介绍文本。</p>
	`

	docs := []*schema.Document{
		{
			Content: htmlContent,
			ID:      "doc-1",
		},
	}

	splits, err := splitter.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("splitter.Transform failed, err=%v", err)
	}

	for i, split := range splits {
		log.Printf("分割 %d:", i+1)
		log.Printf("  内容: %s", split.Content)
		log.Printf("  元数据: %v", split.MetaData)
	}
}
```

## 配置说明

分割器可以通过 `HeaderConfig` 结构体进行配置：

```go
type HeaderConfig struct {
    // Headers 指定要识别的标题及其在元数据中的名称
    // 键格式: "h1", "h2", "h3" 等
    // 值: 此标题级别的元数据键名称
    // 例子: {"h1": "标题", "h2": "章节", "h3": "小节"}
    Headers map[string]string
    
    // IDGenerator 是用于为分割块生成新 ID 的可选函数
    // 如果为 nil，所有分割都将使用原始文档 ID
    // 例子: func(ctx context.Context, originalID string, splitIndex int) string {
    //     return fmt.Sprintf("%s_chunk_%d", originalID, splitIndex)
    // }
    IDGenerator IDGenerator
}
```

## 输出示例

给定 HTML 输入：

```html
<h1>第一章</h1>
<p>介绍文本</p>
<h2>第 1.1 节</h2>
<p>章节内容</p>
<h2>第 1.2 节</h2>
<p>更多内容</p>
```

使用配置 `Headers: {"h1": "章节", "h2": "小节"}`，您将得到：

**分割 1:**
```go
{
    Content: "介绍文本",
    MetaData: {
        "章节": "第一章"
    }
}
```

**分割 2:**
```go
{
    Content: "章节内容",
    MetaData: {
        "章节": "第一章",
        "小节": "第 1.1 节"
    }
}
```

**分割 3:**
```go
{
    Content: "更多内容",
    MetaData: {
        "章节": "第一章",
        "小节": "第 1.2 节"
    }
}
```

## 自定义 ID 生成器

```go
idGenerator := func(ctx context.Context, originalID string, splitIndex int) string {
    return fmt.Sprintf("%s_split_%d", originalID, splitIndex)
}

splitter, err := html.NewHeaderSplitter(ctx, &html.HeaderConfig{
    Headers: map[string]string{
        "h1": "标题",
        "h2": "章节",
    },
    IDGenerator: idGenerator,
})
```

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    htmlSplitter "github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
)

splitter, _ := htmlSplitter.NewHeaderSplitter(ctx, &htmlSplitter.HeaderConfig{
    Headers: map[string]string{"h1": "标题", "h2": "章节"},
})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(splitter)

run, _ := chain.Compile(ctx)
splitDocs, _ := run.Invoke(ctx, docs)
```

## 工作原理

1. **解析 HTML**：分割器将 HTML 内容解析为 DOM 树
2. **识别标题**：它识别配置中指定的标题（例如 h1、h2）
3. **分割内容**：当找到标题时，其之前的内容成为单独的块
4. **跟踪层次结构**：标题文本存储为元数据，维护父子关系
5. **重置层次结构**：当遇到相同或更高级别的标题时，较低级别的标题将被清除

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
