# OpenAI ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/openai"
```

## Configuration

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey:  "your-key",       // Required
    Model:   "gpt-4o",         // Required
    BaseURL: "",               // Optional, custom endpoint
    Temperature:         ptrFloat32(0.7), // Optional, 0.0-2.0
    MaxCompletionTokens: ptrInt(4096),    // Optional
    ReasoningEffort:     openai.ReasoningEffortLevelHigh, // Optional
})
```

## Azure OpenAI

Use the OpenAI model with Azure-specific config:

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    ByAzure:    true,
    BaseURL:    "https://{RESOURCE_NAME}.openai.azure.com",
    APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
    APIVersion: "2024-06-01",
    Model:      "gpt-4o",
})
```

## Request/Response Modifiers

Many providers offer OpenAI-compatible APIs with extra fields. Use these options to customize the raw request/response without forking the model implementation.

### WithRequestPayloadModifier

Modify the serialized JSON request body before it is sent. Use this to inject provider-specific fields.

```go
resp, err := chatModel.Generate(ctx, messages,
    openai.WithRequestPayloadModifier(
        func(ctx context.Context, msgs []*schema.Message, rawBody []byte) ([]byte, error) {
            // Parse rawBody, add extra fields, return modified JSON
            var body map[string]any
            json.Unmarshal(rawBody, &body)
            body["custom_field"] = "value"
            return json.Marshal(body)
        },
    ),
)
```

### WithResponseMessageModifier

Transform the output message using the raw response body. Use this to extract provider-specific fields from non-streaming responses.

```go
resp, err := chatModel.Generate(ctx, messages,
    openai.WithResponseMessageModifier(
        func(ctx context.Context, msg *schema.Message, rawBody []byte) (*schema.Message, error) {
            // Extract custom data from rawBody into msg.Extra or msg.Content
            return msg, nil
        },
    ),
)
```

### WithResponseChunkMessageModifier

Transform each streaming chunk using the raw chunk body. When `end` is true (stream finished), `msg` and `rawBody` may be nil.

```go
stream, err := chatModel.Stream(ctx, messages,
    openai.WithResponseChunkMessageModifier(
        func(ctx context.Context, msg *schema.Message, rawBody []byte, end bool) (*schema.Message, error) {
            if end {
                return msg, nil
            }
            // Process each chunk
            return msg, nil
        },
    ),
)
```

### WithExtraFields

A simpler alternative when you only need to add top-level fields to the request body:

```go
resp, err := chatModel.Generate(ctx, messages,
    openai.WithExtraFields(map[string]any{
        "custom_param": "value",
    }),
)
```
