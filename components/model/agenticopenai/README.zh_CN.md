# OpenAI Agentic Model

基于 [Eino](https://github.com/cloudwego/eino) 的 OpenAI 模型实现，实现了 `AgenticModel` 组件接口。这使得该模型能够无缝集成到 Eino 的 Agent 能力中，提供增强的自然语言处理和生成功能。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel` 接口
- 易于集成到 Eino 的 agent 系统中
- 可配置的模型参数
- 支持 Responses API
- 支持流式响应 (Streaming)
- 支持工具调用 (Tools)，包括函数工具 (Function Tools)、MCP 工具 (MCP Tools) 和服务器工具 (Server Tools)
- 支持 Azure OpenAI

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@latest
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
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/schema"
	openaischema "github.com/cloudwego/eino/schema/openai"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/responses"
	"github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	am, err := agenticopenai.New(ctx, &agenticopenai.Config{
		BaseURL: "https://api.agenticopenai.com/v1",
		Model:   os.Getenv("OPENAI_MODEL_ID"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Reasoning: &responses.ReasoningParam{
			Effort:  responses.ReasoningEffortLow,
			Summary: responses.ReasoningSummaryDetailed,
		},
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err: %v", err)
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what is the weather like in Beijing"),
	}

	am_, err := am.WithTools([]*schema.ToolInfo{
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
	})
	if err != nil {
		log.Fatalf("failed to create agentic model with tools, err: %v", err)
	}

	msg, err := am_.Generate(ctx, input)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	meta := msg.ResponseMeta.Extension.(*openaischema.ResponseMetaExtension)

	log.Printf("request_id: %s\n", meta.ID)
	respBody, _ := sonic.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

## 配置

可以使用 `agenticopenai.Config` 结构体配置 `AgenticModel`：

```go
type Config struct {
	// ByAzure 指定是否使用 Azure OpenAI 服务。
	// 可选。
	ByAzure bool

	// BaseURL 指定 OpenAI 服务端点的基准 URL。
	// 可选。
	BaseURL string

	// APIKey 指定用于认证的 API 密钥。
	// 必填。
	APIKey string

	// Timeout 指定等待 API 响应的最大持续时间。
	// 可选。
	Timeout *time.Duration

	// HTTPClient 指定用于发送 HTTP 请求的客户端。
	// 可选。
	HTTPClient *http.Client

	// MaxRetries 指定失败请求的最大重试次数。
	// 可选。
	MaxRetries *int

	// Model 指定用于响应的模型 ID。
	// 必填。
	Model string

	// MaxTokens 指定响应中生成的最大 token 数。
	// 可选。
	MaxTokens *int

	// Temperature 控制模型输出的随机性。
	// 较高的值（如 0.8）使输出更随机，而较低的值（如 0.2）使输出更集中和确定。
	// 范围：0.0 到 2.0。
	// 可选。
	Temperature *float32

	// TopP 通过核心采样控制多样性。
	// 它指定 token 选择的累积概率阈值。
	// 建议修改此项或 Temperature，但不要同时修改。
	// 范围：0.0 到 1.0。
	// 可选。
	TopP *float32

	// ServiceTier 指定处理请求的延迟层级。
	// 可选。
	ServiceTier *responses.ResponseNewParamsServiceTier

	// Text 指定文本生成输出的配置。
	// 可选。
	Text *responses.ResponseTextConfigParam

	// Reasoning 指定推理模型的配置。
	// 可选。
	Reasoning *responses.ReasoningParam

	// Store 指定是否在服务器上存储响应。
	// 可选。
	Store *bool

	// MaxToolCalls 指定单轮中允许的最大工具调用次数。
	// 可选。
	MaxToolCalls *int

	// ParallelToolCalls 指定是否允许在单轮中进行多次工具调用。
	// 可选。
	ParallelToolCalls *bool

	// Include 指定响应中包含的额外字段列表。
	// 可选。
	Include []responses.ResponseIncludable

	// ServerTools 指定模型可用的服务器端工具。
	// 可选。
	ServerTools []*ServerToolConfig

	// MCPTools 指定模型可用的 Model Context Protocol 工具。
	// 可选。
	MCPTools []*responses.ToolMcpParam

	// CustomHeader 指定 API 请求中包含的自定义 HTTP 标头。
	// CustomHeader 允许传递额外的元数据或身份验证信息。
	// 可选。
	CustomHeader map[string]string

	// ExtraFields 指定将直接添加到 HTTP 请求体的额外字段。
	// 这允许支持尚未显式支持的供应商特定或未来参数。
	// 可选。
	ExtraFields map[string]any
}
```

## 高级用法

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
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/responses"
	"github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	am, err := agenticopenai.New(ctx, &agenticopenai.Config{
		BaseURL: "https://api.agenticopenai.com/v1",
		Model:   os.Getenv("OPENAI_MODEL_ID"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Reasoning: &responses.ReasoningParam{
			Effort:  responses.ReasoningEffortLow,
			Summary: responses.ReasoningSummaryDetailed,
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
	if lastBlock.Type != schema.ContentBlockTypeFunctionToolCall {
		log.Fatalf("last block is not function tool call, type: %s", lastBlock.Type)
	}

	toolCall := lastBlock.FunctionToolCall
	toolResultMsg := schema.FunctionToolResultAgenticMessage(toolCall.CallID, toolCall.Name, "20 degrees")

	secondInput := append(firstInput, concatenated, toolResultMsg)

	gResp, err := am.Generate(ctx, secondInput)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	meta := concatenated.ResponseMeta.OpenAIExtension
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
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

func main() {
	ctx := context.Background()

	am, err := agenticopenai.New(ctx, &agenticopenai.Config{
		BaseURL: "https://api.agenticopenai.com/v1",
		Model:   os.Getenv("OPENAI_MODEL_ID"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Reasoning: &responses.ReasoningParam{
			Effort:  responses.ReasoningEffortLow,
			Summary: responses.ReasoningSummaryDetailed,
		},
		Include: []responses.ResponseIncludable{
			responses.ResponseIncludableWebSearchCallActionSources,
		},
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	serverTools := []*agenticopenai.ServerToolConfig{
		{
			WebSearch: &responses.WebSearchToolParam{
				Type: responses.WebSearchToolTypeWebSearch,
			},
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			ServerTool: &schema.AllowedServerTool{
				Name: string(agenticopenai.ServerToolNameWebSearch),
			},
		},
	}

	opts := []model.Option{
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		agenticopenai.WithServerTools(serverTools),
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

	for _, block := range concatenated.ContentBlocks {
		if block.ServerToolCall != nil {
			serverToolArgs := block.ServerToolCall.Arguments.(*agenticopenai.ServerToolCallArguments)
			args, _ := sonic.MarshalIndent(serverToolArgs, "  ", "  ")
			log.Printf("server_tool_args: %s\n", string(args))
		}

		if block.ServerToolResult != nil {
			result := block.ServerToolResult.Result.(*agenticopenai.ServerToolResult)
			resultJSON, _ := sonic.MarshalIndent(result, "  ", "  ")
			log.Printf("server_tool_result: %s\n", string(resultJSON))
		}
	}

	meta := concatenated.ResponseMeta.OpenAIExtension
	log.Printf("request_id: %s\n", meta.ID)

	respBody, _ := sonic.MarshalIndent(concatenated, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

更多示例请参考 `examples` 目录。
