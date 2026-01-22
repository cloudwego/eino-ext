# Volcengine Ark Agentic Model

基于 [Eino](https://github.com/cloudwego/eino) 的火山引擎 Ark 模型实现，实现了 `AgenticModel` 组件接口。这使得该模型能够无缝集成到 Eino 的 Agent 能力中，提供增强的自然语言处理和生成功能。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel` 接口
- 易于集成到 Eino 的 agent 系统中
- 可配置的模型参数
- 支持 Responses API
- 支持流式响应 (Streaming)
- 支持工具调用 (Tools)，包括函数工具 (Function Tools)、MCP 工具 (MCP Tools) 和服务器工具 (Server Tools)
- 支持前缀缓存 (Prefix Cache) 和会话缓存 (Session Cache)

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
    // Timeout 指定等待 API 响应的最大持续时间
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。默认值：10 分钟
    Timeout *time.Duration
    
    // HTTPClient 指定用于发送 HTTP 请求的客户端。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。默认值 &http.Client{Timeout: Timeout}
    HTTPClient *http.Client
    
    // RetryTimes 指定失败 API 调用的重试次数
    // 可选。默认值：2
    RetryTimes *int
    
    // BaseURL 指定 Ark 服务的基准 URL
    // 可选。默认值："https://agenticark.cn-beijing.volces.com/api/v3"
    BaseURL string 
    
    // Region 指定 Ark 服务所在的区域
    // 可选。默认值："cn-beijing"
    Region string
    
    // 以下三个字段与认证有关 - 需要 APIKey 或 AccessKey/SecretKey 对之一
    // 有关认证的详细信息，请参阅：https://www.volcengine.com/docs/82379/1298459
    // 如果同时提供，APIKey 优先
    APIKey string 
    
    AccessKey string 
    
    SecretKey string 
    
    // 以下字段对应于 Ark 的 responses API 参数
    // 参考：https://www.volcengine.com/docs/82379/1298454
    
    // Model 指定 ark 平台上的端点 ID
    // 必填
    Model string 
    
    // MaxTokens 指定响应中要生成的最大令牌数。
    // 可选。
	MaxTokens *int
    
    // Temperature 指定要使用的采样温度
    // 通常建议修改此项或 TopP，但不能同时修改
    // 范围：0.0 到 1.0。值越高，输出越随机
    // 可选。默认值：1.0
    Temperature *float64
    
    // TopP 通过核心采样控制多样性
    // 通常建议修改此项或 Temperature，但不能同时修改
    // 范围：0.0 到 1.0。值越低，输出越集中
    // 可选。默认值：0.7
    TopP *float64
    
    // Stop 序列，API 将在这些序列处停止生成更多 token
    // 可选。示例：[]string{"\n", "User:"}
    Stop []string
    
    // FrequencyPenalty 根据频率惩罚 token 以防止重复
    // 范围：-2.0 到 2.0。正值降低重复的可能性
    // 可选。默认值：0
    FrequencyPenalty *float64 
    
    // LogitBias 修改特定 token 在补全中出现的可能性
    // 可选。将 token ID 映射到 -100 到 100 的偏置值
    LogitBias map[string]int32
    
    // PresencePenalty 根据存在与否惩罚 token 以防止重复
    // 范围：-2.0 到 2.0。正值增加新主题的可能性
    // 可选。默认值：0
    PresencePenalty *float64
	
    // LogProbs 指定是否返回输出 token 的对数概率。
    LogProbs *bool
    
    // TopLogProbs 指定每个 token 位置返回的最可能 token 的数量，每个都带有相关的对数概率。
    TopLogProbs *int 
    
    // RepetitionPenalty 基于 token 在目前为止的文本中的现有频率对其进行惩罚。
    // 范围：0.0 到 2.0。1.0 表示无惩罚。
    RepetitionPenalty *float64

    // Thinking 控制模型是否设置为激活深度思考模式。
    // 默认设置为启用。
    Thinking *responses.ResponsesThinking

    // Reasoning 指定模型的推理力度。
    // 可选。

    // EnablePassBackReasoning 控制模型是否在下一次请求中传回推理项。
    // 注意 doubao 1.6 不支持传回推理项。
    // 可选. 默认值：true
    EnablePassBackReasoning *bool

    // MaxToolCalls 限制聊天补全中生成的最大工具调用数。
    // 可选。
    MaxToolCalls *int64

    // ParallelToolCalls 控制模型是否设置为执行并行工具调用。
    // 可选。
    ParallelToolCalls *bool

    // ServerTools 指定模型可用的服务器端工具。
    // 可选。
    ServerTools []*ServerToolConfig

    // MCPTools 指定模型可用的 MCP 工具。
    // 可选。
    MCPTools []*responses.ToolMcp

    // Cache 指定模型的缓存配置。
    // 可选。
    Cache *CacheConfig

    // CustomHeader 请求模型时传递的 http 标头
    CustomHeader map[string]string 
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
