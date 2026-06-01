# Markdown Header Splitter

[English](README.md) | 简体中文

一个基于标题级别拆分 Markdown 文档的转换器。该转换器专为 [Eino](https://github.com/cloudwego/eino) 设计，通过在标题边界处拆分来组织 Markdown 内容，同时将标题层次结构保存为元数据。

## 特性

- 基于可配置的标题级别（例如 `#`、`##`、`###`）拆分 Markdown 文档
- 将标题层次结构保存为文档元数据
- 正确处理代码块（不在代码块内拆分）
- 可选择性地从拆分内容中删除标题行
- 可自定义拆分文档的 ID 生成方式

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown
```

## 快速开始

```go
package main

import (
    "context"
    "log"
    
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
)

func main() {
    ctx := context.Background()
    
    transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
        Headers: map[string]string{
            "#":   "h1",
            "##":  "h2",
            "###": "h3",
        },
        TrimHeaders: true,
    })
    if err != nil {
        log.Fatalf("创建拆分器失败: %v", err)
    }
    
    doc := &schema.Document{
        Content: "# 标题\n引言内容\n## 第一节\n章节内容",
    }
    
    splitDocs, err := transformer.Transform(ctx, []*schema.Document{doc})
    if err != nil {
        log.Fatalf("转换失败: %v", err)
    }
    
    for _, doc := range splitDocs {
        log.Printf("内容: %s, 元数据: %v\n", doc.Content, doc.MetaData)
    }
}
```

## 配置

拆分器可以通过 `HeaderConfig` 结构体进行配置：

```go
type HeaderConfig struct {
    // Headers 指定要识别的标题及其在文档元数据中的名称。
    // 标题只能由 '#' 组成。
    // Key: 标题模式（例如 "##"）
    // Value: 元数据键名
    Headers map[string]string
    
    // TrimHeaders 指定结果是否包含标题行。
    // 如果为 true，标题行将从拆分内容中删除。
    // 如果为 false，标题行将包含在拆分内容中。
    TrimHeaders bool
    
    // IDGenerator 是一个可选函数，用于为拆分块生成新的 ID。
    // 如果为 nil，则所有拆分都将使用原始文档 ID。
    IDGenerator IDGenerator
}

type IDGenerator func(ctx context.Context, originalID string, splitIndex int) string
```

### 配置选项

- **Headers**（必需）：定义在哪些标题级别进行拆分以及在元数据中如何命名的映射
  - 键只能包含 `#` 字符
  - 示例：`{"#": "标题", "##": "章节", "###": "小节"}`

- **TrimHeaders**（可选，默认值：`false`）：控制拆分内容中是否包含标题行
  - `true`：从内容中删除标题行
  - `false`：在内容中保留标题行

- **IDGenerator**（可选）：用于为拆分文档生成 ID 的自定义函数
  - 默认行为：所有拆分使用原始文档 ID

## 示例

查看以下示例了解更多用法：

- [标题分割器](./examples/headersplitter/)

## 工作原理

1. **标题检测**：拆分器逐行扫描文档，查找以配置的标题模式开头的行。

2. **代码块处理**：正确处理代码块（由 ` ``` ` 或 `~~~` 包围）- 拆分器不会在代码块内拆分内容。

3. **标题层次结构**：遇到标题时，它会被添加到元数据中。如果找到相同或更高级别的新标题，则元数据中该级别或更低级别的先前标题将被替换。

4. **内容拆分**：在标题边界处拆分内容。每个拆分的文档包含：
   - 标题之间的内容
   - 包含层次结构中所有活动标题的元数据
   - 一个 ID（原始的或生成的）

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [示例代码](examples/headersplitter/main.go)
