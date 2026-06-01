# VikingDB Indexer

[English](README.md) | 中文

[Eino](https://github.com/cloudwego/eino) 的 VikingDB 索引器实现，实现了 `Indexer` 接口。该组件集成火山引擎 VikingDB 服务，用于存储带有向量嵌入的文档。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/indexer.Indexer`
- 易于集成 Eino 索引系统
- 支持火山引擎 VikingDB 集合
- 多种 embedding 模式：
  - 内置 VikingDB embedding（Embedding V2）
  - 使用自定义 embedder 的自定义 embedding
  - 平台向量化的多模态支持
- 支持稠密和稀疏向量
- 批量更新操作以提升性能
- 可配置的 TTL 和自定义字段支持

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb@latest
```

## 快速开始

### 使用内置 VikingDB Embedding

```go
import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// 使用内置 embedding 创建 VikingDB 索引器
	indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: true,
			ModelName:  "bge-large-zh",  // 内置 embedding 模型
			UseSparse:  false,            // 设置为 true 以启用稀疏向量
		},
		AddBatchSize: 5,
	})

	// 准备文档
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
		},
	}

	// 索引文档
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("index error: %v\n", err)
		return
	}
	fmt.Println("indexed ids:", ids)
}
```

### 使用自定义 Embedding

```go
import (
	"context"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// 创建自定义 embedding 组件
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 使用自定义 embedding 创建 VikingDB 索引器
	indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  emb,  // 使用自定义 embedding
		},
		AddBatchSize: 5,
	})

	docs := []*schema.Document{
		{ID: "1", Content: "Sample document"},
	}

	ids, _ := indexer.Store(ctx, docs)
	fmt.Println("indexed ids:", ids)
}
```

## 配置

可以使用 `IndexerConfig` 结构体配置索引器：

```go
type IndexerConfig struct {
    // VikingDB 连接设置
    Host              string  // 必填：VikingDB 服务主机
    Region            string  // 必填：服务区域（例如 "cn-beijing"）
    AK                string  // 必填：访问密钥
    SK                string  // 必填：密钥
    Scheme            string  // 选填："http" 或 "https"（默认："https"）
    ConnectionTimeout int64   // 选填：连接超时时间（秒）

    // 集合设置
    Collection string  // 必填：集合名称

    // 多模态支持
    // 如果数据集在平台上进行向量化，设置为 true
    // 为 true 时，无需配置 EmbeddingConfig
    WithMultiModal bool

    // Embedding 配置
    EmbeddingConfig EmbeddingConfig

    // 选填：更新操作的批量大小（默认：5）
    AddBatchSize int
}

type EmbeddingConfig struct {
    // UseBuiltin 决定是否使用 VikingDB 内置 embedding（Embedding V2）
    // 为 true 时，配置 ModelName 和 UseSparse
    // 为 false 时，配置 Embedding
    // 参见：https://www.volcengine.com/docs/84313/1254617
    UseBuiltin bool

    // ModelName 指定内置模型名称
    // 示例："bge-large-zh"、"text-embedding-ada-002"
    ModelName string

    // UseSparse 决定是否返回稀疏向量
    // 支持稀疏向量的模型：设为 true 返回稠密+稀疏，false 仅返回稠密
    // 不支持稀疏向量的模型：设为 true 会导致错误
    UseSparse bool

    // Embedding 在 UseBuiltin 为 false 时使用
    // 如果提供（从这里或 indexer.Option），它优先于内置方法
    Embedding embedding.Embedder
}
```

## 高级特性

### 自定义字段和 TTL

您可以使用 metadata 为文档添加自定义字段和 TTL：

```go
import "github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"

doc := &schema.Document{
    ID:      "1",
    Content: "Sample content",
    MetaData: map[string]any{
        volc_vikingdb.ExtraKeyVikingDBFields: map[string]any{
            "category": "technology",
            "author":   "John Doe",
        },
        volc_vikingdb.ExtraKeyVikingDBTTL: int64(86400), // 24 小时（秒）
    },
}

ids, _ := indexer.Store(ctx, []*schema.Document{doc})
```

### 稠密和稀疏向量

使用支持稀疏向量的模型的内置 embedding 时：

```go
indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
    // ... 其他配置
    EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
        UseBuiltin: true,
        ModelName:  "model-with-sparse-support",
        UseSparse:  true,  // 启用稀疏向量提取
    },
})
```

## 存储格式

文档在 VikingDB 中存储时包含以下字段：
- `id`：文档 ID
- `content`：文档内容
- `vector`：稠密向量 embedding
- `sparse_vector`：稀疏向量（如果启用）
- 来自 metadata 的额外自定义字段

## 更多详情

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [VikingDB 文档](https://www.volcengine.com/docs/84313/1254617)
- [VikingDB Go SDK](https://github.com/volcengine/volc-sdk-golang)
## 示例

查看以下示例了解更多用法：

- [嵌入索引器](./examples/embed_indexer/)
- [基础索引器](./examples/indexer/)

