# Volcengine Ark Agentic Model

A Volcengine Ark model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `AgenticModel` component interface. This enables seamless integration with Eino's Agent capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.AgenticModel`
- Easy integration with Eino's agent system
- Configurable model parameters
- Support for responses api
- Support for streaming responses
- Support for tool calling (Function Tools, MCP Tools, Server Tools)
- Support for Prefix Cache and Session Cache

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticark@latest
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
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
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

## Configuration

The `AgenticModel` can be configured using the `agenticark.Config` struct:

```go
type Config struct {
    // Timeout specifies the maximum duration to wait for API responses.
    // If HTTPClient is set, Timeout will not be used.
    // Optional.
    Timeout *time.Duration
    
    // HTTPClient specifies the HTTP client used to send requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: &http.Client{Timeout: Timeout}
    HTTPClient *http.Client
    
    // RetryTimes specifies the number of retry attempts for failed API calls.
    // Optional.
    RetryTimes *int
    
    // BaseURL specifies the base URL for the Ark service endpoint.
    // Optional.
    BaseURL string
    
    // Region specifies the geographic region where the Ark service is located.
    // Optional.
    Region string
    
    // APIKey specifies the API key for authentication.
    // Either APIKey or both AccessKey and SecretKey must be provided.
    // APIKey takes precedence if both authentication methods are provided.
    // For details, see: https://www.volcengine.com/docs/82379/1298459
    APIKey string
    
    // AccessKey specifies the access key for authentication.
    // Must be used together with SecretKey.
    AccessKey string
    
    // SecretKey specifies the secret key for authentication.
    // Must be used together with AccessKey.
    SecretKey string
    
    // Model specifies the identifier of the model endpoint on the Ark platform.
    // For details, see: https://www.volcengine.com/docs/82379/1298454
    // Required.
    Model string
    
    // MaxTokens specifies the maximum number of tokens to generate in the response.
    // Optional.
    MaxTokens *int
    
    // Temperature controls the randomness of the model's output.
    // Lower values (e.g., 0.2) make the output more focused and deterministic.
    // Higher values (e.g., 1.0) make the output more creative and varied.
    // Range: 0.0 to 2.0.
    // Optional.
    Temperature *float32
    
    // TopP controls diversity via nucleus sampling, an alternative to Temperature.
    // TopP specifies the cumulative probability threshold for token selection.
    // For example, 0.1 means only tokens comprising the top 10% probability mass are considered.
    // We recommend using either Temperature or TopP, but not both.
    // Range: 0.0 to 1.0.
    // Optional.
    TopP *float32
    
    // ServiceTier specifies the service tier to use for the request.
    // Optional.
    ServiceTier *responses.ResponsesServiceTier_Enum
    
    // Text specifies text generation configuration options.
    // Optional.
    Text *responses.ResponsesText
    
    // Thinking controls whether the model uses deep thinking mode.
    // Optional.
    Thinking *responses.ResponsesThinking
    
    // Reasoning specifies the effort level for the model's reasoning process.
    // Optional.
    Reasoning *responses.ResponsesReasoning
    
    // EnablePassBackReasoning controls whether the model passes back reasoning items in the next request.
    // Note that doubao 1.6 does not support pass back reasoning.
    // Optional. Default: true
    EnablePassBackReasoning *bool
    
    // MaxToolCalls specifies the maximum number of tool calls the model can make in a single response.
    // Optional.
    MaxToolCalls *int64
    
    // ParallelToolCalls determines whether the model can invoke multiple tools simultaneously.
    // Optional.
    ParallelToolCalls *bool
    
    // ServerTools specifies server-side tools available to the model.
    // Optional.
    ServerTools []*ServerToolConfig
    
    // MCPTools specifies Model Context Protocol tools available to the model.
    // Optional.
    MCPTools []*responses.ToolMcp
    
    // Cache specifies response caching configuration for the session.
    // Optional.
    Cache *CacheConfig
    
    // CustomHeader specifies custom HTTP headers to include in API requests.
    // CustomHeader allows passing additional metadata or authentication information.
    // Optional.
    CustomHeader map[string]string
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
	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
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

For more examples, please refer to the `examples` directory.
