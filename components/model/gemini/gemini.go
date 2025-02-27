/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gemini

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
)

// NewChatModel creates a new Gemini chat model instance
//
// Parameters:
//   - ctx: The context for the operation
//   - cfg: Configuration for the Gemini model
//
// Returns:
//   - model.ChatModel: A chat model interface implementation
//   - error: Any error that occurred during creation
//
// Example:
//
//	model, err := gemini.NewChatModel(ctx, &gemini.Config{
//	    Client: client,
//	    Model: "gemini-pro",
//	})
func NewChatModel(_ context.Context, cfg *Config) (*ChatModel, error) {
	return &ChatModel{
		cli: cfg.Client,

		model:               cfg.Model,
		maxTokens:           cfg.MaxTokens,
		temperature:         cfg.Temperature,
		topP:                cfg.TopP,
		topK:                cfg.TopK,
		responseSchema:      cfg.ResponseSchema,
		enableCodeExecution: cfg.EnableCodeExecution,
		safetySettings:      cfg.SafetySettings,
	}, nil
}

// Config contains the configuration options for the Gemini model
type Config struct {
	// Client is the Gemini API client instance
	// Required for making API calls to Gemini
	Client *genai.Client

	// Model specifies which Gemini model to use
	// Examples: "gemini-pro", "gemini-pro-vision", "gemini-1.5-flash"
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Optional. Example: maxTokens := 100
	MaxTokens *int

	// Temperature controls randomness in responses
	// Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
	// Optional. Example: temperature := float32(0.7)
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: topP := float32(0.95)
	TopP *float32

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: topK := int32(40)
	TopK *int32

	// ResponseSchema defines the structure for JSON responses
	// Optional. Used when you want structured output in JSON format
	ResponseSchema *openapi3.Schema

	// EnableCodeExecution allows the model to execute code
	// Warning: Be cautious with code execution in production
	// Optional. Default: false
	EnableCodeExecution bool

	// SafetySettings configures content filtering for different harm categories
	// Controls the model's filtering behavior for potentially harmful content
	// Optional.
	SafetySettings []*genai.SafetySetting
}

type ChatModel struct {
	cli *genai.Client

	model               string
	maxTokens           *int
	topP                *float32
	temperature         *float32
	topK                *int32
	responseSchema      *openapi3.Schema
	tools               []*genai.Tool
	origTools           []*schema.ToolInfo
	toolChoice          *schema.ToolChoice
	enableCodeExecution bool
	safetySettings      []*genai.SafetySetting
}

func (c *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {
	session, conf, err := c.initGenerativeModelSession(opts...)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: c.origTools}, opts...).Tools,
		Config:   conf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}
	contents, err := c.convSchemaMessages(input)
	if err != nil {
		return nil, err
	}
	if len(contents) > 1 {
		session.History = append(session.History, contents[:len(contents)-1]...)
	}

	result, err := session.SendMessage(ctx, contents[len(contents)-1].Parts...)
	if err != nil {
		return nil, fmt.Errorf("send message fail: %w", err)
	}

	message, err = c.convResponse(result)
	if err != nil {
		return nil, fmt.Errorf("convert response fail: %w", err)
	}

	callbacks.OnEnd(ctx, c.convCallbackOutput(message, conf))
	return message, nil
}

func (c *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {
	session, conf, err := c.initGenerativeModelSession(opts...)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: c.origTools}, opts...).Tools,
		Config:   conf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}
	for i := 0; i < len(input)-1; i++ {
		content, err := c.convSchemaMessage(input[i])
		if err != nil {
			return nil, fmt.Errorf("convert schema message fail: %w", err)
		}
		session.History = append(session.History, content)
	}

	content, err := c.convSchemaMessage(input[len(input)-1])
	if err != nil {
		return nil, fmt.Errorf("convert schema message fail: %w", err)
	}
	resultIter := session.SendMessageStream(ctx, content.Parts...)

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			panicErr := recover()

			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}
			sw.Close()
		}()
		for {
			resp, err_ := resultIter.Next()
			if errors.Is(err_, iterator.Done) {
				return
			}
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			message, err_ := c.convResponse(resp)
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			closed := sw.Send(c.convCallbackOutput(message, conf), nil)
			if closed {
				return
			}
		}
	}()
	srList := sr.Copy(2)
	callbacks.OnEndWithStreamOutput(ctx, srList[0])
	return schema.StreamReaderWithConvert(srList[1], func(t *model.CallbackOutput) (*schema.Message, error) {
		return t.Message, nil
	}), nil
}

func (c *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	gTools, err := c.toGeminiTools(tools)
	if err != nil {
		return err
	}

	c.tools = gTools
	c.origTools = tools
	tc := schema.ToolChoiceAllowed
	c.toolChoice = &tc
	return nil
}

func (c *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	gTools, err := c.toGeminiTools(tools)
	if err != nil {
		return err
	}

	c.tools = gTools
	c.origTools = tools
	tc := schema.ToolChoiceForced
	c.toolChoice = &tc
	return nil
}

func (c *ChatModel) initGenerativeModelSession(opts ...model.Option) (*genai.ChatSession, *model.Config, error) {
	commonOptions := model.GetCommonOptions(&model.Options{
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		TopP:        c.topP,
		Tools:       nil,
		ToolChoice:  c.toolChoice,
	}, opts...)
	geminiOptions := model.GetImplSpecificOptions(&options{
		TopK:           c.topK,
		ResponseSchema: c.responseSchema,
	}, opts...)
	conf := &model.Config{}

	var m *genai.GenerativeModel
	if commonOptions.Model != nil {
		m = c.cli.GenerativeModel(*commonOptions.Model)
		conf.Model = *commonOptions.Model
	} else {
		m = c.cli.GenerativeModel(c.model)
		conf.Model = c.model
	}
	m.SafetySettings = c.safetySettings

	tools := c.tools
	if commonOptions.Tools != nil {
		var err error
		tools, err = c.toGeminiTools(commonOptions.Tools)
		if err != nil {
			return nil, nil, err
		}
	}

	m.Tools = make([]*genai.Tool, len(tools))
	copy(m.Tools, tools)
	if c.enableCodeExecution {
		m.Tools = append(m.Tools, &genai.Tool{
			CodeExecution: &genai.CodeExecution{},
		})
	}

	if commonOptions.MaxTokens != nil {
		conf.MaxTokens = *commonOptions.MaxTokens
		m.SetMaxOutputTokens(int32(*commonOptions.MaxTokens))
	}
	if commonOptions.TopP != nil {
		conf.TopP = *commonOptions.TopP
		m.SetTopP(*commonOptions.TopP)
	}
	if commonOptions.Temperature != nil {
		conf.Temperature = *commonOptions.Temperature
		m.SetTemperature(*commonOptions.Temperature)
	}
	if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingNone,
			}}
		case schema.ToolChoiceAllowed:
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingAuto,
			}}
		case schema.ToolChoiceForced:
			// The predicted function call will be any one of the provided "functionDeclarations".
			if len(m.Tools) == 0 {
				return nil, nil, fmt.Errorf("tool choice is forced but tool is not provided")
			} else {
				m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingAny,
				}}
			}
		default:
			return nil, nil, fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}
	if geminiOptions.TopK != nil {
		m.SetTopK(*geminiOptions.TopK)
	}
	if geminiOptions.ResponseSchema != nil {
		m.ResponseMIMEType = "application/json"
		var err error
		m.ResponseSchema, err = c.convOpenSchema(geminiOptions.ResponseSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("convert response schema fail: %w", err)
		}
	}
	return m.StartChat(), conf, nil
}

func (c *ChatModel) toGeminiTools(tools []*schema.ToolInfo) ([]*genai.Tool, error) {
	gTools := make([]*genai.Tool, len(tools))
	for i, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Desc,
		}

		openSchema, err := tool.ToOpenAPIV3()
		if err != nil {
			return nil, fmt.Errorf("get open schema fail: %w", err)
		}
		funcDecl.Parameters, err = c.convOpenSchema(openSchema)
		if err != nil {
			return nil, fmt.Errorf("convert open schema fail: %w", err)
		}

		gTools[i] = &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl},
		}
	}

	return gTools, nil
}

func (c *ChatModel) convOpenSchema(schema *openapi3.Schema) (*genai.Schema, error) {
	if schema == nil {
		return nil, nil
	}
	var err error

	result := &genai.Schema{
		Format:      schema.Format,
		Description: schema.Description,
		Nullable:    schema.Nullable,
	}

	switch schema.Type {
	case openapi3.TypeObject:
		result.Type = genai.TypeObject
		if schema.Properties != nil {
			properties := make(map[string]*genai.Schema)
			for name, prop := range schema.Properties {
				if prop == nil || prop.Value == nil {
					continue
				}
				properties[name], err = c.convOpenSchema(prop.Value)
				if err != nil {
					return nil, err
				}
			}
			result.Properties = properties
		}
		if schema.Required != nil {
			result.Required = schema.Required
		}

	case openapi3.TypeArray:
		result.Type = genai.TypeArray
		if schema.Items != nil && schema.Items.Value != nil {
			result.Items, err = c.convOpenSchema(schema.Items.Value)
			if err != nil {
				return nil, err
			}
		}

	case openapi3.TypeString:
		result.Type = genai.TypeString
		if schema.Enum != nil {
			enums := make([]string, 0, len(schema.Enum))
			for _, e := range schema.Enum {
				if str, ok := e.(string); ok {
					enums = append(enums, str)
				} else {
					return nil, fmt.Errorf("enum value must be a string, schema: %+v", schema)
				}
			}
			result.Enum = enums
		}

	case openapi3.TypeNumber:
		result.Type = genai.TypeNumber
	case openapi3.TypeInteger:
		result.Type = genai.TypeInteger
	case openapi3.TypeBoolean:
		result.Type = genai.TypeBoolean
	default:
		result.Type = genai.TypeUnspecified
	}

	return result, nil
}

func (c *ChatModel) convSchemaMessages(messages []*schema.Message) ([]*genai.Content, error) {
	result := make([]*genai.Content, len(messages))
	for i, message := range messages {
		content, err := c.convSchemaMessage(message)
		if err != nil {
			return nil, fmt.Errorf("convert schema message fail: %w", err)
		}
		result[i] = content
	}
	return result, nil
}

func (c *ChatModel) convSchemaMessage(message *schema.Message) (*genai.Content, error) {
	if message == nil {
		return nil, nil
	}

	content := &genai.Content{
		Role: toGeminiRole(message.Role),
	}

	if message.ToolCalls != nil {
		for _, call := range message.ToolCalls {
			args := make(map[string]any)
			err := sonic.UnmarshalString(call.Function.Arguments, &args)
			if err != nil {
				return nil, fmt.Errorf("unmarshal schema tool call arguments to map[string]any fail: %w", err)
			}
			content.Parts = append(content.Parts, &genai.FunctionCall{
				Name: call.Function.Name,
				Args: args,
			})
		}
	}

	if message.Role == schema.Tool {
		response := make(map[string]any)
		err := sonic.UnmarshalString(message.Content, &response)
		if err != nil {
			return nil, fmt.Errorf("unmarshal schema tool call response to map[string]any fail: %w", err)
		}
		content.Parts = append(content.Parts, &genai.FunctionResponse{
			Name:     message.ToolCallID,
			Response: response,
		})
	} else {
		if message.Content != "" {
			content.Parts = append(content.Parts, genai.Text(message.Content))
		}
		content.Parts = append(content.Parts, c.convMedia(message.MultiContent)...)
	}
	return content, nil
}

func (c *ChatModel) convMedia(contents []schema.ChatMessagePart) []genai.Part {
	result := make([]genai.Part, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, genai.Text(content.Text))
		case schema.ChatMessagePartTypeImageURL:
			if content.ImageURL != nil {
				result = append(result, genai.FileData{
					MIMEType: content.ImageURL.MIMEType,
					URI:      content.ImageURL.URI,
				})
			}
		case schema.ChatMessagePartTypeAudioURL:
			if content.AudioURL != nil {
				result = append(result, genai.FileData{
					MIMEType: content.AudioURL.MIMEType,
					URI:      content.AudioURL.URI,
				})
			}
		case schema.ChatMessagePartTypeVideoURL:
			if content.VideoURL != nil {
				result = append(result, genai.FileData{
					MIMEType: content.VideoURL.MIMEType,
					URI:      content.VideoURL.URI,
				})
			}
		case schema.ChatMessagePartTypeFileURL:
			if content.FileURL != nil {
				result = append(result, genai.FileData{
					MIMEType: content.FileURL.MIMEType,
					URI:      content.FileURL.URI,
				})
			}
		}
	}
	return result
}

func (c *ChatModel) convResponse(resp *genai.GenerateContentResponse) (*schema.Message, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini result is empty")
	}

	message, err := c.convCandidate(resp.Candidates[0])
	if err != nil {
		return nil, fmt.Errorf("convert candidate fail: %w", err)
	}

	if resp.UsageMetadata != nil {
		if message.ResponseMeta == nil {
			message.ResponseMeta = &schema.ResponseMeta{}
		}
		message.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}
	return message, nil
}

func (c *ChatModel) convCandidate(candidate *genai.Candidate) (*schema.Message, error) {
	result := &schema.Message{}
	result.ResponseMeta = &schema.ResponseMeta{
		FinishReason: candidate.FinishReason.String(),
	}
	if candidate.Content != nil {
		if candidate.Content.Role == roleModel {
			result.Role = schema.Assistant
		} else {
			result.Role = schema.User
		}

		var texts []string
		for _, part := range candidate.Content.Parts {
			switch tp := part.(type) {
			case genai.Text:
				texts = append(texts, string(tp))
			case genai.FunctionCall:
				fc, err := convFC(&tp)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = append(result.ToolCalls, *fc)
			case *genai.FunctionCall:
				fc, err := convFC(tp)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = append(result.ToolCalls, *fc)
			case *genai.CodeExecutionResult:
				texts = append(texts, tp.Output)
			case *genai.ExecutableCode:
				texts = append(texts, tp.Code)
			default:
				return nil, fmt.Errorf("unsupported part type: %T", part)
			}
		}
		if len(texts) == 1 {
			result.Content = texts[0]
		} else if len(texts) > 1 {
			for _, text := range texts {
				result.MultiContent = append(result.MultiContent, schema.ChatMessagePart{
					Type: schema.ChatMessagePartTypeText,
					Text: text,
				})
			}
		}
	}
	return result, nil
}

func convFC(tp *genai.FunctionCall) (*schema.ToolCall, error) {
	args, err := sonic.MarshalString(tp.Args)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini tool call arguments fail: %w", err)
	}
	return &schema.ToolCall{
		ID: tp.Name,
		Function: schema.FunctionCall{
			Name:      tp.Name,
			Arguments: args,
		},
	}, nil
}

func (c *ChatModel) convCallbackOutput(message *schema.Message, conf *model.Config) *model.CallbackOutput {
	callbackOutput := &model.CallbackOutput{
		Message: message,
		Config:  conf,
	}
	if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
		callbackOutput.TokenUsage = &model.TokenUsage{
			PromptTokens:     message.ResponseMeta.Usage.PromptTokens,
			CompletionTokens: message.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      message.ResponseMeta.Usage.TotalTokens,
		}
	}
	return callbackOutput
}

const (
	roleModel = "model"
	roleUser  = "user"
)

func toGeminiRole(role schema.RoleType) string {
	if role == schema.Assistant {
		return roleModel
	}
	return roleUser
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
