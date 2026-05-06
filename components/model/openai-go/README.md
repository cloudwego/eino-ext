# OpenAI (official openai-go SDK)

An OpenAI model implementation for [Eino](https://github.com/cloudwego/eino) using the official OpenAI Go SDK (`github.com/openai/openai-go/v3`). This is intended as a starting point for eventual replacement of the existing openai implementation (see ../openai) which is based on github.com/sashabaranov/go-openai. Newer models from OpenAI increasingly do not fully support the older chat completions API which github.com/sashabaranov/go-openai is based on. Consequently, this component targets the **Responses API only**.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Responses API (non-stream + streaming)
- Tool calling support (function tools)
- Multimodal inputs via `schema.Message.UserInputMultiContent`:
  - text
  - image_url (URL or base64 via `Base64Data` + `MIMEType`)
  - file_url (URL or base64)

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/openai-go@latest
```

## Quick start

```go
package main

import (
  "context"
  "log"
  "os"

  "github.com/cloudwego/eino/schema"
  "github.com/cloudwego/eino-ext/components/model/openai-go"
)

func main() {
  ctx := context.Background()

  cm, err := openaigo.NewChatModel(ctx, &openaigo.Config{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-5.4", // any Responses API capable model
  })
  if err != nil {
    log.Fatal(err)
  }

  out, err := cm.Generate(ctx, []*schema.Message{
    {Role: schema.User, Content: "Hello"},
  })
  if err != nil {
    log.Fatal(err)
  }

  log.Println(out.Content)
}
```

## Tool calling

Bind tools using `WithTools()`:

```go
cm2, err := cm.WithTools([]*schema.ToolInfo{
  {
    Name: "get_weather",
    Desc: "Get weather at the given location",
    ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
      "location": {Type: schema.String, Required: true},
    }),
  },
})
```

Then control selection with Eino common options:

- `model.WithTools(...)`
- `model.WithToolChoice(schema.ToolChoiceAllowed|Forced|Forbidden, allowedToolNames...)`

## Streaming

Use `Stream()` to receive incremental `*schema.Message` deltas.
