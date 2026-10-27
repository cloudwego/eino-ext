# GLM Model

A GLM (Zhipu AI / BigModel) chat model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. GLM exposes an OpenAI-compatible chat completion API, so this module is a thin wrapper over the shared OpenAI-compatible transport, following the same pattern as the `qwen` and `deepseek` modules.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Chat completion and streaming responses
- Tool / function calling
- Deep-thinking (reasoning) mode on reasoning-capable models (e.g. `glm-4.6`, `glm-4.5-flash`), surfaced via `schema.Message.ReasoningContent`
- Built-in web search option

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/glm@latest
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/glm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://docs.bigmodel.cn/cn/guide/start/model-overview
	chatModel, err := glm.NewChatModel(ctx, &glm.ChatModelConfig{
		APIKey: os.Getenv("ZHIPU_API_KEY"),
		Model:  "glm-4.5-flash",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("as a machine, how do you answer user's question?"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
```

## Configuration

The model is configured via `glm.ChatModelConfig`. `APIKey` and `Model` are required; `BaseURL` defaults to `https://open.bigmodel.cn/api/paas/v4`. The remaining fields (`Temperature`, `TopP`, `MaxTokens`, `Stop`, `PresencePenalty`, `FrequencyPenalty`, `ResponseFormat`, `User`) map to the standard OpenAI chat completion parameters.

### Reasoning (deep thinking)

Enable thinking either on the config (`EnableThinking`) or per call (`glm.WithThinking(true)`). The reasoning trace is returned via `schema.Message.ReasoningContent` (and streamed as `reasoning_content` deltas). Only effective on reasoning-capable models such as `glm-4.6` and `glm-4.5-flash`.

### Web search

`glm.WithWebSearch(true)` enables GLM's built-in web search tool. Because GLM delivers web search through the request `tools` array, do not combine it with eino function tools (`BindTools`/`WithTools`) in the same call.

## Examples

- [Basic Generation](./examples/generate/)
- [Streaming Response](./examples/stream/)
- [Tool Calling](./examples/tool/)
- [Reasoning / Thinking](./examples/thinking/)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [GLM / BigModel Documentation](https://docs.bigmodel.cn/cn/guide/start/model-overview)
