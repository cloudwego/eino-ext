# HTTP Request Tools

A set of HTTP request tools for [Eino](https://github.com/cloudwego/eino) that implement the `InvokableTool` interface. These tools allow you to perform GET and POST requests easily and integrate them with Eino’s chat model interaction system and `ToolsNode` for enhanced functionality.

## Features

- Implements `github.com/cloudwego/eino/components/tool.InvokableTool`
- Supports both GET and POST requests
- Configurable request headers, timeouts, and response content types (JSON or text)
- Simple integration with Eino’s tool system

## Installation

Use `go get` to install the package (adjust the module path to your project structure):

```bash
go get github.com/cloudwego/eino-ext/components/tool/request
```

## Quick Start

Below are two examples demonstrating how to use the GET and POST tools individually.

### GET Request Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	req "github.com/cloudwego/eino-ext/components/tool/request/get"
)

func main() {
	// Configure the GET tool
	config := &req.Config{
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
		ResponseContentType: "json",
		Timeout:             10 * time.Second,
	}

	ctx := context.Background()

	// Create the GET tool
	tool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Prepare the GET request payload
	request := &req.GetRequest{
		URL: "https://jsonplaceholder.typicode.com/posts",
	}

	jsonReq, err := sonic.Marshal(request)
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	// Execute the GET request using the InvokableTool interface
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("GET request failed: %v", err)
	}

	fmt.Println(resp)
}
```

### POST Request Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	post "github.com/cloudwego/eino-ext/components/tool/request/post"
)

func main() {
	config := &post.Config{
		Headers: map[string]string{
			"User-Agent":   "MyCustomAgent",
			"Content-Type": "application/json; charset=UTF-8",
		},
		ResponseContentType: "json",
		Timeout:             10 * time.Second,
	}

	ctx := context.Background()

	tool, err := post.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	request := &post.PostRequest{
		URL:  "https://jsonplaceholder.typicode.com/posts",
		Body: `{"title": "my title","body": "my body","userId": 1}`,
	}

	jsonReq, err := sonic.Marshal(request)

	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Post failed: %v", err)
	}

	fmt.Println(resp)
}

```

## Configuration

Both GET and POST tools share similar configuration parameters defined in their respective `Config` structs. For example:

```go
// Config represents the common configuration for HTTP request tools.
type Config struct {
	Headers             map[string]string `json:"headers"`              // Optional: additional headers to set
	Timeout             time.Duration     `json:"timeout"`              // Optional: request timeout, default is 30 seconds
	ResponseContentType string            `json:"response_content_type"`// Optional: "json" or "text", determines how responses are processed
}
```

For the GET tool, the request schema is defined as:

```go
type GetRequest struct {
	URL string `json:"url" jsonschema_description:"The URL to perform the GET request"`
}
```

And for the POST tool, the request schema is:

```go
type PostRequest struct {
	URL  string `json:"url" jsonschema_description:"The URL to perform the POST request"`
	Body string `json:"body" jsonschema_description:"The request body to be sent in the POST request"`
}
```

Both tools automatically handle response processing based on the `ResponseContentType` setting, unmarshalling JSON responses when appropriate.


## Example with agent 

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/openai"
	req "github.com/cloudwego/eino-ext/components/tool/request/get"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// float32Ptr is a helper to return a pointer for a float32 value.
func float32Ptr(f float32) *float32 {
	return &f
}

func main() {
	// Load OpenAI API key from environment variables.
	openAIAPIKey := os.Getenv("OPENAI_API_KEY")
	if openAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	ctx := context.Background()

	// Setup GET tool configuration.
	config := &req.Config{
		Headers: map[string]string{
			"User-Agent": "MyCustomAgent",
		},
		ResponseContentType: "json",
		Timeout:             10 * time.Second,
	}

	// Instantiate the GET tool.
	getTool, err := req.NewTool(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create GET tool: %v", err)
	}

	// Retrieve the tool info to bind it to the ChatModel.
	toolInfo, err := getTool.Info(ctx)
	if err != nil {
		log.Fatalf("Failed to get tool info: %v", err)
	}

	// Create the ChatModel using OpenAI.
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:       "gpt-4o", // or another supported model
		APIKey:      openAIAPIKey,
		Temperature: float32Ptr(0.7),
	})
	if err != nil {
		log.Fatalf("Failed to create ChatModel: %v", err)
	}

	// Bind the tool to the ChatModel.
	err = chatModel.BindTools([]*schema.ToolInfo{toolInfo})
	if err != nil {
		log.Fatalf("Failed to bind tool to ChatModel: %v", err)
	}

	// Create the Tools node with the GET tool.
	toolsNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{getTool},
	})
	if err != nil {
		log.Fatalf("Failed to create ToolNode: %v", err)
	}

	// Build the chain with the ChatModel and the Tools node.
	chain := compose.NewChain[[]*schema.Message, []*schema.Message]()
	chain.
		AppendChatModel(chatModel, compose.WithNodeName("chat_model")).
		AppendToolsNode(toolsNode, compose.WithNodeName("tools"))

	// Compile the chain to obtain the agent.
	agent, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("Failed to compile chain: %v", err)
	}

	// Define the API specification (api_spec) in OpenAPI (YAML) format.
	apiSpec := `
openapi: "3.0.0"
info:
  title: JSONPlaceholder API
  version: "1.0.0"
servers:
  - url: https://jsonplaceholder.typicode.com
paths:
  /posts:
    get:
      summary: Get posts
      parameters:
        - name: _limit
          in: query
          required: false
          schema:
            type: integer
            example: 2
          description: Limit the number of results
      responses:
        "200":
          description: Successful response
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    userId:
                      type: integer
                    id:
                      type: integer
                    title:
                      type: string
                    body:
                      type: string
  /comments:
    get:
      summary: Get comments
      parameters:
        - name: _limit
          in: query
          required: false
          schema:
            type: integer
            example: 2
          description: Limit the number of results
      responses:
        "200":
          description: Successful response
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    postId:
                      type: integer
                    id:
                      type: integer
                    name:
                      type: string
                    email:
                      type: string
                    body:
                      type: string
`

	// Create a system message that includes the API documentation.
	systemMessage := fmt.Sprintf(`You have access to an API to help answer user queries.
Here is documentation on the API:
%s`, apiSpec)

	// Define initial messages (system and user).
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemMessage,
		},
		{
			Role:    schema.User,
			Content: "Fetch the top two posts. What are their titles?",
		},
	}

	// Invoke the agent with the messages.
	resp, err := agent.Invoke(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to invoke agent: %v", err)
	}

	// Output the response messages.
	for idx, msg := range resp {
		fmt.Printf("Message %d: %s: %s\n", idx, msg.Role, msg.Content)
	}
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [InvokableTool Interface Reference](https://pkg.go.dev/github.com/cloudwego/eino/components/tool)
