# Claude Agentic Model

An Anthropic Claude model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `AgenticModel` component interface. This enables seamless integration with Eino's Agent capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.AgenticModel`
- Easy integration with Eino's agent system
- Configurable model parameters
- Support for Anthropic Messages API
- Support for streaming responses
- Support for tool calling (Function Tools, Deferred Tools, Client Tool Search, Server Tools)
- Support for prompt caching
- Support for AWS Bedrock and Google Vertex AI

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticclaude@latest
```

## Quick Start

Here's a quick example of how to use the `AgenticModel`:

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

## Configuration

The `AgenticModel` can be configured using the `agenticclaude.Config` struct:

```go
type Config struct {
    // HTTPClient specifies the client to send HTTP requests.
    // It is not applied when using Google Vertex AI.
    // Optional.
    HTTPClient *http.Client

    // ByBedrock specifies the configuration for using AWS Bedrock.
    // Optional.
    ByBedrock *BedrockConfig

    // ByGoogleVertexAI specifies the configuration for using Google Vertex AI.
    // Optional.
    ByGoogleVertexAI *GoogleVertexAIConfig

    // BaseURL is the custom API endpoint URL
    // Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
    // Optional.
    BaseURL string

    // APIKey is your Anthropic API key
    // Obtain from: https://console.anthropic.com/account/keys
    // Required for direct Anthropic API requests.
    APIKey string

    // Model specifies which Claude model to use.
    // Required.
    Model string

    // MaxTokens limits the maximum number of tokens in the response
    // Range: 1 to model's context length
    // Required.
    MaxTokens int

    // StopSequences specifies custom stop sequences
    // The model will stop generating when it encounters any of these sequences
    // Optional.
    StopSequences []string

    // DisableParallelToolUse specifies whether to disable parallel tool use.
    // It only takes effect when AgenticToolChoice is set.
    // Optional.
    DisableParallelToolUse *bool

    // Thinking specifies the configuration for Claude thinking mode.
    // Optional.
    Thinking *anthropic.ThinkingConfigParamUnion

    // CustomHeaders specifies custom HTTP headers to include in API requests.
    // CustomHeaders allows passing additional metadata or authentication information.
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

    // CacheControl configures automatic prompt caching behavior.
    // When non-nil, automatically applies a cache_control marker to the last
    // cacheable block in the request.
    // Optional.
    CacheControl *anthropic.CacheControlEphemeralParam
}
```

## Extension Fields

Several fields of the Eino agentic schema are typed `any` so that each model implementation can attach provider-specific data. To consume the data this package produces, type-assert those fields to the concrete types defined here. For the strongly-typed extension fields (`ClaudeExtension`), no assertion is required.

### ResponseMeta

`AgenticResponseMeta.ClaudeExtension` is populated with the strongly-typed `*claude.ResponseMetaExtension`, so no type assertion is needed. The generic `Extension any` field is not used by this package.

```go
// github.com/cloudwego/eino/schema/claude
type ResponseMetaExtension struct {
    ID           string       // the upstream message ID
    StopReason   string       // why generation stopped, e.g. "end_turn", "tool_use"
    StopSequence string       // the custom stop sequence that was hit, if any
    StopDetails  *StopDetails // additional stop information
}
```

```go
ext := msg.ResponseMeta.ClaudeExtension // *claude.ResponseMetaExtension
```

### AssistantGenText Extension

`UserInputText` has no extension. Only `AssistantGenText` carries one: its `ClaudeExtension` field is populated with the strongly-typed `*claude.AssistantGenTextExtension`, so no assertion is needed. The generic `Extension any` field is not used by this package.

```go
// github.com/cloudwego/eino/schema/claude
type AssistantGenTextExtension struct {
    Citations []*TextCitation // citations attached to the generated text, if any
}
```

```go
ext := block.AssistantGenText.ClaudeExtension // *claude.AssistantGenTextExtension
```

### ServerToolCall & ServerToolResult

This package supports Claude server-side (built-in) tools such as web search, web fetch, code execution, and tool search. For these blocks, the generic `any` fields are populated with concrete types defined in this package.

`ServerToolCall.Arguments` is populated with `*agenticclaude.ServerToolCallArguments`. Exactly one field is set, matching the invoked tool.

```go
// package agenticclaude
type ServerToolCallArguments struct {
    WebSearch               *WebSearchArguments               // web_search
    WebFetch                *WebFetchArguments                // web_fetch
    CodeExecution           *CodeExecutionArguments           // code_execution
    BashCodeExecution       *BashCodeExecutionArguments       // bash_code_execution
    TextEditorCodeExecution *TextEditorCodeExecutionArguments // text_editor_code_execution
    ToolSearchToolBm25      *ToolSearchToolBm25Arguments      // tool_search_tool_bm25
    ToolSearchToolRegex     *ToolSearchToolRegexArguments     // tool_search_tool_regex
}
```

```go
args := block.ServerToolCall.Arguments.(*agenticclaude.ServerToolCallArguments)
```

`ServerToolResult.Content` is populated with `*agenticclaude.ServerToolResult`. Exactly one field is set, matching the invoked tool.

```go
// package agenticclaude
type ServerToolResult struct {
    WebSearch               *WebSearchResult               // web_search
    WebFetch                *WebFetchResult                // web_fetch
    CodeExecution           *CodeExecutionResult           // code_execution
    BashCodeExecution       *BashCodeExecutionResult       // bash_code_execution
    TextEditorCodeExecution *TextEditorCodeExecutionResult // text_editor_code_execution
    ToolSearchToolBm25      *ToolSearchToolResult          // tool_search_tool_bm25
    ToolSearchToolRegex     *ToolSearchToolResult          // tool_search_tool_regex
}
```

```go
result := block.ServerToolResult.Content.(*agenticclaude.ServerToolResult)
```

## Advanced Usage

### Cache

Use `CacheControl` to enable auto-caching for multi-turn conversations. When set (non-nil), the API automatically applies a cache_control marker to the last cacheable block in the request.

For fine-grained control, use `SetContentBlockCacheControl` or `SetToolInfoCacheControl` to manually place cache breakpoints on specific blocks or tools.

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

### Tool Calling

The `AgenticModel` supports tool calling, including Function Tools, Deferred Tools, Client Tool Search, and Server Tools.

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

For more examples, please refer to the `examples` directory.
