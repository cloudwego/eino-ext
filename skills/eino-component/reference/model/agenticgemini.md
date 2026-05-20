<!--
Copyright 2026 CloudWeGo Authors
-->

# Gemini AgenticModel

Use `agenticgemini` when you need the `model.AgenticModel` path backed by Google Gemini and `*schema.AgenticMessage`.

```go
import "github.com/cloudwego/eino-ext/components/model/agenticgemini"
```

## Configuration

```go
am, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
    Client: genaiClient,       // Required
    Model:  "gemini-2.0-flash",
})
```

`agenticgemini.Config` fields:

| Field | Type | Notes |
|-------|------|-------|
| `Client` | `*genai.Client` | Required Gemini client |
| `Model` | `string` | Model name |
| `MaxTokens` | `*int` | Optional maximum output tokens |
| `Temperature` | `*float32` | Optional, range 0.0 to 1.0 |
| `TopP` | `*float32` | Optional, range 0.0 to 1.0 |
| `TopK` | `*int32` | Optional top-k sampling |
| `ResponseJSONSchema` | `*jsonschema.Schema` | Optional structured JSON schema |
| `EnableCodeExecution` | `*genai.ToolCodeExecution` | Optional CodeExecution server tool |
| `EnableGoogleSearch` | `*genai.GoogleSearch` | Optional GoogleSearch server tool |
| `EnableGoogleSearchRetrieval` | `*genai.GoogleSearchRetrieval` | Optional GoogleSearchRetrieval server tool |
| `EnableComputerUse` | `*genai.ComputerUse` | Optional ComputerUse server tool |
| `EnableURLContext` | `*genai.URLContext` | Optional URLContext server tool |
| `EnableFileSearch` | `*genai.FileSearch` | Optional FileSearch server tool |
| `EnableGoogleMaps` | `*genai.GoogleMaps` | Optional GoogleMaps server tool |
| `SafetySettings` | `[]*genai.SafetySetting` | Optional safety settings |
| `ThinkingConfig` | `*genai.ThinkingConfig` | Optional thinking config |
| `ResponseModalities` | `[]agenticgemini.ResponseModality` | Optional response modalities |
| `MediaResolution` | `genai.MediaResolution` | Optional media resolution |
| `Cache` | `*agenticgemini.CacheConfig` | Optional prefix-cache config |

Response modalities: `ResponseModalityText`, `ResponseModalityImage`, `ResponseModalityAudio`.

## Call Options

Provider-specific options:

```go
resp, err := am.Generate(ctx, messages,
    agenticgemini.WithTopK(40),
    agenticgemini.WithThinkingConfig(thinkingConfig),
    agenticgemini.WithResponseJSONSchema(jsonSchema),
    agenticgemini.WithResponseModalities([]agenticgemini.ResponseModality{
        agenticgemini.ResponseModalityText,
    }),
    agenticgemini.WithCachedContentName("cachedContents/abc"),
)
```

Available provider options: `WithTopK`, `WithResponseJSONSchema`, `WithThinkingConfig`, `WithResponseModalities`, `WithCachedContentName`.

Common model options also apply, including `model.WithModel`, `model.WithMaxTokens`, `model.WithTemperature`, `model.WithTopP`, `model.WithTools`, and `model.WithAgenticToolChoice`.

## Cache

Prefix cache is created with `CreatePrefixCache`:

```go
cached, err := am.CreatePrefixCache(ctx, prefixMessages, model.WithTools(toolInfos))
if err != nil {
    return err
}

resp, err := am.Generate(ctx, messages,
    agenticgemini.WithCachedContentName(cached.Name),
)
```

`CacheConfig` supports `TTL` and `ExpireTime` when creating cached content.

## Tools

For code that works against the `model.AgenticModel` interface, pass tools at call time:

```go
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("call a function if needed")},
    model.WithTools(toolInfos),
)
```

## Notes

- `Generate` and `Stream` reject empty input with `gemini input is empty`.
- Gemini server tools are configured through the `Enable*` fields on `Config`.
- Tool choice should use `model.WithAgenticToolChoice`; avoid classic `model.WithToolChoice` for agentic messages.
