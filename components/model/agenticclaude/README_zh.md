# Claude Agentic 模型

这是一个面向 [Eino](https://github.com/cloudwego/eino) 的 Anthropic Claude 模型实现，满足 `AgenticModel` 组件接口，可无缝接入 Eino 的 Agent 能力，用于更复杂的自然语言生成与工具交互场景。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.AgenticModel`
- 易于与 Eino Agent 系统集成
- 支持灵活的模型参数配置
- 支持 Anthropic Messages API
- 支持流式响应
- 支持工具调用（函数工具、延迟加载工具、客户端工具搜索、Server Tool）
- 支持 Prompt 缓存
- 支持 AWS Bedrock 和 Google Vertex AI

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticclaude@latest
```

## 快速开始

下面是一个使用 `AgenticModel` 的快速示例：

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	am, err := agenticclaude.New(ctx, &agenticclaude.Config{
		BaseURL:   os.Getenv("CLAUDE_BASE_URL"),
		Model:     os.Getenv("CLAUDE_MODEL"),
		APIKey:    os.Getenv("CLAUDE_API_KEY"),
		MaxTokens: 4096,
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err: %v", err)
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what is the weather like in Beijing"),
	}

	msg, err := am.Generate(ctx, input, model.WithTools([]*schema.ToolInfo{
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
	}))
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	if meta := msg.ResponseMeta.ClaudeExtension; meta != nil {
		log.Printf("request_id: %s\n", meta.ID)
	}

	respBody, _ := sonic.MarshalIndent(msg, "  ", "  ")
	log.Printf("body: %s\n", string(respBody))
}
```

## 配置

可以通过 `agenticclaude.Config` 结构体对 `AgenticModel` 进行配置：

```go
type Config struct {
    // HTTPClient 指定用于发送 HTTP 请求的客户端。
    // 使用 Google Vertex AI 时不生效。
    // 可选。
    HTTPClient *http.Client

    // ByBedrock 指定使用 AWS Bedrock 的配置。
    // 可选。
    ByBedrock *BedrockConfig

    // ByGoogleVertexAI 指定使用 Google Vertex AI 的配置。
    // 可选。
    ByGoogleVertexAI *GoogleVertexAIConfig

    // BaseURL 自定义 API 端点 URL。
    // 用于指定不同的 API 端点，例如代理或企业部署。
    // 可选。
    BaseURL string

    // APIKey 是 Anthropic API 密钥。
    // 获取地址：https://console.anthropic.com/account/keys
    // 直接使用 Anthropic API 时必填。
    APIKey string

    // Model 指定使用的 Claude 模型。
    // 必填。
    Model string

    // MaxTokens 限制响应中的最大 token 数。
    // 范围：1 到模型的上下文长度。
    // 必填。
    MaxTokens int

    // StopSequences 指定自定义停止序列。
    // 模型在遇到这些序列中的任何一个时将停止生成。
    // 可选。
    StopSequences []string

    // DisableParallelToolUse 指定是否禁用并行工具调用。
    // 仅在设置了 AgenticToolChoice 时生效。
    // 可选。
    DisableParallelToolUse *bool

    // Thinking 指定 Claude 思考模式的配置。
    // 可选。
    Thinking *anthropic.ThinkingConfigParamUnion

    // CustomHeaders 指定 API 请求中包含的自定义 HTTP 标头。
    // 可用于传递额外的元数据或认证信息。
    // 可选。
    CustomHeaders map[string]string

    // ExtraFields 指定请求体中包含的额外字段。
    // 这些字段将合并到顶层 JSON 请求体中，覆盖具有相同键的任何现有字段。
    // 可选。
    //
    // 示例：
    //
    //	ExtraFields: map[string]any{
    //	    "reasoning_effort": "high",
    //	    "service_tier": "default",
    //	}
    //
    // 生成的请求体将为：
    //
    //	{
    //	    "model": "o1",
    //	    "messages": [...],
    //	    "reasoning_effort": "high",
    //	    "service_tier": "default"
    //	}
    ExtraFields map[string]any

    // CacheControl 配置自动提示缓存行为。
    // 非 nil 时，自动在请求中最后一个可缓存的 block 上应用 cache_control 标记。
    // 可选。
    CacheControl *anthropic.CacheControlEphemeralParam
}
```

## 高级用法

### 缓存

使用 `CacheControl` 为多轮对话启用自动缓存。设置后（非 nil），API 会自动在请求中最后一个可缓存的 block 上应用 cache_control 标记。

如需细粒度控制，可使用 `SetContentBlockCacheControl` 或 `SetToolInfoCacheControl` 手动在特定的 block 或 tool 上放置缓存断点。

```go
cacheCtrl := anthropic.NewCacheControlEphemeralParam()
cacheCtrl.TTL = anthropic.CacheControlEphemeralTTLTTL5m

am, err := agenticclaude.New(ctx, &agenticclaude.Config{
    BaseURL:      os.Getenv("CLAUDE_BASE_URL"),
    Model:        os.Getenv("CLAUDE_MODEL"),
    APIKey:       os.Getenv("CLAUDE_API_KEY"),
    MaxTokens:    4096,
    CacheControl: &cacheCtrl,
})
```

### 工具调用 (Tool Calling)

`AgenticModel` 支持工具调用，包括函数工具、延迟加载工具、客户端工具搜索和 Server Tool。

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
	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	am, err := agenticclaude.New(ctx, &agenticclaude.Config{
		BaseURL:   os.Getenv("CLAUDE_BASE_URL"),
		Model:     os.Getenv("CLAUDE_MODEL"),
		APIKey:    os.Getenv("CLAUDE_API_KEY"),
		MaxTokens: 4096,
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
		msg, recvErr := sResp.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	lastBlock := concatenated.ContentBlocks[len(concatenated.ContentBlocks)-1]
	if lastBlock.Type != schema.ContentBlockTypeFunctionToolCall {
		log.Fatalf("last block is not function tool call, type: %s", lastBlock.Type)
	}

	toolCall := lastBlock.FunctionToolCall
	toolResultMsg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: toolCall.CallID,
				Name:   toolCall.Name,
				Content: []*schema.FunctionToolResultContentBlock{
					{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: "20 degrees"}},
				},
			}),
		},
	}

	secondInput := append(firstInput, concatenated, toolResultMsg)

	gResp, err := am.Generate(ctx, secondInput, opts...)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	if meta := concatenated.ResponseMeta.ClaudeExtension; meta != nil {
		log.Printf("request_id: %s\n", meta.ID)
	}

	respBody, _ := sonic.MarshalIndent(gResp, "  ", "  ")
	log.Printf("body: %s\n", string(respBody))
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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	am, err := agenticclaude.New(ctx, &agenticclaude.Config{
		BaseURL:   os.Getenv("CLAUDE_BASE_URL"),
		Model:     os.Getenv("CLAUDE_MODEL"),
		APIKey:    os.Getenv("CLAUDE_API_KEY"),
		MaxTokens: 4096,
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	serverTools := []*agenticclaude.ServerToolConfig{
		{
			WebSearch20260209: &anthropic.WebSearchTool20260209Param{},
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			ServerTool: &schema.AllowedServerTool{
				Name: string(agenticclaude.ServerToolNameWebSearch),
			},
		},
	}

	opts := []model.Option{
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		agenticclaude.WithServerTools(serverTools),
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what's cloudwego/eino"),
	}

	resp, err := am.Stream(ctx, input, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, recvErr := resp.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	for _, block := range concatenated.ContentBlocks {
		if block.ServerToolCall != nil {
			serverToolArgs := block.ServerToolCall.Arguments.(*agenticclaude.ServerToolCallArguments)
			args, _ := sonic.MarshalIndent(serverToolArgs, "  ", "  ")
			log.Printf("server_tool_args: %s\n", string(args))
		}

		if block.ServerToolResult != nil {
			result := block.ServerToolResult.Content.(*agenticclaude.ServerToolResult)
			resultJSON, _ := sonic.MarshalIndent(result, "  ", "  ")
			log.Printf("server_tool_result: %s\n", string(resultJSON))
		}
	}

	if meta := concatenated.ResponseMeta.ClaudeExtension; meta != nil {
		log.Printf("request_id: %s\n", meta.ID)
	}

	respBody, _ := sonic.MarshalIndent(concatenated, "  ", "  ")
	log.Printf("body: %s\n", string(respBody))
}
```

更多示例请参考 `examples` 目录。
