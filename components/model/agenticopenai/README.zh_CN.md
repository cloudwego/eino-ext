# OpenAI Agentic Model

基于 [Eino](https://github.com/cloudwego/eino) 的 OpenAI 模型实现，实现了 `AgenticModel` 组件接口。本包提供两种模型实现：**Chat**（Chat Completions API）和 **Responses**（Responses API），使其能够无缝集成到 Eino 的 Agent 能力中。

## 功能特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel` 接口
- 易于集成到 Eino 的 agent 系统中
- 可配置的模型参数
- 同时支持 Chat Completions API 和 Responses API
- 支持流式响应 (Streaming)
- 支持工具调用 (Tools)，包括函数工具 (Function Tools)、MCP 工具 (MCP Tools) 和服务器工具 (Server Tools)
- 支持前缀缓存 (Prefix Cache) 和多轮对话自动缓存
- 支持 Azure OpenAI 服务

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@latest
```

## 快速开始

### Responses API

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	am, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  os.Getenv("OPENAI_MODEL_ID"),
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

	respBody, _ := sonic.MarshalIndent(msg, "  ", "  ")
	log.Printf("response: %s\n", string(respBody))
}
```

### Chat Completions API

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	m, err := agenticopenai.NewChatModel(ctx, &agenticopenai.ChatConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  os.Getenv("OPENAI_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("failed to create chat model, err: %v", err)
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what is the weather like in Beijing"),
	}

	msg, err := m.Generate(ctx, input)
	if err != nil {
		log.Fatalf("failed to generate, err: %v", err)
	}

	respBody, _ := sonic.MarshalIndent(msg, "  ", "  ")
	log.Printf("response: %s\n", string(respBody))
}
```

## 配置

### ResponsesConfig

可以使用 `agenticopenai.ResponsesConfig` 结构体配置 `ResponsesModel`：

```go
type ResponsesConfig struct {
    // ByAzure 指定是否使用 Azure OpenAI 服务。
    // 可选。
    ByAzure bool

    // BaseURL 指定 OpenAI 服务端点的基准 URL。
    // 可选。默认值：https://api.openai.com/v1
    BaseURL string

    // APIKey 指定用于认证的 API 密钥。
    // 必填。
    APIKey string

    // Timeout 指定等待 API 响应的最大持续时间。
    // 可选。
    Timeout *time.Duration

    // HTTPClient 指定用于发送请求的 HTTP 客户端。
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
    // 范围：0.0 到 2.0。
    // 可选。
    Temperature *float32

    // TopP 通过核采样控制多样性。
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

    // Store 指定是否将响应存储在服务器上。
    // 可选。
    Store *bool

    // MaxToolCalls 指定单轮中允许的最大工具调用次数。
    // 可选。
    MaxToolCalls *int

    // ParallelToolCalls 指定是否允许单轮中进行多次工具调用。
    // 可选。
    ParallelToolCalls *bool

    // Include 指定响应中需要包含的额外字段列表。
    // 可选。
    Include []responses.ResponseIncludable

    // Truncation 指定如何处理超出模型上下文窗口的内容。
    // 可选。
    Truncation *responses.ResponseNewParamsTruncation

    // EnableAutoCache 控制是否开启多轮对话自动缓存。
    // 启用后，模型通过定位输入中最近的缓存消息（通过 ResponseMeta 中的 Response ID）
    // 自动维护上下文。
    // 可选。
    EnableAutoCache bool

    // PromptCacheRetention 指定提示缓存的保留策略。
    // 可选。
    PromptCacheRetention *responses.ResponseNewParamsPromptCacheRetention

    // CustomHeaders 指定 API 请求中包含的自定义 HTTP 标头。
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
}
```

### ChatConfig

可以使用 `agenticopenai.ChatConfig` 结构体配置 `ChatModel`：

```go
type ChatConfig struct {
    // APIKey 是认证密钥。
    // 必填。
    APIKey string

    // Timeout 指定等待 API 响应的最大持续时间。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。
    Timeout time.Duration

    // HTTPClient 指定用于发送 HTTP 请求的客户端。
    // 如果设置了 HTTPClient，则不会使用 Timeout。
    // 可选。
    HTTPClient *http.Client

    // ByAzure 指定是否使用 Azure OpenAI 服务。
    // 可选。默认值：false
    ByAzure bool

    // AzureModelMapperFunc 用于将模型名称映射为 Azure OpenAI 服务的部署名称。
    // Azure 可选。
    AzureModelMapperFunc func(model string) string

    // APIVersion 指定 Azure OpenAI API 版本。
    // Azure 必填。
    APIVersion string

    // BaseURL 指定 API 端点 URL。
    // 可选。默认值：https://api.openai.com/v1
    BaseURL string

    // Model 指定要使用的模型 ID。
    // 必填。
    Model string

    // MaxCompletionTokens 指定可以生成的 token 数量上限。
    // 可选。
    MaxCompletionTokens *int

    // Temperature 指定采样温度。
    // 范围：0.0 到 2.0。
    // 可选。默认值：1.0
    Temperature *float32

    // TopP 通过核采样控制多样性。
    // 范围：0.0 到 1.0。
    // 可选。默认值：1.0
    TopP *float32

    // Stop 指定 API 停止生成 token 的序列。
    // 可选。
    Stop []string

    // PresencePenalty 通过基于存在性惩罚 token 来防止重复。
    // 范围：-2.0 到 2.0。
    // 可选。默认值：0
    PresencePenalty *float32

    // FrequencyPenalty 通过基于频率惩罚 token 来防止重复。
    // 范围：-2.0 到 2.0。
    // 可选。默认值：0
    FrequencyPenalty *float32

    // LogitBias 修改特定 token 出现在补全中的可能性。
    // 可选。
    LogitBias map[string]int

    // LogProbs 指定是否返回输出 token 的对数概率。
    // 可选。默认值：false
    LogProbs bool

    // TopLogProbs 指定在每个 token 位置返回最可能的 token 数量。
    // 可选。
    TopLogProbs int

    // CustomHeaders 指定请求中包含的自定义 HTTP 标头。
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
}
```

## 扩展字段说明 (Extension Fields)

Eino agentic schema 中的若干字段被定义为 `any` 类型，以便各模型实现可以挂载各自特有的数据。当你消费本包产生的
响应时，必须先将这些 `any` 字段类型断言为本包定义的具体类型，才能读取其中的内容。本节逐一说明每一个此类字段
及其承载的确切类型。

### ResponseMeta

返回的每个 `*schema.AgenticMessage` 都带有 `ResponseMeta *schema.AgenticResponseMeta`。其结构在两种模型实现
之间有所不同。

```go
type AgenticResponseMeta struct {
    // TokenUsage 始终被填充，包含 prompt / completion / total 的 token 统计。
    TokenUsage *TokenUsage

    // OpenAIExtension 由 Responses API 实现 (NewResponsesModel) 填充。
    OpenAIExtension *openai.ResponseMetaExtension

    // Extension 是一个 `any` 字段，由 Chat Completions 实现 (NewChatModel) 填充。
    Extension any

    // GeminiExtension / ClaudeExtension 在本包中不使用。
}
```

#### Responses API —— `ResponseMeta.OpenAIExtension`

`NewResponsesModel` 会填充强类型的 `OpenAIExtension` 字段（无需断言）。它暴露了服务端的响应状态，包括用于
前缀缓存的响应 ID：

```go
type ResponseMetaExtension struct {
    ID                 string             // 服务端分配的响应 ID
    Status             ResponseStatus     // 例如 "completed"、"incomplete"
    Error              *ResponseError     // 响应失败时非 nil
    IncompleteDetails  *IncompleteDetails // 响应不完整时非 nil
    PreviousResponseID string
    Reasoning          *Reasoning // 推理模型的 reasoning effort / summary
    ServiceTier        ServiceTier
    // ... 以及其他服务端字段
}
```

```go
ext := msg.ResponseMeta.OpenAIExtension // 强类型，无需断言
```

> **说明：** `OpenAIExtension.ID` 正是 `EnableAutoCache` 和 `WithHeadPreviousResponseID` 用来延续缓存多轮
> 对话的值。参见 [缓存](#缓存)。

#### Chat Completions API —— `ResponseMeta.Extension` (`any`)

`NewChatModel` 无法使用 `OpenAIExtension`（它是 Responses 专用的），因此改为填充通用的 `Extension any`
字段。**使用前必须将其断言为 `*agenticopenai.ChatResponseMetaExtension`**：

```go
type ChatResponseMetaExtension struct {
    ID           string           // 上游请求 ID
    FinishReason string           // 例如 "stop"、"length"、"tool_calls"
    LogProbs     *schema.LogProbs // 仅当 ChatConfig 中开启 LogProbs 时才会被填充
}
```

```go
// 具体类型始终是 *agenticopenai.ChatResponseMetaExtension。
ext, ok := msg.ResponseMeta.Extension.(*agenticopenai.ChatResponseMetaExtension)
```

### AssistantGenText 扩展

模型生成的文本承载在 `AssistantGenText` 内容块中。普通的 `UserInputText` 块（用户提供的文本）没有扩展，
只有模型生成的 `AssistantGenText` 块才有。本包会填充其强类型的
`OpenAIExtension *openai.AssistantGenTextExtension` 字段（无需断言），它承载拒答信息与引用标注：

```go
type AssistantGenTextExtension struct {
    // Refusal 在模型拒绝回答时被设置；Refusal.Reason 保存拒答文本。
    Refusal *OutputRefusal

    // Annotations 承载附加在生成文本上的引用标注
    //（URL 引用、文件引用、容器文件引用、文件路径）。
    Annotations []*TextAnnotation
}
```

> **说明：** `AssistantGenText` 同样暴露了供其他厂商使用的通用 `Extension any` 字段，但本包只会填充
> `OpenAIExtension`。

### ServerToolCall 与 ServerToolResult

当模型调用内置服务器工具（web search、file search、code interpreter、image generation、shell 或 tool
search）时，产生的内容块会携带 `*schema.ServerToolCall` 与 `*schema.ServerToolResult`。两者都将其负载封装
在一个 `any` 字段中，本包始终用自己的具体类型来填充它。

```go
type ServerToolCall struct {
    Name      string // 工具名，例如 "web_search"（参见 agenticopenai.ServerToolName* 常量）
    CallID    string
    Arguments any    // 具体类型：*agenticopenai.ServerToolCallArguments
}

type ServerToolResult struct {
    Name    string
    CallID  string
    Content any    // 具体类型：*agenticopenai.ServerToolResult
}
```

#### `ServerToolCall.Arguments` (`any`)

断言为 `*agenticopenai.ServerToolCallArguments`。其子字段中恰好有一个非 nil，对应被调用的工具。可通过
`ServerToolCall.Name`（或非 nil 的子字段）来判断应读取哪一个：

```go
type ServerToolCallArguments struct {
    WebSearch       *WebSearchArguments       // ServerToolNameWebSearch
    FileSearch      *FileSearchArguments      // ServerToolNameFileSearch
    CodeInterpreter *CodeInterpreterArguments // ServerToolNameCodeInterpreter
    ImageGeneration *ImageGenerationArguments // ServerToolNameImageGeneration
    Shell           *ShellArguments           // ServerToolNameShell
    ToolSearch      *ToolSearchArguments      // tool search 调用
}
```

#### `ServerToolResult.Content` (`any`)

断言为 `*agenticopenai.ServerToolResult`。与参数一样，恰好有一个子字段被填充，对应产生该结果的工具：

```go
type ServerToolResult struct {
    WebSearch       *WebSearchResult
    FileSearch      *FileSearchResult
    CodeInterpreter *CodeInterpreterResult
    ImageGeneration *ImageGenerationResult
    Shell           *ShellResult
    ToolSearch      *ToolSearchResult // ToolSearchResult.Tools 为 []*schema.ToolInfo
}
```

## 高级用法

### 缓存

使用 `EnableAutoCache` 开启多轮对话自动缓存。若某条缓存消息已经失效，可以调用 `InvalidateMessageCaches` 临时跳过该缓存。

如果需要显式复用前缀缓存，可以通过 `WithHeadPreviousResponseID` 传入响应 ID。

```go
am, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
    APIKey:          os.Getenv("OPENAI_API_KEY"),
    Model:           os.Getenv("OPENAI_MODEL_ID"),
    EnableAutoCache: true,
})
```

下图展示了 SDK 如何在每次调用时解析 `PreviousResponseID`，以及 `InvalidateMessageCaches` 如何重置缓存状态：

```mermaid
flowchart TD
    Input["am.Generate(ctx, input, opts...)"] --> Detail
    Detail["input = [User₁, Asst₁(resp_001), User₂, Asst₂(resp_002), User₃]<br/>opts = WithHeadPreviousResponseID(resp_abc) (optional)"]
    Detail --> B{"Find cached<br/>Response ID<br/>in messages?"}
    B -- "Yes (found resp_002 at Asst₂)" --> C["Send [User₃] only<br/>PreviousResponseID = resp_002"]
    B -- No --> H{"WithHeadPreviousResponseID<br/>provided?"}
    H -- "Yes (resp_abc)" --> I["Send full input<br/>PreviousResponseID = resp_abc"]
    H -- No --> D["Send full input<br/>No cache reuse"]
    C --> E["Response (resp_003) marked as cached<br/>— used as cache anchor in next turn"]
    I --> E
    D --> E

    E -. "Model switched /<br/>history edited" .-> F["InvalidateMessageCaches(input)"]
    F -. "Next call" .-> B

    style Input fill:#e8f4fd,stroke:#4a90d9
    style Detail fill:#e8f4fd,stroke:#4a90d9
    style C fill:#d4edda,stroke:#28a745
    style I fill:#d4edda,stroke:#28a745
    style D fill:#fff3cd,stroke:#ffc107
    style E fill:#d4edda,stroke:#28a745
    style F fill:#f8d7da,stroke:#dc3545
```

**示例 — 对话中途切换模型：**

```go
// 使用模型 A 构建的对话：[User₁, Assistant₁(model_A), User₂]
// 现在切换到模型 B — 模型 A 的缓存前缀已失效。

input := []*schema.AgenticMessage{user1, assistant1, user2}
agenticopenai.InvalidateMessageCaches(input)

// 下次 Generate 将跳过失效的缓存锚点，发送完整输入。
resp, err := modelB.Generate(ctx, input)
```

### 工具调用 (Tool Calling)

`ResponsesModel` 支持工具调用，包括函数工具、MCP 工具和服务器工具。

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
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	am, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  os.Getenv("OPENAI_MODEL_ID"),
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

	am, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  os.Getenv("OPENAI_MODEL_ID"),
		Include: []responses.ResponseIncludable{
			responses.ResponseIncludableWebSearchCallActionSources,
		},
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	serverTools := []*agenticopenai.ResponsesServerToolConfig{
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
		agenticopenai.WithResponsesServerTools(serverTools),
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
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
			result := block.ServerToolResult.Content.(*agenticopenai.ServerToolResult)
			resultJSON, _ := sonic.MarshalIndent(result, "  ", "  ")
			log.Printf("server_tool_result: %s\n", string(resultJSON))
		}
	}

	respBody, _ := sonic.MarshalIndent(concatenated, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

更多示例请参考 `examples` 目录。
