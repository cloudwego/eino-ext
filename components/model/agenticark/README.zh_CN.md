# Volcengine Ark Agentic Model

基于 [Eino](https://github.com/cloudwego/eino) 的火山引擎 Ark 模型实现，实现了 `AgenticModel` 组件接口。这使得该模型能够无缝集成到 Eino 的 Agent 能力中，提供增强的自然语言处理和生成功能。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel` 接口
- 易于集成到 Eino 的 agent 系统中
- 可配置的模型参数
- 支持 Responses API
- 支持流式响应 (Streaming)
- 支持工具调用 (Tools)，包括函数工具 (Function Tools)、MCP 工具 (MCP Tools) 和服务器工具 (Server Tools)
- 支持前缀缓存 (Prefix Cache) 和多轮对话自动缓存

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticark@latest
```

## 快速开始

以下是如何使用 `AgenticModel` 的一个快速示例：

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// 获取 ARK_API_KEY 和 ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	am, err := agenticark.New(ctx, &agenticark.Config{
		Model:  os.Getenv("ARK_MODEL_ID"),
		APIKey: os.Getenv("ARK_API_KEY"),
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err: %v", err)
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what is the weather like in Beijing"),
	}

	msg, err := am.Generate(ctx, input)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	meta := msg.ResponseMeta.Extension.(*agenticark.ResponseMetaExtension)

	log.Printf("request_id: %s\n", meta.ID)
	respBody, _ := sonic.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

## 配置

可以使用 `agenticark.Config` 结构体配置 `AgenticModel`：

```go
type Config struct {
    // Timeout 指定等待 API 响应的最大持续时间。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。
    Timeout *time.Duration

    // HTTPClient 指定用于发送 HTTP 请求的客户端。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。默认值：&http.Client{Timeout: Timeout}
    HTTPClient *http.Client

    // RetryTimes 指定失败 API 调用的重试次数。
    // 可选。
    RetryTimes *int

    // BaseURL 指定 Ark 服务端点的基准 URL。
    // 可选。
    BaseURL string

    // Region 指定 Ark 服务所在的地理区域。
    // 可选。
    Region string

    // APIKey 指定用于认证的 API 密钥。
    // 需要提供 APIKey 或同时提供 AccessKey 和 SecretKey。
    // 如果同时提供两种认证方式，APIKey 优先。
    // 详情参见：https://www.volcengine.com/docs/82379/1298459
    APIKey string

    // AccessKey 指定用于认证的访问密钥。
    // 必须与 SecretKey 配合使用。
    AccessKey string

    // SecretKey 指定用于认证的秘密密钥。
    // 必须与 AccessKey 配合使用。
    SecretKey string

    // Model 指定 Ark 平台上的模型端点标识符。
    // 详情参见：https://www.volcengine.com/docs/82379/1298454
    // 必填。
    Model string

    // MaxTokens 指定响应中生成的最大 token 数。
    // 可选。
    MaxTokens *int

    // Temperature 控制模型输出的随机性。
    // 较低的值（如 0.2）使输出更集中和确定。
    // 较高的值（如 1.0）使输出更具创造性和多样性。
    // 范围：0.0 到 2.0。
    // 可选。
    Temperature *float32

    // TopP 通过核采样控制多样性，是 Temperature 的替代方案。
    // TopP 指定 token 选择的累积概率阈值。
    // 例如，0.1 表示仅考虑概率质量前 10% 的 token。
    // 建议修改 Temperature 或 TopP 其中之一，但不要同时修改。
    // 范围：0.0 到 1.0。
    // 可选。
    TopP *float32

    // ServiceTier 指定请求使用的服务层级。
    // 可选。
    ServiceTier *responses.ResponsesServiceTier_Enum

    // Text 指定文本生成配置选项。
    // 可选。
    Text *responses.ResponsesText

    // Thinking 控制模型是否使用深度思考模式。
    // 可选。
    Thinking *responses.ResponsesThinking

    // Reasoning 指定模型推理过程的力度级别。
    // 可选。
    Reasoning *responses.ResponsesReasoning

    // EnablePassBackReasoning 控制模型是否在下一次请求中传回推理项。
    // 注意 doubao 1.6 不支持传回推理项。
    // 可选。默认值：true
    EnablePassBackReasoning *bool

    // MaxToolCalls 指定模型在单次响应中可进行的最大工具调用次数。
    // 可选。
    MaxToolCalls *int64

    // ParallelToolCalls 决定模型是否可以同时调用多个工具。
    // 可选。
    ParallelToolCalls *bool

    // ServerTools 指定模型可用的服务器端工具。
    // 可选。
    ServerTools []*ServerToolConfig

    // MCPTools 指定模型可用的 MCP（模型上下文协议）工具。
    // 可选。
    MCPTools []*responses.ToolMcp

    // EnableAutoCache 控制是否开启多轮对话自动缓存。
    // 启用后，对话轮次将被存储，模型通过定位输入中最近的缓存消息（通过 ResponseMeta 中的 Response ID）
    // 自动维护上下文。该缓存消息及其之前的所有输入将从请求中排除。
    // 如果缓存消息失效，可以调用 InvalidateMessageCaches 临时使缓存无效。
    // 可选。
    EnableAutoCache bool

    // ExpireAtSec 指定自动缓存或前缀缓存的过期 Unix 时间戳（秒）。
    // 可选。
    ExpireAtSec *int64

    // ContextManagement 指定上下文管理策略，帮助模型有效利用上下文窗口。
    // 支持清除思维链内容和工具调用内容。
    // 可选。
    ContextManagement *contextmanagement.ContextManagement

    // CustomHeaders 指定 API 请求中包含的自定义 HTTP 标头。
    // 可选。
    CustomHeaders map[string]string
}
```

## 高级用法

### 缓存

使用 `EnableAutoCache` 开启多轮对话自动缓存。若某条缓存消息已经失效，可以调用 `InvalidateMessageCaches` 临时跳过该缓存。

如果需要显式复用前缀缓存，可以先调用 `CreatePrefixCache`，再通过 `WithHeadPreviousResponseID` 传入返回的响应 ID。

```go
expireAtSec := time.Now().Add(time.Hour).Unix()

am, err := agenticark.New(ctx, &agenticark.Config{
	Model:           os.Getenv("ARK_MODEL_ID"),
	APIKey:          os.Getenv("ARK_API_KEY"),
	EnableAutoCache: true,
	ExpireAtSec:     &expireAtSec,
})
```

### 工具调用 (Tool Calling)

`AgenticModel` 支持工具调用，包括函数工具、MCP 工具和服务器工具。

#### 函数工具示例

```go
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	// 获取 ARK_API_KEY 和 ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	am, err := agenticark.New(ctx, &agenticark.Config{
		Model:  os.Getenv("ARK_MODEL_ID"),
		APIKey: os.Getenv("ARK_API_KEY"),
		Thinking: &responses.ResponsesThinking{
			Type: responses.ThinkingType_disabled.Enum(),
		},
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	functionTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "get the weather in a city",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](
					orderedmap.WithInitialData(
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "city",
							Value: &jsonschema.Schema{
								Type:        "string",
								Description: "the city to get the weather",
							},
						},
					),
				),
				Required: []string{"city"},
			}),
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			FunctionName: "get_weather",
		},
	}

	opts := []model.Option{
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		model.WithTools(functionTools),
	}

	firstInput := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what's the weather like in Beijing today"),
	}

	sResp, err := am.Stream(ctx, firstInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, err := sResp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", err)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	lastBlock := concatenated.ContentBlocks[len(concatenated.ContentBlocks)-1]
	
	toolCall := lastBlock.FunctionToolCall
	toolResultMsg := schema.FunctionToolResultAgenticMessage(toolCall.CallID, toolCall.Name, "20 degrees")

	secondInput := append(firstInput, concatenated, toolResultMsg)

	gResp, err := am.Generate(ctx, secondInput)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	meta := concatenated.ResponseMeta.Extension.(*agenticark.ResponseMetaExtension)
	log.Printf("request_id: %s\n", meta.ID)

	respBody, _ := sonic.MarshalIndent(gResp, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```


#### 服务器工具示例

```go
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	am, err := agenticark.New(ctx, &agenticark.Config{
		Model:  os.Getenv("ARK_MODEL_ID"),
		APIKey: os.Getenv("ARK_API_KEY"),
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	serverTools := []*agenticark.ServerToolConfig{
		{
			WebSearch: &responses.ToolWebSearch{
				Type: responses.ToolType_web_search,
			},
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			ServerTool: &schema.AllowedServerTool{
				Name: string(agenticark.ServerToolNameWebSearch),
			},
		},
	}

	opts := []model.Option{
		agenticark.WithServerTools(serverTools),
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		agenticark.WithThinking(&responses.ResponsesThinking{
			Type: responses.ThinkingType_disabled.Enum(),
		}),
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what's the weather like in Beijing today"),
	}

	resp, err := am.Stream(ctx, input, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, err := resp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", err)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	meta := concatenated.ResponseMeta.Extension.(*agenticark.ResponseMetaExtension)
	for _, block := range concatenated.ContentBlocks {
		if block.ServerToolCall == nil {
			continue
		}

		serverToolArgs := block.ServerToolCall.Arguments.(*agenticark.ServerToolCallArguments)

		args, _ := sonic.MarshalIndent(serverToolArgs, "  ", "  ")
		log.Printf("server_tool_args: %s\n", string(args))
	}

	log.Printf("request_id: %s\n", meta.ID)
	respBody, _ := sonic.MarshalIndent(concatenated, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

更多示例请参考 `examples` 目录。
