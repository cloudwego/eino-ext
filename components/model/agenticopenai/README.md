# OpenAI Agentic Model

An OpenAI model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `AgenticModel` component interface. This package provides two model implementations: **Chat** (Chat Completions API) and **Responses** (Responses API), enabling seamless integration with Eino's Agent capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/model.AgenticModel`
- Easy integration with Eino's agent system
- Configurable model parameters
- Support for both Chat Completions API and Responses API
- Support for streaming responses
- Support for tool calling (Function Tools, MCP Tools, Server Tools)
- Support for Prefix Cache and auto-caching for multi-turn conversations
- Support for Azure OpenAI Service

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@latest
```

## Quick Start

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

## Configuration

### ResponsesConfig

The `ResponsesModel` can be configured using the `agenticopenai.ResponsesConfig` struct:

```go
type ResponsesConfig struct {
    // ByAzure specifies whether to use Azure OpenAI service.
    // Optional.
    ByAzure bool

    // BaseURL specifies the base URL for the OpenAI service endpoint.
    // Optional. Default: https://api.openai.com/v1
    BaseURL string

    // APIKey specifies the API key for authentication.
    // Required.
    APIKey string

    // Timeout specifies the maximum duration to wait for API responses.
    // Optional.
    Timeout *time.Duration

    // HTTPClient specifies the HTTP client used to send requests.
    // Optional.
    HTTPClient *http.Client

    // MaxRetries specifies the maximum number of retry attempts for failed requests.
    // Optional.
    MaxRetries *int

    // Model specifies the ID of the model to use for the response.
    // Required.
    Model string

    // MaxTokens specifies the maximum number of tokens to generate in the response.
    // Optional.
    MaxTokens *int

    // Temperature controls the randomness of the model's output.
    // Range: 0.0 to 2.0.
    // Optional.
    Temperature *float32

    // TopP controls diversity via nucleus sampling.
    // Range: 0.0 to 1.0.
    // Optional.
    TopP *float32

    // ServiceTier specifies the latency tier for processing the request.
    // Optional.
    ServiceTier *responses.ResponseNewParamsServiceTier

    // Text specifies configuration for text generation output.
    // Optional.
    Text *responses.ResponseTextConfigParam

    // Reasoning specifies configuration for reasoning models.
    // Optional.
    Reasoning *responses.ReasoningParam

    // Store specifies whether to store the response on the server.
    // Optional.
    Store *bool

    // MaxToolCalls specifies the maximum number of tool calls allowed in a single turn.
    // Optional.
    MaxToolCalls *int

    // ParallelToolCalls specifies whether to allow multiple tool calls in a single turn.
    // Optional.
    ParallelToolCalls *bool

    // Include specifies a list of additional fields to include in the response.
    // Optional.
    Include []responses.ResponseIncludable

    // Truncation specifies how to handle context that exceeds the model's context window.
    // Optional.
    Truncation *responses.ResponseNewParamsTruncation

    // EnableAutoCache controls whether auto-caching for multi-turn conversations is active.
    // When enabled, the model automatically maintains context by locating the most recent
    // cached message in the input (via Response ID in ResponseMeta).
    // Optional.
    EnableAutoCache bool

    // PromptCacheRetention specifies the retention policy for the prompt cache.
    // Optional.
    PromptCacheRetention *responses.ResponseNewParamsPromptCacheRetention

    // CustomHeaders specifies custom HTTP headers to include in API requests.
    // Optional.
    CustomHeaders map[string]string

    // ExtraFields specifies extra fields to include in the request body.
    // These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
    // Optional.
    //
    // Example:
    //
    //	ExtraFields: map[string]any{
    //	    "reasoning_effort": "high",
    //	    "service_tier": "default",
    //	}
    //
    // The resulting request body will be:
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

The `ChatModel` can be configured using the `agenticopenai.ChatConfig` struct:

```go
type ChatConfig struct {
    // APIKey is your authentication key.
    // Required.
    APIKey string

    // Timeout specifies the maximum duration to wait for API responses.
    // If HTTPClient is set, Timeout will not be used.
    // Optional.
    Timeout time.Duration

    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional.
    HTTPClient *http.Client

    // ByAzure indicates whether to use Azure OpenAI Service.
    // Optional. Default: false
    ByAzure bool

    // AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
    // Optional for Azure.
    AzureModelMapperFunc func(model string) string

    // APIVersion specifies the Azure OpenAI API version.
    // Required for Azure.
    APIVersion string

    // BaseURL specifies the API endpoint URL.
    // Optional. Default: https://api.openai.com/v1
    BaseURL string

    // Model specifies the ID of the model to use.
    // Required.
    Model string

    // MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated.
    // Optional.
    MaxCompletionTokens *int

    // Temperature specifies what sampling temperature to use.
    // Range: 0.0 to 2.0.
    // Optional. Default: 1.0
    Temperature *float32

    // TopP controls diversity via nucleus sampling.
    // Range: 0.0 to 1.0.
    // Optional. Default: 1.0
    TopP *float32

    // Stop sequences where the API will stop generating further tokens.
    // Optional.
    Stop []string

    // PresencePenalty prevents repetition by penalizing tokens based on presence.
    // Range: -2.0 to 2.0.
    // Optional. Default: 0
    PresencePenalty *float32

    // FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
    // Range: -2.0 to 2.0.
    // Optional. Default: 0
    FrequencyPenalty *float32

    // LogitBias modifies likelihood of specific tokens appearing in completion.
    // Optional.
    LogitBias map[string]int

    // LogProbs specifies whether to return log probabilities of the output tokens.
    // Optional. Default: false
    LogProbs bool

    // TopLogProbs specifies the number of most likely tokens to return at each token position.
    // Optional.
    TopLogProbs int

    // CustomHeaders specifies custom HTTP headers to include in the request.
    // Optional.
    CustomHeaders map[string]string

    // ExtraFields specifies extra fields to include in the request body.
    // These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
    // Optional.
    //
    // Example:
    //
    //	ExtraFields: map[string]any{
    //	    "reasoning_effort": "high",
    //	    "service_tier": "default",
    //	}
    //
    // The resulting request body will be:
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

## Extension Fields

Several fields in the Eino agentic schema are typed as `any` so that each model implementation can attach
provider-specific data. When you consume responses produced by this package, you must type-assert these `any`
fields to the concrete types defined here before you can read them. This section documents every such field and
the exact type it carries.

### ResponseMeta

Each returned `*schema.AgenticMessage` carries a `ResponseMeta *schema.AgenticResponseMeta`. Its shape differs
between the two model implementations.

```go
type AgenticResponseMeta struct {
    // TokenUsage is always populated with prompt / completion / total token counts.
    TokenUsage *TokenUsage

    // OpenAIExtension is populated by the Responses API implementation (NewResponsesModel).
    OpenAIExtension *openai.ResponseMetaExtension

    // Extension is an `any` field populated by the Chat Completions implementation (NewChatModel).
    Extension any

    // GeminiExtension / ClaudeExtension are unused by this package.
}
```

#### Responses API — `ResponseMeta.OpenAIExtension`

`NewResponsesModel` populates the strongly typed `OpenAIExtension` field (no assertion required). It exposes the
server-side response state, including the response ID used for prefix caching:

```go
type ResponseMetaExtension struct {
    ID                 string             // server-assigned response ID
    Status             ResponseStatus     // e.g. "completed", "incomplete"
    Error              *ResponseError     // non-nil when the response failed
    IncompleteDetails  *IncompleteDetails // non-nil when the response is incomplete
    PreviousResponseID string
    Reasoning          *Reasoning // reasoning effort / summary, for reasoning models
    ServiceTier        ServiceTier
    // ... and other server-side fields
}
```

```go
ext := msg.ResponseMeta.OpenAIExtension // strongly typed, no assertion needed
```

> **Note:** The `OpenAIExtension.ID` is exactly the value consumed by `EnableAutoCache` and
> `WithHeadPreviousResponseID` to continue a cached multi-turn conversation. See [Cache](#cache).

#### Chat Completions API — `ResponseMeta.Extension` (`any`)

`NewChatModel` cannot use `OpenAIExtension` (which is Responses-specific), so it populates the generic
`Extension any` field instead. **You must assert it to `*agenticopenai.ChatResponseMetaExtension`** before use:

```go
type ChatResponseMetaExtension struct {
    ID           string           // the upstream request ID
    FinishReason string           // e.g. "stop", "length", "tool_calls"
    LogProbs     *schema.LogProbs // populated only when LogProbs is enabled in ChatConfig
}
```

```go
// The concrete type is always *agenticopenai.ChatResponseMetaExtension.
ext, ok := msg.ResponseMeta.Extension.(*agenticopenai.ChatResponseMetaExtension)
```

### AssistantGenText Extension

The text generated by the model is carried in `AssistantGenText` content blocks. The plain `UserInputText`
block (user-provided text) has no extension — only the model-generated `AssistantGenText` block does. This
package populates its strongly-typed `OpenAIExtension *openai.AssistantGenTextExtension` field (no assertion
required), which carries refusals and citation annotations:

```go
type AssistantGenTextExtension struct {
    // Refusal is set when the model declined to answer; Refusal.Reason holds the refusal text.
    Refusal *OutputRefusal

    // Annotations carry citations attached to the generated text
    // (URL citations, file citations, container-file citations, file paths).
    Annotations []*TextAnnotation
}
```

> **Note:** `AssistantGenText` also exposes a generic `Extension any` field for other providers, but this
> package only ever populates `OpenAIExtension`.

### ServerToolCall & ServerToolResult

When the model invokes a built-in server tool (web search, file search, code interpreter, image generation,
shell, or tool search), the resulting content blocks carry `*schema.ServerToolCall` and `*schema.ServerToolResult`.
Both wrap their payload in an `any` field, which this package always populates with its own concrete types.

```go
type ServerToolCall struct {
    Name      string // tool name, e.g. "web_search" (see agenticopenai.ServerToolName* constants)
    CallID    string
    Arguments any    // concrete type: *agenticopenai.ServerToolCallArguments
}

type ServerToolResult struct {
    Name    string
    CallID  string
    Content any    // concrete type: *agenticopenai.ServerToolResult
}
```

#### `ServerToolCall.Arguments` (`any`)

Assert to `*agenticopenai.ServerToolCallArguments`. Exactly one of its sub-fields is non-nil, matching the tool
that was invoked. Inspect `ServerToolCall.Name` (or the non-nil field) to decide which one to read:

```go
type ServerToolCallArguments struct {
    WebSearch       *WebSearchArguments       // ServerToolNameWebSearch
    FileSearch      *FileSearchArguments      // ServerToolNameFileSearch
    CodeInterpreter *CodeInterpreterArguments // ServerToolNameCodeInterpreter
    ImageGeneration *ImageGenerationArguments // ServerToolNameImageGeneration
    Shell           *ShellArguments           // ServerToolNameShell
    ToolSearch      *ToolSearchArguments      // tool search calls
}
```

#### `ServerToolResult.Content` (`any`)

Assert to `*agenticopenai.ServerToolResult`. As with arguments, exactly one sub-field is populated, matching the
tool that produced the result:

```go
type ServerToolResult struct {
    WebSearch       *WebSearchResult
    FileSearch      *FileSearchResult
    CodeInterpreter *CodeInterpreterResult
    ImageGeneration *ImageGenerationResult
    Shell           *ShellResult
    ToolSearch      *ToolSearchResult // ToolSearchResult.Tools is []*schema.ToolInfo
}
```

## Advanced Usage

### Cache

Use `EnableAutoCache` to enable auto-caching for multi-turn conversations. If a cached message becomes invalid, call `InvalidateMessageCaches` to temporarily skip it.

For explicit prefix reuse, pass the response ID with `WithHeadPreviousResponseID`.

```go
am, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
    APIKey:          os.Getenv("OPENAI_API_KEY"),
    Model:           os.Getenv("OPENAI_MODEL_ID"),
    EnableAutoCache: true,
})
```

### Tool Calling

The `ResponsesModel` supports tool calling, including Function Tools, MCP Tools, and Server Tools.

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

#### Server Tool Example

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

For more examples, please refer to the `examples` directory.
