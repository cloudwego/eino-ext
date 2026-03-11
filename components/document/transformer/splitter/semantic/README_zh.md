# Eino 语义分割器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的基于语义的文档分割器组件，实现了 `Transformer` 接口，使用嵌入向量根据语义相似性分割文档，确保语义相关的内容保持在一起。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Transformer` 接口
- 使用嵌入向量基于语义相似性分割文档
- 可配置的缓冲区大小以实现上下文感知分割
- 可自定义分隔符用于初始文本分段
- 强制执行最小块大小
- 基于百分位数的阈值确定分割点
- 可选的自定义 ID 生成器用于分割块
- 易于集成到 Eino 工作流

## 工作原理

1. **初始分割**：使用分隔符（换行符、句号等）分割文本
2. **上下文缓冲**：将相邻句子与缓冲区结合以创建富含上下文的块
3. **嵌入**：使用提供的嵌入器对每个缓冲块进行嵌入
4. **相似度计算**：计算相邻嵌入之间的余弦相似度
5. **阈值确定**：基于百分位数的阈值确定在哪里分割
6. **最终分割**：在相似度低于阈值的点分割文本
7. **合并小块**：小于最小大小的块与相邻块合并

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic
```

## 快速开始

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.Config{
		APIKey: "your-api-key",
	})
	if err != nil {
		log.Fatalf("openai.NewEmbedder failed, err=%v", err)
	}

	splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
		Embedding:    embedder,
		BufferSize:   1,
		MinChunkSize: 50,
		Percentile:   0.9,
	})
	if err != nil {
		log.Fatalf("semantic.NewSplitter failed, err=%v", err)
	}

	docs := []*schema.Document{
		{
			ID: "doc-1",
			Content: `人工智能简介。人工智能正在改变各个行业。
			机器学习是人工智能的一个子集。它从数据中学习。
			明天的天气预报。天气将晴朗温暖。`,
		},
	}

	splits, err := splitter.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("splitter.Transform failed, err=%v", err)
	}

	for i, split := range splits {
		log.Printf("分割 %d: %s", i+1, split.Content)
	}
}
```

## 配置说明

分割器可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // Embedding 用于生成向量以计算相似度（必填）
    Embedding embedding.Embedder
    
    // BufferSize 指定在嵌入期间在每个块之前和之后连接多少个块（选填）
    // 这允许块携带更多上下文信息
    // 默认值: 0
    // 例子: 1 表示每个块包括前后各 1 个句子
    BufferSize int
    
    // MinChunkSize 指定最小块大小（选填）
    // 小于此大小的块将与相邻块连接
    // 默认值: 0（无最小值）
    // 例子: 50 个字符
    MinChunkSize int
    
    // Separators 依次用于分割文本（选填）
    // 默认值: ["\n", ".", "?", "!"]
    // 例子: ["\n\n", "\n", "。"]
    Separators []string
    
    // LenFunc 用于计算字符串长度（选填）
    // 默认值: len() 函数
    // 例子: func(s string) int { return utf8.RuneCountInString(s) }
    LenFunc func(s string) int
    
    // Percentile 指定分割阈值（选填）
    // 如果两个块之间的相似度差异大于 X 百分位数，它们将被分割
    // 默认值: 0.9（第 90 百分位数）
    // 例子: 0.95 用于更严格的分割
    Percentile float64
    
    // IDGenerator 是用于生成新 ID 的可选函数（选填）
    // 默认值: 保留原始文档 ID
    IDGenerator IDGenerator
}
```

## 自定义设置示例

```go
splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
    Embedding:    embedder,
    BufferSize:   2,
    MinChunkSize: 100,
    Separators:   []string{"\n\n", "\n", "。", "！", "？"},
    Percentile:   0.95,
    IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
        return fmt.Sprintf("%s_semantic_%d", originalID, splitIndex)
    },
})
```

## 理解 BufferSize

`BufferSize` 参数对语义分割至关重要：

- **BufferSize = 0**：每个句子独立嵌入
- **BufferSize = 1**：每个句子与前后各 1 个句子一起嵌入
- **BufferSize = 2**：每个句子与前后各 2 个句子一起嵌入

更大的缓冲区大小提供更多上下文，但会增加嵌入成本。

## 理解 Percentile

`Percentile` 参数控制分割敏感度：

- **Percentile = 0.9**（90%）：在相似度较低的点分割（更多块）
- **Percentile = 0.95**（95%）：仅在相似度非常低的点分割（更少块）
- **Percentile = 0.5**（50%）：非常激进的分割（许多小块）

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    semanticSplitter "github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic"
)

splitter, _ := semanticSplitter.NewSplitter(ctx, &semanticSplitter.Config{
    Embedding:    embedder,
    BufferSize:   1,
    MinChunkSize: 50,
})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(splitter)

run, _ := chain.Compile(ctx)
splitDocs, _ := run.Invoke(ctx, docs)
```

## 性能考虑

- **嵌入成本**：每个句子（带缓冲区）都需要一次嵌入调用。对于长文档，这可能很昂贵。
- **缓冲区权衡**：更大的缓冲区提供更好的上下文，但会增加嵌入大小和成本。
- **块大小**：设置最小块大小有助于避免创建太多微小的块。

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
