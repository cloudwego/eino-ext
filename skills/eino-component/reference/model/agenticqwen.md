<!--
Copyright 2026 CloudWeGo Authors
-->

# Qwen AgenticModel

Use `agenticqwen` when you need the `model.AgenticModel` path backed by Qwen/DashScope's OpenAI-compatible API and `*schema.AgenticMessage`.

```go
import "github.com/cloudwego/eino-ext/components/model/agenticqwen"
```

## Configuration

```go
am, err := agenticqwen.New(ctx, &agenticqwen.Config{
    APIKey: "your-key",      // Required
    Model:  "qwen-plus",
})
```

`agenticqwen.Config` fields:

| Field | Type | Notes |
|-------|------|-------|
| `APIKey` | `string` | Required |
| `Timeout` | `time.Duration` | Optional; ignored when `HTTPClient` is set |
| `HTTPClient` | `*http.Client` | Optional custom HTTP client |
| `BaseURL` | `string` | Optional; default `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` |
| `Model` | `string` | Required model ID |
| `MaxTokens` | `*int` | Optional maximum output tokens |
| `Temperature` | `*float32` | Optional, range 0.0 to 2.0 |
| `TopP` | `*float32` | Optional, range 0.0 to 1.0 |
| `Stop` | `[]string` | Optional stop sequences |
| `PresencePenalty` | `*float32` | Optional presence penalty |
| `Seed` | `*int` | Optional deterministic sampling seed |
| `FrequencyPenalty` | `*float32` | Optional frequency penalty |
| `LogitBias` | `map[string]int` | Optional token bias map |
| `User` | `*string` | Optional end-user identifier |
| `EnableThinking` | `*bool` | Optional thinking mode switch |
| `PreserveThinking` | `*bool` | Optional multi-turn thinking preservation |
| `Modalities` | `[]agenticqwen.Modality` | Optional output modalities |
| `Audio` | `*agenticqwen.AudioConfig` | Required when `Modalities` includes audio |

Modality and audio constants:

```go
agenticqwen.ModalityText
agenticqwen.ModalityAudio
agenticqwen.AudioFormatWav
agenticqwen.AudioVoiceCherry
agenticqwen.AudioVoiceSerena
agenticqwen.AudioVoiceEthan
agenticqwen.AudioVoiceChelsie
```

## Call Options

Provider-specific options:

```go
resp, err := am.Generate(ctx, messages,
    agenticqwen.WithEnableThinking(true),
    agenticqwen.WithPreserveThinking(true),
    model.WithTools(toolInfos),
)
```

Available provider options: `WithEnableThinking`, `WithPreserveThinking`.

Common model options also apply, including `model.WithModel`, `model.WithMaxTokens`, `model.WithTemperature`, `model.WithTopP`, `model.WithStop`, `model.WithTools`, and `model.WithAgenticToolChoice`.

## Audio Output

For Qwen-Omni models that return audio, set both `Modalities` and `Audio`:

```go
am, err := agenticqwen.New(ctx, &agenticqwen.Config{
    APIKey: "your-key",
    Model:  "qwen-omni-turbo",
    Modalities: []agenticqwen.Modality{
        agenticqwen.ModalityText,
        agenticqwen.ModalityAudio,
    },
    Audio: &agenticqwen.AudioConfig{
        Format: agenticqwen.AudioFormatWav,
        Voice:  agenticqwen.AudioVoiceCherry,
    },
})
```

## Tools

For code that works against the `model.AgenticModel` interface, pass tools at call time:

```go
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("use tools if needed")},
    model.WithTools(toolInfos),
)
```

## Notes

- `New` returns an error when `config` is nil.
- `EnableThinking` and `PreserveThinking` are encoded into provider-specific extra fields before calling the underlying OpenAI-compatible client.
- Response metadata extension is extracted from the underlying OpenAI-compatible response into `AgenticResponseMeta.Extension`.
