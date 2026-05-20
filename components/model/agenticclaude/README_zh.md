# Claude Agentic 模型

这是一个面向 [Eino](https://github.com/cloudwego/eino) 的 Anthropic Claude 模型实现，满足 `AgenticModel` 组件接口，可无缝接入 Eino 的 Agent 能力，用于更复杂的自然语言生成与工具交互场景。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.AgenticModel`
- 易于与 Eino Agent 系统集成
- 支持灵活的模型参数配置
- 支持 Anthropic Messages API
- 支持流式响应
- 支持工具调用（函数工具、延迟加载工具、客户端工具搜索、Server Tool）
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
    // HTTPClient specifies the client to send HTTP requests.
    // Optional.
    HTTPClient *http.Client

    // ByBedrock specifies the configuration for using AWS Bedrock.
    // Optional.
    ByBedrock *BedrockConfig

    // ByGoogleVertexAI specifies the configuration for using Google Vertex AI.
    // Optional.
    ByGoogleVertexAI *GoogleVertexAIConfig

    // BaseURL is the custom API endpoint URL.
    // Optional.
    BaseURL string

    // APIKey is your Anthropic API key.
    // Required for direct Anthropic API requests.
    APIKey string

    // Model specifies which Claude model to use.
    // Required.
    Model string

    // MaxTokens limits the maximum number of tokens in the response.
    // Required.
    MaxTokens int

    // StopSequences specifies custom stop sequences.
    // Optional.
    StopSequences []string

    // DisableParallelToolUse specifies whether to disable parallel tool use.
    // Optional.
    DisableParallelToolUse *bool

    // Thinking specifies the configuration for Claude thinking mode.
    // Optional.
    Thinking *anthropic.ThinkingConfigParamUnion

    // ServerTools specifies server-side tools available to the model.
    // Optional.
    ServerTools []*ServerToolConfig

    // CustomHeaders specifies custom HTTP headers to include in API requests.
    // Optional.
    CustomHeaders map[string]string

    // ExtraFields specifies additional fields that will be directly added to the HTTP request body.
    // Optional.
    ExtraFields map[string]any
}
```

## Advanced Usage

### Tool Calling

`AgenticModel` 支持工具调用，包括函数工具、延迟加载工具、客户端工具搜索和 Server Tool。

#### Function Tool Example

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

#### Server Tool Example

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
