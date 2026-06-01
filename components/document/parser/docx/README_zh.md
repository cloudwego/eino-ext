# Eino - DOCX 解析器

Docx 解析器是 Eino 的文档解析组件，实现了用于解析 Microsoft Word (docx) 文件的 'Parser' 接口。该包旨在解析 Microsoft Word (`.docx`) 文件并将其内容提取为结构化的纯文本格式。

## 📜 概述

`docx` 包提供了一个可配置的解析器，可以从 `io.Reader` 读取 `.docx` 文档。它可以选择性地提取文档的各个部分，包括正文文本、页眉、页脚和表格。输出可以作为单个合并文档返回，也可以拆分为单独的部分。

该解析器基于 `docx2md` 库构建，用于处理 `.docx` 文件的底层 XML 结构。

## ✨ 功能特性

+ **正文内容提取**：从主文档正文解析所有段落和文本
+ **可配置提取**：轻松启用或禁用包含：
    - 页眉
    - 页脚
    - 表格
+ **灵活输出**：
    - 将所有提取的内容合并为单个文档
    - 将内容拆分为单独的部分（例如，正文、页眉、页脚）
+ 基于 `docx2md` 库的轻量级封装

## ⚙️ 配置

`DocxParser` 的行为由 `Config` 结构体控制。如果未提供配置，则使用默认配置。

以下是可用的配置选项：

| 字段 | 类型 | 描述 | 默认值 |
| --- | --- | --- | --- |
| `ToSections` | `bool` | 如果为 `true`，将提取的内容拆分为不同的部分（正文、页眉、页脚等）。否则，合并所有内容。 | `false` |
| `IncludeComments` | `bool` | **已弃用**。由于底层解析库的更改，不再支持此选项。 | `false` |
| `IncludeHeaders` | `bool` | 如果为 `true`，包含所有文档页眉的内容。 | `false` |
| `IncludeFooters` | `bool` | 如果为 `true`，包含所有文档页脚的内容。 | `false` |
| `IncludeTables` | `bool` | 如果为 `true`，提取并格式化文档中所有表格的内容。 | `false` |


## 🚀 使用示例

下面是如何使用 `DocxParser` 读取 `.docx` 文件并打印其内容的基本示例。

首先，确保你拥有必要的包：

```bash
go get github.com/cloudwego/eino
go get github.com/eino-contrib/docx2md
```

### 示例

查看以下示例了解更多用法：

- [测试数据示例](./examples/testdata/)

### 配置选项

```go
config := &docx.Config{
    ToSections:      false, // 是否按部分拆分内容
    IncludeHeaders:  true,  // 在输出中包含页眉
    IncludeFooters:  true,  // 在输出中包含页脚
    IncludeTables:   true,  // 包含表格内容
}

parser, err := docx.NewDocxParser(context.Background(), config)
```

## 输出格式

当 `ToSections` 为 `false`（默认）时，解析器返回单个文档，其中包含所有连接的内容。

当 `ToSections` 为 `true` 时，解析器返回按部分类型拆分的多个文档：

+ "main" - 主文档内容
+ "headers" - 页眉内容（如果启用）
+ "footers" - 页脚内容（如果启用）
+ "tables" - 表格内容（如果启用）

每个部分前面都有一个标题行（例如，"=== MAIN CONTENT ==="）来标识部分类型。

## 限制

+ 目前仅提取纯文本内容
+ 不保留格式、图像和其他富内容
+ 复杂的表格结构可能无法完美呈现
+ 关于注释的说明：`IncludeComments` 选项现已弃用。底层 DOCX 解析库从 AGPL 许可的依赖项切换到 `docx2md`（MIT 许可）以解决与项目的 Apache 2.0 许可证的许可冲突。新库目前不支持注释提取。
