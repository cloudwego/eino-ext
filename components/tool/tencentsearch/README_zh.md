# 腾讯云联网搜索 Tool
联网搜索 API （Web Search API，wsa）来源于搜狗搜索，以互联网全网公开资源为基础，实现了从收录至召回排序全链路的智能搜索增强。合作伙伴通过检索词请求接口，以 JSON 形式返回搜索结果对应的排序信息、数据字段，可在业务场景中结合搜索结果的内容，扩充信息内容源，丰富展现效果，提升用户的查询满意度。

这是一个为 [Eino](https://github.com/cloudwego/eino) 实现的腾讯云联网搜索工具。该工具实现了 `InvokableTool` 接口，可以使用腾讯云 Web Search API (WSA) 与 Eino 的 ChatModel 交互系统和 `ToolsNode` 无缝集成，提供联网搜索能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/tool.InvokableTool` 接口
- 易于与 Eino 工具系统集成
- 可配置的搜索参数（搜索关键词、结果数量、站内搜索、时间范围、行业过滤、搜索模式）
- 简化的搜索结果，包含标题、链接、摘要和描述
- 支持自定义腾讯云 API Endpoint 配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/tool/tencentsearch
```

## 前置条件

使用此工具之前，您需要：

1. **获取腾讯云联网搜索密钥**：
   - 参考文档：<https://cloud.tencent.com/document/product/1806/121812>
   - 开通腾讯云“联网搜索”功能
   - 获取 SecretKey，SecretID

## 使用示例

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/tool/tencentsearch"
)

func main() {
	ctx := context.Background()

	// 初始化配置，建议从环境变量中读取密钥
	conf := &tencentsearch.Config{
		SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
		SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
		Endpoint:  "wsa.tencentcloudapi.com", // 默认
		Mode:      0,                         // 0-自然检索，1-多模态VR，2-混合
		Cnt:       10,                        // 返回结果条数
	}

	// 创建 Tool
	searchTool, err := tencentsearch.NewTool(ctx, conf)
	if err != nil {
		panic(err)
	}

	// 使用 Tool 进行搜索
	input := `{"query": "eino framework"}`
	resp, err := searchTool.InvokableRun(ctx, input)
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
- `Cnt`: 每次请求默认返回的搜索结果条数，合法值为 `10/20/30/40/50`；非法值会自动回退为 `10`。
- `ToolName`: Tool 名称 (默认: `tencent_search`)。
- `ToolDesc`: Tool 的描述信息。

`SecretID` 和 `SecretKey` 初始化时必填；缺失时分别返回 `secret_id is required`、`secret_key is required`。

## 搜索请求 (SearchRequest)

```go
type SearchRequest struct {
	Query    string  `json:"query" jsonschema:"required" jsonschema_description:"queried string to the search engine"`
	Mode     *int64  `json:"mode,omitempty" jsonschema_description:"0-natural, 1-VR, 2-mixed"`
	Site     *string `json:"site,omitempty" jsonschema_description:"site domain to search within"`
	FromTime *int64  `json:"from_time,omitempty" jsonschema_description:"start time timestamp in seconds"`
	ToTime   *int64  `json:"to_time,omitempty" jsonschema_description:"end time timestamp in seconds"`
	Industry *string `json:"industry,omitempty" jsonschema_description:"industry filter, one of gov/news/acad/finance"`
	Cnt      *uint64 `json:"cnt,omitempty" jsonschema_description:"number of search results to return, 10/20/30/40/50"`
}
```

- `Query`: 必填搜索词，不能为空；为空时返回 `query is required`。
- `Mode`: 可选返回结果类型，`0` 自然检索，`1` 多模态VR，`2` 混合。
- `Site`: 可选站内搜索域名。
- `FromTime`: 可选起始时间，秒级 Unix 时间戳。
- `ToTime`: 可选结束时间，秒级 Unix 时间戳。
- `Industry`: 可选行业过滤参数，可选值为 `gov/news/acad/finance`，分别表示党政机关、权威媒体、学术（英文）、金融；
- `Cnt`: 可选返回结果数量，合法值为 `10/20/30/40/50`；非法值会自动回退为 `10`。

## 示例

### 示例 1：基本搜索

```go
searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})
if err != nil {
	return err
}

req := tencentsearch.SearchRequest{
	Query: "人工智能",
}
args, _ := json.Marshal(req)
resp, err := searchTool.InvokableRun(ctx, string(args))
if err != nil {
	return err
}
```

### 示例 2：带数量限制和行业过滤的搜索

```go
searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	Cnt:       10, // 支持每次返回 10/20/30/40/50 个结果
})
if err != nil {
	return err
}

cnt := uint64(20)
industry := "news"
req := tencentsearch.SearchRequest{
	Query:    "今日财经新闻",
	Industry: &industry,
	Cnt:      &cnt,
}
args, _ := json.Marshal(req)
resp, err := searchTool.InvokableRun(ctx, string(args))
if err != nil {
	return err
}
```

### 示例 3：与 Eino ToolsNode 集成

```go
import (
	"github.com/cloudwego/eino/components/tool"
)

searchTool, err := tencentsearch.NewTool(ctx, &tencentsearch.Config{
	SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
	SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
})
if err != nil {
	return err
}

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

1. **工具创建**：使用您的腾讯云 API 凭据和配置初始化工具。
2. **请求处理**：调用时，工具接收带有查询参数的 JSON 格式 `SearchRequest`。
3. **API 调用**：工具使用指定的参数调用腾讯云 Web Search API。
4. **响应简化**：原始腾讯云 API 响应被简化为仅包含基本字段（标题、链接、摘要、描述）。
5. **JSON 响应**：简化的结果作为 JSON 字符串返回，便于使用。

## API 限制

请注意腾讯云联网搜索 API 的限制：
- Query 参数不能为空
- 每次查询可返回 10/20/30/40/50 个结果
- `Industry` 参数仅尊享版支持
- 工具会在调用 API 前将非法 `Cnt` 值自动规范化为 `10`

## 更多详情

- [Tencent Cloud 联网搜索](https://cloud.tencent.com/document/api/1806/121812)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [示例代码](examples/main.go)
