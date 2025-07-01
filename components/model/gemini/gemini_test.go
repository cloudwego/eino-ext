package gemini

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	genai "google.golang.org/genai"
)

func TestNewChatModel(t *testing.T) {
	ctx := context.Background()

	t.Run("valid config", func(t *testing.T) {
		mockClient := &genai.Client{}
		model, err := NewChatModel(ctx, &Config{
			Client: mockClient,
			Model:  "gemini-pro",
		})
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "gemini-pro", model.model)
	})

	t.Run("nil config", func(t *testing.T) {
		_, err := NewChatModel(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client must be provided")
	})

	t.Run("nil client", func(t *testing.T) {
		_, err := NewChatModel(ctx, &Config{
			Model: "gemini-pro",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client must be provided")
	})

	t.Run("empty model", func(t *testing.T) {
		mockClient := &genai.Client{}
		_, err := NewChatModel(ctx, &Config{
			Client: mockClient,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model name must be set")
	})

	t.Run("with project and location", func(t *testing.T) {
		mockClient := &genai.Client{}
		model, err := NewChatModel(ctx, &Config{
			Client:    mockClient,
			Model:     "gemini-pro",
			Project:   "my-project",
			Location:  "us-central1",
			Publisher: "google",
		})
		assert.NoError(t, err)
		assert.Equal(t, "projects/my-project/locations/us-central1/publishers/google/models/gemini-pro", model.model)
	})

	t.Run("with default publisher", func(t *testing.T) {
		mockClient := &genai.Client{}
		model, err := NewChatModel(ctx, &Config{
			Client:   mockClient,
			Model:    "gemini-pro",
			Project:  "my-project",
			Location: "us-central1",
		})
		assert.NoError(t, err)
		assert.Equal(t, "projects/my-project/locations/us-central1/publishers/google/models/gemini-pro", model.model)
	})

	t.Run("with fully qualified model name", func(t *testing.T) {
		mockClient := &genai.Client{}
		fullyQualifiedModel := "projects/my-project/locations/us-central1/publishers/anthropic/models/claude-3"
		model, err := NewChatModel(ctx, &Config{
			Client:   mockClient,
			Model:    fullyQualifiedModel,
			Project:  "my-project",
			Location: "us-central1",
		})
		assert.NoError(t, err)
		assert.Equal(t, fullyQualifiedModel, model.model)
	})

	t.Run("with configuration options", func(t *testing.T) {
		mockClient := &genai.Client{}
		maxTokens := 100
		temperature := float32(0.7)
		topP := float32(0.9)
		topK := int32(40)
		topKFloat := float32(topK)
		responseSchema := &openapi3.Schema{Type: &openapi3.Types{"object"}}
		safetySettings := []*genai.SafetySetting{
			{
				Category:  "HARM_CATEGORY_HARASSMENT",
				Threshold: "BLOCK_MEDIUM_AND_ABOVE",
			},
		}

		model, err := NewChatModel(ctx, &Config{
			Client: mockClient,
			Model:  "gemini-pro",
			GenerateContentConfig: &genai.GenerateContentConfig{
				MaxOutputTokens: int32(maxTokens),
				Temperature:     &temperature,
				TopP:            &topP,
				TopK:            &topKFloat,
				SafetySettings:  safetySettings,
			},
			ResponseSchema: responseSchema,
		})
		assert.NoError(t, err)
		assert.Equal(t, "gemini-pro", model.model)
		assert.Equal(t, responseSchema, model.responseSchema)
		assert.NotNil(t, model.generateContentConfig)
		assert.Equal(t, int32(maxTokens), model.generateContentConfig.MaxOutputTokens)
		assert.Equal(t, &temperature, model.generateContentConfig.Temperature)
		assert.Equal(t, &topP, model.generateContentConfig.TopP)
		assert.Equal(t, &topKFloat, model.generateContentConfig.TopK)
		assert.Equal(t, safetySettings, model.generateContentConfig.SafetySettings)
	})
}

func TestBindTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}

	t.Run("bind tools", func(t *testing.T) {
		err := cm.BindTools([]*schema.ToolInfo{
			{Name: "test_tool", Desc: "Test tool"},
		})
		assert.NoError(t, err)
		assert.Len(t, cm.origTools, 1)
		assert.Equal(t, "test_tool", cm.origTools[0].Name)
		assert.NotNil(t, cm.toolChoice)
		assert.Equal(t, schema.ToolChoiceAllowed, *cm.toolChoice)
	})

	t.Run("bind forced tools", func(t *testing.T) {
		err := cm.BindForcedTools([]*schema.ToolInfo{
			{Name: "test_tool", Desc: "Test tool"},
		})
		assert.NoError(t, err)
		assert.NotNil(t, cm.toolChoice)
		assert.Equal(t, schema.ToolChoiceForced, *cm.toolChoice)
	})

	t.Run("empty tools", func(t *testing.T) {
		err := cm.BindTools([]*schema.ToolInfo{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tools to bind")
	})
}

func TestConvSchemaMessageToParts(t *testing.T) {
	cm := &ChatModel{}

	t.Run("text message", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.User,
			Content: "Hello",
		}

		parts, err := cm.convSchemaMessageToParts(message)
		assert.NoError(t, err)
		assert.Len(t, parts, 1)
		assert.Equal(t, "Hello", parts[0].Text)
	})

	t.Run("tool call message", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "test_id",
					Function: schema.FunctionCall{
						Name:      "test_function",
						Arguments: `{"param":"value"}`,
					},
				},
			},
		}

		parts, err := cm.convSchemaMessageToParts(message)
		assert.NoError(t, err)
		assert.Len(t, parts, 1)
		assert.NotNil(t, parts[0].FunctionCall)
		assert.Equal(t, "test_function", parts[0].FunctionCall.Name)
	})

	t.Run("tool response message", func(t *testing.T) {
		message := &schema.Message{
			Role:       schema.Tool,
			ToolCallID: "test_id",
			ToolName:   "test_function",
			Content:    `{"result":"success"}`,
		}

		parts, err := cm.convSchemaMessageToParts(message)
		assert.NoError(t, err)
		assert.Len(t, parts, 1)
		assert.NotNil(t, parts[0].FunctionResponse)
		assert.Equal(t, "test_function", parts[0].FunctionResponse.Name)
	})

	t.Run("invalid tool call arguments", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "test_id",
					Function: schema.FunctionCall{
						Name:      "test_function",
						Arguments: `invalid json`,
					},
				},
			},
		}

		_, err := cm.convSchemaMessageToParts(message)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal schema tool call arguments")
	})

	t.Run("invalid tool response content", func(t *testing.T) {
		message := &schema.Message{
			Role:       schema.Tool,
			ToolCallID: "test_id",
			ToolName:   "test_function",
			Content:    `invalid json`,
		}

		_, err := cm.convSchemaMessageToParts(message)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal schema tool call response")
	})
}

func TestConvCandidate(t *testing.T) {
	cm := &ChatModel{}

	t.Run("text candidate", func(t *testing.T) {
		candidate := &genai.Candidate{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					genai.NewPartFromText("Hello"),
				},
			},
			FinishReason: "STOP",
		}

		message, err := cm.convCandidate(candidate)
		assert.NoError(t, err)
		assert.Equal(t, "Hello", message.Content)
		assert.Equal(t, schema.Assistant, message.Role)
		assert.Equal(t, "STOP", message.ResponseMeta.FinishReason)
	})

	t.Run("function call candidate", func(t *testing.T) {
		candidate := &genai.Candidate{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							Name: "test_function",
							Args: map[string]any{"param": "value"},
						},
					},
				},
			},
			FinishReason: "STOP",
		}

		message, err := cm.convCandidate(candidate)
		assert.NoError(t, err)
		assert.Len(t, message.ToolCalls, 1)
		assert.Equal(t, "test_function", message.ToolCalls[0].Function.Name)
	})

	t.Run("multiple text parts", func(t *testing.T) {
		candidate := &genai.Candidate{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					genai.NewPartFromText("Hello"),
					genai.NewPartFromText(" World"),
				},
			},
			FinishReason: "STOP",
		}

		message, err := cm.convCandidate(candidate)
		assert.NoError(t, err)
		assert.Len(t, message.MultiContent, 2)
		assert.Equal(t, "Hello", message.MultiContent[0].Text)
		assert.Equal(t, " World", message.MultiContent[1].Text)
	})
}

func TestConvFC(t *testing.T) {
	t.Run("function call conversion", func(t *testing.T) {
		fc := &genai.FunctionCall{
			Name: "test_function",
			Args: map[string]any{
				"param1": "value1",
				"param2": 42,
			},
		}

		result, err := convFC(fc)
		assert.NoError(t, err)
		assert.Equal(t, "test_function", result.Function.Name)

		var args map[string]any
		err = sonic.UnmarshalString(result.Function.Arguments, &args)
		assert.NoError(t, err)
		assert.Equal(t, "value1", args["param1"])
		assert.Equal(t, float64(42), args["param2"]) // JSON numbers are float64
	})
}

func TestConvCallbackOutput(t *testing.T) {
	cm := &ChatModel{}

	message := &schema.Message{
		Content: "Hello",
		ResponseMeta: &schema.ResponseMeta{
			Usage: &schema.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	conf := &model.Config{
		Model: "gemini-pro",
	}

	output := cm.convCallbackOutput(message, conf)
	assert.Equal(t, message, output.Message)
	assert.Equal(t, conf, output.Config)
	assert.Equal(t, 10, output.TokenUsage.PromptTokens)
	assert.Equal(t, 5, output.TokenUsage.CompletionTokens)
	assert.Equal(t, 15, output.TokenUsage.TotalTokens)
}

func TestToGeminiRole(t *testing.T) {
	assert.Equal(t, "model", toGeminiRole(schema.Assistant))
	assert.Equal(t, "user", toGeminiRole(schema.User))
	assert.Equal(t, "user", toGeminiRole(schema.System))
	assert.Equal(t, "user", toGeminiRole(schema.Tool))
}

func TestGetType(t *testing.T) {
	cm := &ChatModel{}
	assert.Equal(t, "Gemini", cm.GetType())
}

func TestIsCallbacksEnabled(t *testing.T) {
	cm := &ChatModel{}
	assert.True(t, cm.IsCallbacksEnabled())
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{model: "test model"}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.NoError(t, err)
	assert.Equal(t, "test model", ncm.(*ChatModel).model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).origTools[0].Name)
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

// HIGH PRIORITY TESTS - Core Functionality

func TestMakeResponseFormatterTransparent(t *testing.T) {
	cm := &ChatModel{}

	t.Run("extract response formatter tool call to content", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "formatter_call_1",
					Function: schema.FunctionCall{
						Name:      RESPONSE_FORMATTER_TOOL_NAME,
						Arguments: `{"result":"structured response data","status":"complete"}`,
					},
				},
			},
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Equal(t, `{"result":"structured response data","status":"complete"}`, result.Content)
		assert.Nil(t, result.ToolCalls, "Response formatter tool call should be completely removed")
	})

	t.Run("keep other tool calls while removing response formatter", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "regular_tool_1",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"New York"}`,
					},
				},
				{
					ID: "formatter_call_1",
					Function: schema.FunctionCall{
						Name:      RESPONSE_FORMATTER_TOOL_NAME,
						Arguments: `{"weather":"sunny","temp":25}`,
					},
				},
				{
					ID: "regular_tool_2",
					Function: schema.FunctionCall{
						Name:      "log_request",
						Arguments: `{"timestamp":"2024-01-01"}`,
					},
				},
			},
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Equal(t, `{"weather":"sunny","temp":25}`, result.Content)
		assert.Len(t, result.ToolCalls, 2, "Should keep non-formatter tool calls")
		assert.Equal(t, "get_weather", result.ToolCalls[0].Function.Name)
		assert.Equal(t, "log_request", result.ToolCalls[1].Function.Name)
	})

	t.Run("multiple response formatter tool calls - use last one", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "formatter_call_1",
					Function: schema.FunctionCall{
						Name:      RESPONSE_FORMATTER_TOOL_NAME,
						Arguments: `{"result":"first response"}`,
					},
				},
				{
					ID: "formatter_call_2",
					Function: schema.FunctionCall{
						Name:      RESPONSE_FORMATTER_TOOL_NAME,
						Arguments: `{"result":"final response"}`,
					},
				},
			},
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Equal(t, `{"result":"final response"}`, result.Content)
		assert.Nil(t, result.ToolCalls)
	})

	t.Run("no tool calls - pass through unchanged", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "Regular response without tools",
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Equal(t, "Regular response without tools", result.Content)
		assert.Nil(t, result.ToolCalls)
	})

	t.Run("only regular tool calls - no changes", func(t *testing.T) {
		message := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID: "regular_tool_1",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				},
			},
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Empty(t, result.Content, "Content should remain empty")
		assert.Len(t, result.ToolCalls, 1)
		assert.Equal(t, "get_weather", result.ToolCalls[0].Function.Name)
	})

	t.Run("nil tool calls - pass through unchanged", func(t *testing.T) {
		message := &schema.Message{
			Role:      schema.Assistant,
			Content:   "Response with nil tool calls",
			ToolCalls: nil,
		}

		result := cm.makeResponseFormatterTransparent(message)
		assert.Equal(t, "Response with nil tool calls", result.Content)
		assert.Nil(t, result.ToolCalls)
	})
}

func TestAggregateToolResponses(t *testing.T) {
	cm := &ChatModel{}

	t.Run("simple tool call and response pairing", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather?",
			},
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temperature":75,"condition":"sunny"}`,
			},
			{
				Role:    schema.Assistant,
				Content: "It's 75°F and sunny in NYC.",
			},
		}

		result := cm.aggregateToolResponses(context.Background(), messages)

		// Should have: User message, Tool call message, Tool response message, Assistant message
		assert.Len(t, result, 4)
		assert.Equal(t, schema.User, result[0].Role)
		assert.Equal(t, schema.Assistant, result[1].Role)
		assert.Len(t, result[1].ToolCalls, 1)
		assert.Equal(t, schema.Tool, result[2].Role)
		assert.Equal(t, schema.Assistant, result[3].Role)
	})

	t.Run("multiple tool calls in single message get split and paired", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
					{
						ID: "call_2",
						Function: schema.FunctionCall{
							Name:      "get_time",
							Arguments: `{"timezone":"EST"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temp":75}`,
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_2",
				ToolName:   "get_time",
				Content:    `{"time":"2:30 PM"}`,
			},
		}

		result := cm.aggregateToolResponses(context.Background(), messages)

		// Should have: Call1, Response1, Call2, Response2
		assert.Len(t, result, 4)

		// First call-response pair
		assert.Equal(t, schema.Assistant, result[0].Role)
		assert.Len(t, result[0].ToolCalls, 1)
		assert.Equal(t, "call_1", result[0].ToolCalls[0].ID)
		assert.Equal(t, schema.Tool, result[1].Role)
		assert.Equal(t, "call_1", result[1].ToolCallID)

		// Second call-response pair
		assert.Equal(t, schema.Assistant, result[2].Role)
		assert.Len(t, result[2].ToolCalls, 1)
		assert.Equal(t, "call_2", result[2].ToolCalls[0].ID)
		assert.Equal(t, schema.Tool, result[3].Role)
		assert.Equal(t, "call_2", result[3].ToolCallID)
	})

	t.Run("orphaned tool responses are filtered out", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temp":75}`,
			},
			{
				Role:       schema.Tool,
				ToolCallID: "orphaned_call",
				ToolName:   "unknown_tool",
				Content:    `{"should":"be_filtered"}`,
			},
		}

		result := cm.aggregateToolResponses(context.Background(), messages)

		// Should only have: Call1, Response1 (orphaned response filtered out)
		assert.Len(t, result, 2)
		assert.Equal(t, "call_1", result[0].ToolCalls[0].ID)
		assert.Equal(t, "call_1", result[1].ToolCallID)
	})

	t.Run("complex conversation with mixed message types", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
			{
				Role:    schema.User,
				Content: "Get weather and time",
			},
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "weather_call",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "weather_call",
				ToolName:   "get_weather",
				Content:    `{"temp":75}`,
			},
			{
				Role:    schema.Assistant,
				Content: "Let me also get the time.",
			},
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "time_call",
						Function: schema.FunctionCall{
							Name:      "get_time",
							Arguments: `{"timezone":"EST"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "time_call",
				ToolName:   "get_time",
				Content:    `{"time":"2:30 PM"}`,
			},
		}

		result := cm.aggregateToolResponses(context.Background(), messages)

		// Should have: System, User, WeatherCall, WeatherResponse, Assistant, TimeCall, TimeResponse
		assert.Len(t, result, 7)
		assert.Equal(t, schema.System, result[0].Role)
		assert.Equal(t, schema.User, result[1].Role)
		assert.Equal(t, schema.Assistant, result[2].Role)
		assert.Equal(t, "weather_call", result[2].ToolCalls[0].ID)
		assert.Equal(t, schema.Tool, result[3].Role)
		assert.Equal(t, "weather_call", result[3].ToolCallID)
		assert.Equal(t, schema.Assistant, result[4].Role)
		assert.Equal(t, "Let me also get the time.", result[4].Content)
		assert.Equal(t, schema.Assistant, result[5].Role)
		assert.Equal(t, "time_call", result[5].ToolCalls[0].ID)
		assert.Equal(t, schema.Tool, result[6].Role)
		assert.Equal(t, "time_call", result[6].ToolCallID)
	})

	t.Run("tool calls without matching responses", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
					{
						ID: "call_2",
						Function: schema.FunctionCall{
							Name:      "get_time",
							Arguments: `{"timezone":"EST"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temp":75}`,
			},
			// Missing response for call_2
		}

		result := cm.aggregateToolResponses(context.Background(), messages)

		// Should only have the call-response pair that has both parts
		assert.Len(t, result, 2)
		assert.Equal(t, "call_1", result[0].ToolCalls[0].ID)
		assert.Equal(t, "call_1", result[1].ToolCallID)
	})

	t.Run("no tool calls - regular messages pass through", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
			{
				Role:    schema.Assistant,
				Content: "Hi there!",
			},
		}

		result := cm.aggregateToolResponses(context.Background(), messages)
		assert.Len(t, result, 2)
		assert.Equal(t, "Hello", result[0].Content)
		assert.Equal(t, "Hi there!", result[1].Content)
	})
}

func TestSplitToolCallMessage(t *testing.T) {
	cm := &ChatModel{}

	t.Run("single tool call - return as-is", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "Calling weather API",
			ToolCalls: []schema.ToolCall{
				{
					ID: "call_1",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"NYC"}`,
					},
				},
			},
		}

		result := cm.splitToolCallMessage(message)
		assert.Len(t, result, 1)
		assert.Equal(t, message, result[0])
	})

	t.Run("multiple tool calls - split into individual messages", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "Getting weather and time",
			ToolCalls: []schema.ToolCall{
				{
					ID: "call_1",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"NYC"}`,
					},
				},
				{
					ID: "call_2",
					Function: schema.FunctionCall{
						Name:      "get_time",
						Arguments: `{"timezone":"EST"}`,
					},
				},
			},
		}

		result := cm.splitToolCallMessage(message)
		assert.Len(t, result, 2)

		// First split message
		assert.Equal(t, schema.Assistant, result[0].Role)
		assert.Equal(t, "Getting weather and time", result[0].Content)
		assert.Len(t, result[0].ToolCalls, 1)
		assert.Equal(t, "call_1", result[0].ToolCalls[0].ID)
		assert.Equal(t, "get_weather", result[0].ToolCalls[0].Function.Name)

		// Second split message
		assert.Equal(t, schema.Assistant, result[1].Role)
		assert.Equal(t, "Getting weather and time", result[1].Content)
		assert.Len(t, result[1].ToolCalls, 1)
		assert.Equal(t, "call_2", result[1].ToolCalls[0].ID)
		assert.Equal(t, "get_time", result[1].ToolCalls[0].Function.Name)
	})

	t.Run("deep copy all message properties", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "Test content",
			// Note: ToolCallID and ToolName are intentionally not set here
			// because they should be derived from the individual tool calls when splitting
			// This ensures each split message gets the correct tool call ID and function name
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Multi content text",
				},
			},
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: "stop",
				Usage: &schema.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			ToolCalls: []schema.ToolCall{
				{
					ID: "call_1",
					Function: schema.FunctionCall{
						Name:      "tool_1",
						Arguments: `{"arg1":"value1"}`,
					},
				},
				{
					ID: "call_2",
					Function: schema.FunctionCall{
						Name:      "tool_2",
						Arguments: `{"arg2":"value2"}`,
					},
				},
			},
		}

		result := cm.splitToolCallMessage(message)
		assert.Len(t, result, 2)

		// Verify deep copy of all properties
		for i, splitMsg := range result {
			assert.Equal(t, message.Role, splitMsg.Role)
			assert.Equal(t, message.Content, splitMsg.Content)
			// ToolCallID and ToolName should be from the individual tool call, not the original message
			// This ensures proper tool call/response pairing in the conversation flow
			assert.Equal(t, message.ToolCalls[i].ID, splitMsg.ToolCallID)
			assert.Equal(t, message.ToolCalls[i].Function.Name, splitMsg.ToolName)

			// MultiContent should be deep copied
			assert.Len(t, splitMsg.MultiContent, 1)
			assert.Equal(t, message.MultiContent[0].Text, splitMsg.MultiContent[0].Text)
			// Verify it's a separate slice
			assert.NotSame(t, &message.MultiContent, &splitMsg.MultiContent)

			// ResponseMeta should be deep copied
			assert.NotNil(t, splitMsg.ResponseMeta)
			assert.Equal(t, message.ResponseMeta.FinishReason, splitMsg.ResponseMeta.FinishReason)
			assert.NotNil(t, splitMsg.ResponseMeta.Usage)
			assert.Equal(t, message.ResponseMeta.Usage.PromptTokens, splitMsg.ResponseMeta.Usage.PromptTokens)
			// Verify it's a separate struct
			assert.NotSame(t, message.ResponseMeta, splitMsg.ResponseMeta)
			assert.NotSame(t, message.ResponseMeta.Usage, splitMsg.ResponseMeta.Usage)

			// Each split message should have exactly one tool call
			assert.Len(t, splitMsg.ToolCalls, 1)
			assert.Equal(t, message.ToolCalls[i].ID, splitMsg.ToolCalls[0].ID)
			assert.Equal(t, message.ToolCalls[i].Function.Name, splitMsg.ToolCalls[0].Function.Name)
		}
	})

	t.Run("no tool calls - return as-is", func(t *testing.T) {
		message := &schema.Message{
			Role:    schema.Assistant,
			Content: "No tool calls here",
		}

		result := cm.splitToolCallMessage(message)
		assert.Len(t, result, 1)
		assert.Equal(t, message, result[0])
	})

	t.Run("empty tool calls - return as-is", func(t *testing.T) {
		message := &schema.Message{
			Role:      schema.Assistant,
			Content:   "Empty tool calls",
			ToolCalls: []schema.ToolCall{},
		}

		result := cm.splitToolCallMessage(message)
		assert.Len(t, result, 1)
		assert.Equal(t, message, result[0])
	})
}

func TestProcessMessages(t *testing.T) {
	cm := &ChatModel{}

	t.Run("empty input returns error", func(t *testing.T) {
		_, _, _, err := cm.processMessages(context.Background(), []*schema.Message{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no input messages provided")
	})

	t.Run("simple conversation without tools", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Empty(t, systemInstruction)
		assert.Empty(t, history)
		assert.Equal(t, schema.User, currentMessage.Role)
		assert.Equal(t, "Hello", currentMessage.Content)
	})

	t.Run("conversation with system message", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Equal(t, "You are a helpful assistant.", systemInstruction)
		assert.Empty(t, history)
		assert.Equal(t, schema.User, currentMessage.Role)
		assert.Equal(t, "Hello", currentMessage.Content)
	})

	t.Run("conversation with tool calls and responses", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather?",
			},
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temperature":75}`,
			},
			{
				Role:    schema.Assistant,
				Content: "The weather is 75°F.",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Empty(t, systemInstruction)
		assert.Len(t, history, 3) // User, Assistant with tool call, Tool response
		assert.Equal(t, schema.Assistant, currentMessage.Role)
		assert.Equal(t, "The weather is 75°F.", currentMessage.Content)
	})

	t.Run("multiple system messages get concatenated", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
			{
				Role:    schema.System,
				Content: "Be concise in your responses.",
			},
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Equal(t, "You are a helpful assistant.\nBe concise in your responses.", systemInstruction)
		assert.Empty(t, history)
		assert.Equal(t, schema.User, currentMessage.Role)
		assert.Equal(t, "Hello", currentMessage.Content)
	})

	t.Run("system message as last message creates user message", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
			{
				Role:    schema.System,
				Content: "Additional instruction.",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Equal(t, "Additional instruction.", systemInstruction)
		assert.Len(t, history, 1)
		assert.Equal(t, schema.User, currentMessage.Role)
		assert.Equal(t, "Please respond based on the system instructions provided.", currentMessage.Content)
	})

	t.Run("complex conversation with tool aggregation", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a weather assistant.",
			},
			{
				Role:    schema.User,
				Content: "Get weather for NYC and Boston",
			},
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
					{
						ID: "call_2",
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"Boston"}`,
						},
					},
				},
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
				Content:    `{"temp":75}`,
			},
			{
				Role:       schema.Tool,
				ToolCallID: "call_2",
				ToolName:   "get_weather",
				Content:    `{"temp":65}`,
			},
			{
				Role:    schema.Assistant,
				Content: "NYC is 75°F, Boston is 65°F",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Equal(t, "You are a weather assistant.", systemInstruction)

		// History should have: User, Call1, Response1, Call2, Response2
		assert.Len(t, history, 5)
		assert.Equal(t, "user", history[0].Role)
		assert.Equal(t, "model", history[1].Role) // Assistant with tool call
		assert.Equal(t, "user", history[2].Role)  // Tool response (converted to user)
		assert.Equal(t, "model", history[3].Role) // Assistant with tool call
		assert.Equal(t, "user", history[4].Role)  // Tool response (converted to user)

		assert.Equal(t, schema.Assistant, currentMessage.Role)
		assert.Equal(t, "NYC is 75°F, Boston is 65°F", currentMessage.Content)
	})
}

func TestProcessMessagesWithResponseSchema(t *testing.T) {
	// Test response formatter instruction injection
	cm := &ChatModel{
		responseSchema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
		},
	}

	t.Run("response formatter instruction added to system instruction with history", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather?",
			},
			{
				Role:    schema.Assistant,
				Content: "Let me check that for you.",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Contains(t, systemInstruction, "IMPORTANT: After completing any necessary tool operations, you MUST provide your final response using the 'provide_structured_response' tool")
		assert.Len(t, history, 1)

		// Check that user message content remains unchanged
		assert.Equal(t, "What's the weather?", history[0].Parts[0].Text)

		assert.Equal(t, schema.Assistant, currentMessage.Role)
		assert.Equal(t, "Let me check that for you.", currentMessage.Content)
	})

	t.Run("response formatter instruction added to system instruction when no history", func(t *testing.T) {
		messages := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		systemInstruction, history, currentMessage, err := cm.processMessages(context.Background(), messages)
		assert.NoError(t, err)
		assert.Contains(t, systemInstruction, "IMPORTANT: After completing any necessary tool operations, you MUST provide your final response using the 'provide_structured_response' tool")
		assert.Empty(t, history)

		// Check that current message content remains unchanged
		assert.Equal(t, "Hello", currentMessage.Content)
	})
}
