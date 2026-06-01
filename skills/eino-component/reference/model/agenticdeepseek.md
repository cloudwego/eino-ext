<!--
Copyright 2026 CloudWeGo Authors
-->

# DeepSeek AgenticModel

Use `agenticdeepseek` when you need the `model.AgenticModel` path backed by DeepSeek's OpenAI-compatible API and `*schema.AgenticMessage`.

```go
import "github.com/cloudwego/eino-ext/components/model/agenticdeepseek"
```

## Configuration

```go
am, err := agenticdeepseek.New(ctx, &agenticdeepseek.Config{
    APIKey: "your-key",        // Required
    Model:  "deepseek-reasoner",
})
```

`agenticdeepseek.Config` fields:

| Field | Type | Notes |
|-------|------|-------|
| `APIKey` | `string` | Required |
| `Timeout` | `time.Duration` | Optional; ignored when `HTTPClient` is set |
| `HTTPClient` | `*http.Client` | Optional custom HTTP client |
| `BaseURL` | `string` | Optional; default `https://api.deepseek.com` |
| `Model` | `string` | Required model ID |
| `MaxTokens` | `*int` | Optional maximum output tokens |
| `Temperature` | `*float32` | Optional sampling temperature |
| `TopP` | `*float32` | Optional nucleus sampling |
| `Stop` | `[]string` | Optional stop sequences |
| `PresencePenalty` | `*float32` | Optional presence penalty |
| `ResponseFormatType` | `agenticdeepseek.ResponseFormatType` | Optional response format |
| `FrequencyPenalty` | `*float32` | Optional frequency penalty |
| `LogProbs` | `*bool` | Optional logprob output switch |
| `TopLogProbs` | `*int` | Optional number of top logprobs |

Response format constants:

```go
agenticdeepseek.ResponseFormatTypeText
agenticdeepseek.ResponseFormatTypeJSONObject
```

## Call Options

This implementation is built on `libs/acl/openai.AgenticClient`, so it primarily uses common model options:

```go
resp, err := am.Generate(ctx, messages,
    model.WithTemperature(0.6),
    model.WithMaxTokens(2048),
    model.WithTopP(0.9),
    model.WithTools(toolInfos),
)
```

Common options include `model.WithModel`, `model.WithMaxTokens`, `model.WithTemperature`, `model.WithTopP`, `model.WithStop`, `model.WithTools`, and `model.WithAgenticToolChoice`.

## Tools

For code that works against the `model.AgenticModel` interface, pass tools at call time:

```go
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("solve with tools if needed")},
    model.WithTools(toolInfos),
)
```

## Notes

- `New` returns an error when `config` is nil.
- Response metadata extension is extracted from the underlying OpenAI-compatible response into `AgenticResponseMeta.Extension`.
- Use `ResponseFormatTypeJSONObject` when the DeepSeek API should return JSON object output.
