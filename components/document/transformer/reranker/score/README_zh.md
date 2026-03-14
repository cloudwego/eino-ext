# Eino 基于评分的重排序器

[English](README.md) | 简体中文

## 简介

这是为 [Eino](https://github.com/cloudwego/eino) 实现的基于评分的文档重排序器组件，实现了 `Transformer` 接口，根据文档评分重新组织文档以优化 LLM 上下文处理。

## 特性

- 实现 `github.com/cloudwego/eino/components/document.Transformer` 接口
- 使用"首因和近因效应"基于评分重新排序文档
- 将高分文档放置在数组的开头和结尾
- 将低分文档放置在中间
- 支持从文档元数据使用自定义评分字段
- 针对 LLM 上下文窗口性能优化

## 为什么使用这种重排序策略？

此重排序器基于研究表明，当相关信息出现在输入上下文的开头或结尾时，LLM 表现出更好的性能，这被称为"首因和近因效应"。

该策略将：
- **高分文档** → 放在开头和结尾（LLM 更加关注的位置）
- **低分文档** → 放在中间（LLM 较少关注的位置）

参考：[Lost in the Middle: How Language Models Use Long Contexts](https://arxiv.org/abs/2307.03172)

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/reranker/score
```

## 快速开始

### 使用默认文档评分

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
)

func main() {
	ctx := context.Background()

	reranker, err := score.NewReranker(ctx, &score.Config{})
	if err != nil {
		log.Fatalf("score.NewReranker failed, err=%v", err)
	}

	docs := []*schema.Document{
		{Content: "文档 1", DocumentScore: 0.5},
		{Content: "文档 2", DocumentScore: 0.9},
		{Content: "文档 3", DocumentScore: 0.3},
		{Content: "文档 4", DocumentScore: 0.8},
		{Content: "文档 5", DocumentScore: 0.1},
	}

	reranked, err := reranker.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("reranker.Transform failed, err=%v", err)
	}

	for i, doc := range reranked {
		log.Printf("位置 %d: %s (评分: %.1f)", i, doc.Content, doc.Score())
	}
}
```

### 使用自定义评分字段

```go
scoreField := "custom_score"
reranker, err := score.NewReranker(ctx, &score.Config{
	ScoreFieldKey: &scoreField,
})

docs := []*schema.Document{
	{
		Content: "文档 1",
		MetaData: map[string]any{"custom_score": 0.5},
	},
	{
		Content: "文档 2",
		MetaData: map[string]any{"custom_score": 0.9},
	},
}

reranked, err := reranker.Transform(ctx, docs)
```

## 配置说明

重排序器可以通过 `Config` 结构体进行配置：

```go
type Config struct {
    // ScoreFieldKey 指定元数据中存储文档评分的键
    // 如果为 nil，使用 Document.Score() 方法（默认）
    // 例子: &"relevance_score"
    ScoreFieldKey *string
}
```

## 重排序示例

输入文档及其评分：`[0.5, 0.9, 0.3, 0.8, 0.1]`

按评分排序后（降序）：`[0.9, 0.8, 0.5, 0.3, 0.1]`

重排序后（交替高低分）：
```
位置 0: 0.9 (最高)     ← 开始：高度关注
位置 1: 0.5 (中高)
位置 2: 0.3 (中低)
位置 3: 0.1 (最低)
位置 4: 0.8 (第二高) ← 结束：高度关注
```

## 在链中使用

```go
import (
    "github.com/cloudwego/eino/compose"
    scoreReranker "github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
)

reranker, _ := scoreReranker.NewReranker(ctx, &scoreReranker.Config{})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(reranker)

run, _ := chain.Compile(ctx)
rerankedDocs, _ := run.Invoke(ctx, docs)
```

## 许可证

本项目采用 Apache License 2.0 许可证 - 详见 LICENSE 文件。
