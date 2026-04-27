# OpenAI Agentic Model

An OpenAI model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `AgenticModel` component interface. This enables seamless integration with Eino's Agent capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.AgenticModel`
- Easy integration with Eino's agent system
- Configurable model parameters
- Support for responses api
- Support for streaming responses
- Support for tool calling (Function Tools, MCP Tools, Server Tools)
- Support for Azure OpenAI

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@latest
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

## Configuration

The `AgenticModel` can be configured using the `agenticopenai.Config` struct:

```go
type Config struct {
    // ByAzure specifies whether to use Azure OpenAI service.
    // Optional.
    ByAzure bool
    
    // BaseURL specifies the base URL for the OpenAI service endpoint.
    // Optional.
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
    // Higher values (e.g., 0.8) make the output more random, while lower values (e.g., 0.2) make it more focused and deterministic.
    // Range: 0.0 to 2.0.
    // Optional.
    Temperature *float32
    
    // TopP controls diversity via nucleus sampling.
    // It specifies the cumulative probability threshold for token selection.
    // Recommended to use either Temperature or TopP, but not both.
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
    
    // ServerTools specifies server-side tools available to the model.
    // Optional.
    ServerTools []*ServerToolConfig
    
    // MCPTools specifies Model Context Protocol tools available to the model.
    // Optional.
    MCPTools []*responses.ToolMcpParam
    
    // CustomHeader specifies custom HTTP headers to include in API requests.
    // CustomHeader allows passing additional metadata or authentication information.
    // Optional.
    CustomHeader map[string]string
    
    // ExtraFields specifies additional fields that will be directly added to the HTTP request body.
    // This allows for vendor-specific or future parameters not yet explicitly supported.
    // Optional.
    ExtraFields map[string]any
}
```

## Advanced Usage

### Tool Calling

The `AgenticModel` supports tool calling, including Function Tools, MCP Tools, and Server Tools.

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
	"github.com/cloudwego/eino/components/mdoel"
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

For more examples, please refer to the `examples` directory.
