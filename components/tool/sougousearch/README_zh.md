# 搜狗搜索 Tool

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的 TencentCloud 自定义搜索工具 (底层为搜索搜索)。该工具实现了 `InvokableTool` 接口，可以使用 TencentCloud 的联网搜索 JSON API 与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成，提供增强的搜索功能。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数（搜索关键词、结果数量、分页参数、搜索模式）
- 简化的搜索结果，包含标题、链接、摘要和描述
- 支持自定义基础 URL 配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/sougousearch
```

## 前置条件

使用此工具之前，您需要：

1. **获取腾讯云联网搜索密钥**：
   - 参考文档：<https://cloud.tencent.com/document/product/1806/121811>
   - 开通腾讯云“联网搜索”功能
   - 获取 SecretKey，SecretID

## 使用示例

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/tool/sougousearch"
)

func main() {
	ctx := context.Background()

	// 初始化配置，建议从环境变量中读取密钥
	conf := &sougousearch.Config{
		SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
		SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
		Endpoint:  "wsa.tencentcloudapi.com", // 默认
		Mode:      0,                         // 0-自然检索，1-多模态VR，2-混合
		Cnt:       10,                        // 返回结果条数
	}

	// 创建 Tool
	sougouTool, err := sougousearch.NewTool(ctx, conf)
	if err != nil {
		panic(err)
	}

	// 使用 Tool 进行搜索
	input := `{"query": "eino framework"}`
	resp, err := sougouTool.InvokableRun(ctx, input)
	if err != nil {
		panic(err)
	}

	fmt.Println(resp)
}
```

## 配置项 (Config)

```go
type Config struct {
	SecretID  string `json:"secret_id"`  // 必需：腾讯云 SecretID
	SecretKey string `json:"secret_key"` // 必需：腾讯云 SecretKey
	Endpoint  string `json:"endpoint"`   // default: "wsa.tencentcloudapi.com"
	Mode      int64  `json:"mode"`       // default: 0
	Cnt       uint64 `json:"cnt"`        // default: 10

	ToolName string `json:"tool_name"`
	ToolDesc string `json:"tool_desc"`
}
```

- `SecretID`: 腾讯云 API SecretID。
- `SecretKey`: 腾讯云 API SecretKey。
- `Endpoint`: 请求节点 (默认: `wsa.tencentcloudapi.com`)。
- `Mode`: 返回结果类型，0-自然检索，1-多模态VR，2-混合 (默认: `0`)。
- `Cnt`: 每次请求默认返回的搜索结果条数 (默认: `10`)。
- `ToolName`: Tool 名称 (默认: `sougou_search`)。
- `ToolDesc`: Tool 的描述信息。

## 示例

### 示例 1：基本搜索

```go
searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})

req := sougousearch.SearchRequest{
	Query: "人工智能",
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### 示例 2：带分页限制的搜索

```go
searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	Cnt:       10, 					// 支持每次返回 10/20/30/40/50 个结果
})

cnt := uint64(3)
req := sougousearch.SearchRequest{
	Query: "Go并发编程",
	Cnt:   &cnt,
}
args, _ := json.Marshal(req)
resp, _ := searchTool.InvokableRun(ctx, string(args))
```

### 示例 3：与 Eino ToolsNode 集成

```go
import (
	"github.com/cloudwego/eino/components/tool"
)

searchTool, _ := sougousearch.NewTool(ctx, &sougousearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})

tools := []tool.BaseTool{searchTool}
// 在您的工作流中与 Eino 的 ToolsNode 一起使用
```

### 完整示例

完整的工作示例请参见 [examples/main.go](examples/main.go)

运行示例：

```bash
export TENCENTCLOUD_SECRET_KEY="your-api-key"
export TENCENTCLOUD_SECRET_ID="your-secret-id"
cd examples && go run main.go
```

## 工作原理

1. **工具创建**：使用您的 TencentCloud API 凭据和配置初始化工具。
2. **请求处理**：调用时，工具接收带有查询参数的 JSON 格式 `SearchRequest`。
3. **API 调用**：工具使用指定的参数调用 Sougou 的自定义搜索 JSON API。
4. **响应简化**：原始 TencentCloud API 响应被简化为仅包含基本字段（标题、链接、摘要、描述）。
5. **JSON 响应**：简化的结果作为 JSON 字符串返回，便于使用。

## API 限制

请注意 TencentCloud 联网搜索 API 的限制：

- 付费层级：30 元/千次，46 元/千次
- 每次查询可返回 10/20/30/40/50 个结果

## 更多详情

- [Tencent Cloud 联网搜索](https://cloud.tencent.com/document/api/1806/121812)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [示例代码](examples/main.go)

