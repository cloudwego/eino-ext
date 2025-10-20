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
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

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
		thinkingConfig:      cfg.ThinkingConfig,
		responseModalities:  cfg.ResponseModalities,
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

	ThinkingConfig *genai.ThinkingConfig

	// ResponseModalities specifies the modalities the model can return.
	// Optional.
	ResponseModalities []GeminiResponseModality
}

type ChatModel struct {
	cli *genai.Client

	model               string
	maxTokens           *int
	topP                *float32
	temperature         *float32
	topK                *int32
	responseSchema      *openapi3.Schema
	tools               []*genai.FunctionDeclaration
	origTools           []*schema.ToolInfo
	toolChoice          *schema.ToolChoice
	enableCodeExecution bool
	safetySettings      []*genai.SafetySetting
	thinkingConfig      *genai.ThinkingConfig
	responseModalities  []GeminiResponseModality
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	modelName, nInput, genaiConf, cbConf, err := cm.genInputAndConf(input, opts...)

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: cm.origTools}, opts...).Tools,
		Config:   cbConf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}
	contents, err := cm.convSchemaMessages(nInput)
	if err != nil {
		return nil, err
	}

	result, err := cm.cli.Models.GenerateContent(ctx, modelName, contents, genaiConf)
	if err != nil {
		return nil, fmt.Errorf("send message fail: %w", err)
	}

	message, err = cm.convResponse(result)
	if err != nil {
		return nil, fmt.Errorf("convert response fail: %w", err)
	}

	callbacks.OnEnd(ctx, cm.convCallbackOutput(message, cbConf))
	return message, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	modelName, nInput, genaiConf, cbConf, err := cm.genInputAndConf(input, opts...)
	if err != nil {
		return nil, err
	}
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    model.GetCommonOptions(&model.Options{Tools: cm.origTools}, opts...).Tools,
		Config:   cbConf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if len(input) == 0 {
		return nil, fmt.Errorf("gemini input is empty")
	}

	contents, err := cm.convSchemaMessages(nInput)
	if err != nil {
		return nil, fmt.Errorf("convert schema message fail: %w", err)
	}
	resultIter := cm.cli.Models.GenerateContentStream(ctx, modelName, contents, genaiConf)

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			pe := recover()

			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}
			sw.Close()
		}()
		for resp, err_ := range resultIter {
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			message, err_ := cm.convResponse(resp)
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			closed := sw.Send(cm.convCallbackOutput(message, cbConf), nil)
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

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to gemini tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.toolChoice = &tc
	ncm.tools = gTools
	ncm.origTools = tools
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return err
	}

	cm.tools = gTools
	cm.origTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	gTools, err := cm.toGeminiTools(tools)
	if err != nil {
		return err
	}

	cm.tools = gTools
	cm.origTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) genInputAndConf(input []*schema.Message, opts ...model.Option) (string, []*schema.Message, *genai.GenerateContentConfig, *model.Config, error) {
	commonOptions := model.GetCommonOptions(&model.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		TopP:        cm.topP,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)
	geminiOptions := model.GetImplSpecificOptions(&options{
		TopK:               cm.topK,
		ResponseSchema:     cm.responseSchema,
		ResponseModalities: cm.responseModalities,
	}, opts...)
	conf := &model.Config{}

	m := &genai.GenerateContentConfig{}
	if commonOptions.Model != nil {
		conf.Model = *commonOptions.Model
	} else {
		conf.Model = cm.model
	}
	m.SafetySettings = cm.safetySettings

	tools := cm.tools
	if commonOptions.Tools != nil {
		var err error
		tools, err = cm.toGeminiTools(commonOptions.Tools)
		if err != nil {
			return "", nil, nil, nil, err
		}
	}

	if len(tools) > 0 {
		t := &genai.Tool{
			FunctionDeclarations: make([]*genai.FunctionDeclaration, len(tools)),
		}
		copy(t.FunctionDeclarations, tools)
		m.Tools = append(m.Tools, t)
	}
	if cm.enableCodeExecution {
		m.Tools = append(m.Tools, &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		})
	}

	if commonOptions.MaxTokens != nil {
		conf.MaxTokens = *commonOptions.MaxTokens
		m.MaxOutputTokens = int32(*commonOptions.MaxTokens)
	}
	if commonOptions.TopP != nil {
		conf.TopP = *commonOptions.TopP
		m.TopP = commonOptions.TopP
	}
	if commonOptions.Temperature != nil {
		conf.Temperature = *commonOptions.Temperature
		m.Temperature = commonOptions.Temperature
	}
	if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeNone,
			}}
		case schema.ToolChoiceAllowed:
			m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAuto,
			}}
		case schema.ToolChoiceForced:
			// The predicted function call will be any one of the provided "functionDeclarations".
			if len(m.Tools) == 0 {
				return "", nil, nil, nil, fmt.Errorf("tool choice is forced but tool is not provided")
			} else {
				m.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeAny,
				}}
			}
		default:
			return "", nil, nil, nil, fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}
	if geminiOptions.TopK != nil {
		topK := float32(*geminiOptions.TopK)
		m.TopK = &topK
	}
	if geminiOptions.ResponseSchema != nil {
		m.ResponseMIMEType = "application/json"
		var err error
		if geminiOptions.ResponseJSONSchema != nil {
			m.ResponseJsonSchema = geminiOptions.ResponseJSONSchema
		} else if geminiOptions.ResponseSchema != nil {
			m.ResponseSchema, err = cm.convOpenAPIV3SchemaGeminiSchema(geminiOptions.ResponseSchema)
		}
		if err != nil {
			return "", nil, nil, nil, fmt.Errorf("convert response schema fail: %w", err)
		}
	}

	if len(geminiOptions.ResponseModalities) > 0 {
		m.ResponseModalities = make([]string, len(geminiOptions.ResponseModalities))
		for i, v := range geminiOptions.ResponseModalities {
			m.ResponseModalities[i] = string(v)
		}
	}

	nInput := make([]*schema.Message, len(input))
	copy(nInput, input)
	if len(input) > 1 && input[0].Role == schema.System {
		var err error
		m.SystemInstruction, err = cm.convSchemaMessage(input[0])
		if err != nil {
			return "", nil, nil, nil, fmt.Errorf("failed to convert system instruction: %w", err)
		}
		nInput = input[1:]
	}

	m.ThinkingConfig = cm.thinkingConfig
	if geminiOptions.ThinkingConfig != nil {
		m.ThinkingConfig = geminiOptions.ThinkingConfig
	}
	return conf.Model, nInput, m, conf, nil
}

func (cm *ChatModel) toGeminiTools(tools []*schema.ToolInfo) ([]*genai.FunctionDeclaration, error) {
	gTools := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Desc,
		}

		js, err := tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("get open schema fail: %w", err)
		}
		funcDecl.Parameters, err = cm.convJSONSchemaToGeminiSchema(js)
		if err != nil {
			return nil, fmt.Errorf("convert open schema fail: %w", err)
		}

		gTools[i] = funcDecl
	}

	return gTools, nil
}

func (cm *ChatModel) convJSONSchemaToGeminiSchema(js *jsonschema.Schema) (*genai.Schema, error) {
	if js == nil {
		return nil, nil
	}

	oType := genai.Type("")
	switch js.Type {
	case string(schema.String):
		oType = genai.TypeString
	case string(schema.Number):
		oType = genai.TypeNumber
	case string(schema.Integer):
		oType = genai.TypeInteger
	case string(schema.Boolean):
		oType = genai.TypeBoolean
	case string(schema.Object):
		oType = genai.TypeObject
	case string(schema.Array):
		oType = genai.TypeArray
	case string(schema.Null):
		oType = genai.TypeNULL
	}

	sc := &genai.Schema{
		Type:        oType,
		Title:       js.Title,
		Format:      js.Format,
		Description: js.Description,
		Default:     js.Default,
		Pattern:     js.Pattern,
	}

	if js.MaxLength != nil {
		sc.MaxLength = genai.Ptr(int64(*js.MaxLength))
	}

	if js.MinLength != nil {
		sc.MinLength = genai.Ptr(int64(*js.MinLength))
	}

	if js.MaxItems != nil {
		sc.MaxItems = genai.Ptr(int64(*js.MaxItems))
	}

	if js.MinItems != nil {
		sc.MinItems = genai.Ptr(int64(*js.MinItems))
	}

	if js.MinProperties != nil {
		sc.MinProperties = genai.Ptr(int64(*js.MinProperties))
	}

	if js.MaxProperties != nil {
		sc.MaxProperties = genai.Ptr(int64(*js.MaxProperties))
	}

	if len(js.Examples) > 0 {
		sc.Example = js.Examples[0]
	}

	if js.AnyOf != nil {
		sc.AnyOf = make([]*genai.Schema, len(js.AnyOf))
		for i, anyOf := range js.AnyOf {
			v, err := cm.convJSONSchemaToGeminiSchema(anyOf)
			if err != nil {
				return nil, err
			}
			sc.AnyOf[i] = v
		}
	}

	if js.Enum != nil {
		sc.Enum = make([]string, len(js.Enum))
		for i, enum := range js.Enum {
			enum_, ok := enum.(string)
			if !ok {
				return nil, fmt.Errorf("enum must be string")
			}
			sc.Enum[i] = enum_
		}
	}

	if js.Items != nil {
		v, err := cm.convJSONSchemaToGeminiSchema(js.Items)
		if err != nil {
			return nil, err
		}
		sc.Items = v
	}

	if js.Properties != nil {
		sc.Properties = make(map[string]*genai.Schema, js.Properties.Len())
		for pair := js.Properties.Oldest(); pair != nil; pair = pair.Next() {
			v, err := cm.convJSONSchemaToGeminiSchema(pair.Value)
			if err != nil {
				return nil, err
			}
			sc.Properties[pair.Key] = v
		}
	}

	if js.Required != nil {
		sc.Required = make([]string, len(js.Required))
		for i, required := range js.Required {
			sc.Required[i] = required
		}
	}

	return sc, nil
}

func (cm *ChatModel) convOpenAPIV3SchemaGeminiSchema(schema *openapi3.Schema) (*genai.Schema, error) {
	if schema == nil {
		return nil, nil
	}
	var err error

	result := &genai.Schema{
		Format:      schema.Format,
		Description: schema.Description,
	}
	if schema.Nullable {
		result.Nullable = &schema.Nullable
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
				properties[name], err = cm.convOpenAPIV3SchemaGeminiSchema(prop.Value)
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
			result.Items, err = cm.convOpenAPIV3SchemaGeminiSchema(schema.Items.Value)
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

func (cm *ChatModel) convSchemaMessages(messages []*schema.Message) ([]*genai.Content, error) {
	result := make([]*genai.Content, len(messages))
	for i, message := range messages {
		content, err := cm.convSchemaMessage(message)
		if err != nil {
			return nil, fmt.Errorf("convert schema message fail: %w", err)
		}
		result[i] = content
	}
	return result, nil
}

func (cm *ChatModel) convSchemaMessage(message *schema.Message) (*genai.Content, error) {
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
			content.Parts = append(content.Parts, genai.NewPartFromFunctionCall(call.Function.Name, args))
		}
	}

	if message.Role == schema.Tool {
		response := make(map[string]any)
		err := sonic.UnmarshalString(message.Content, &response)
		if err != nil {
			response["output"] = message.Content
		}
		content.Parts = append(content.Parts, genai.NewPartFromFunctionResponse(message.ToolCallID, response))
	} else {
		if len(message.UserInputMultiContent) > 0 && len(message.AssistantGenMultiContent) > 0 {
			return nil, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
		}
		if len(message.UserInputMultiContent) > 0 {
			if message.Role != schema.User {
				return nil, fmt.Errorf("user input multi content only support user role, got %s", message.Role)
			}
			parts, err := cm.convInputMedia(message.UserInputMultiContent)
			if err != nil {
				return nil, err
			}
			content.Parts = append(content.Parts, parts...)
			return content, nil
		} else if len(message.AssistantGenMultiContent) > 0 {
			if message.Role != schema.Assistant {
				return nil, fmt.Errorf("assistant gen multi content only support assistant role, got %s", message.Role)
			}
			parts, err := cm.convOutputMedia(message.AssistantGenMultiContent)
			if err != nil {
				return nil, err
			}
			content.Parts = append(content.Parts, parts...)
			return content, nil
		}
		if message.Content != "" {
			content.Parts = append(content.Parts, genai.NewPartFromText(message.Content))
		}
		if message.MultiContent != nil {
			log.Printf("MultiContent field is deprecated, please use UserInputMultiContent or AssistantGenMultiContent instead")
			parts, err := cm.convMedia(message.MultiContent)
			if err != nil {
				return nil, err
			}
			content.Parts = parts
		}
	}
	return content, nil
}

func (cm *ChatModel) convInputMedia(contents []schema.MessageInputPart) ([]*genai.Part, error) {
	result := make([]*genai.Part, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, genai.NewPartFromText(content.Text))
		case schema.ChatMessagePartTypeImageURL:
			if content.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
			}
			if content.Image.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Image.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Image.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for image parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Image.MIMEType))
			} else if content.Image.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for image parts, please use Base64Data instead")
			}
		case schema.ChatMessagePartTypeAudioURL:
			if content.Audio == nil {
				return nil, fmt.Errorf("audio field must not be nil when Type is ChatMessagePartTypeAudioURL in user message")
			}
			if content.Audio.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Audio.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Audio.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for audio parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Audio.MIMEType))
			} else if content.Audio.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for audio parts, please use Base64Data instead")
			}
		case schema.ChatMessagePartTypeVideoURL:
			if content.Video == nil {
				return nil, fmt.Errorf("video field must not be nil when Type is ChatMessagePartTypeVideoURL in user message")
			}
			if content.Video.Extra != nil {
				videoMetaData := GetInputVideoMetaData(content.Video)
				if videoMetaData != nil {
					result = append(result, &genai.Part{VideoMetadata: videoMetaData})
				}
			}
			if content.Video.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Video.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Video.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for video parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Video.MIMEType))
			} else if content.Video.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for video parts, please use Base64Data instead")
			}
		case schema.ChatMessagePartTypeFileURL:
			if content.File == nil {
				return nil, fmt.Errorf("file field must not be nil when Type is ChatMessagePartTypeFileURL in user message")
			}
			if content.File.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.File.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.File.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for file parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.File.MIMEType))
			} else if content.File.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for file parts, please use Base64Data instead")
			}
		}
	}
	return result, nil
}

func (cm *ChatModel) convOutputMedia(contents []schema.MessageOutputPart) ([]*genai.Part, error) {
	result := make([]*genai.Part, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, genai.NewPartFromText(content.Text))
		case schema.ChatMessagePartTypeImageURL:
			if content.Image == nil {
				return nil, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in assistant message")
			}
			if content.Image.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Image.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Image.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for image parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Image.MIMEType))
			} else if content.Image.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for image parts, please use Base64Data instead")
			}
		case schema.ChatMessagePartTypeAudioURL:
			if content.Audio == nil {
				return nil, fmt.Errorf("audio field must not be nil when Type is ChatMessagePartTypeAudioURL in assistant message")
			}
			if content.Audio.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Audio.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Audio.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for audio parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Audio.MIMEType))
			} else if content.Audio.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for audio parts, please use Base64Data instead")
			}
		case schema.ChatMessagePartTypeVideoURL:
			if content.Video == nil {
				return nil, fmt.Errorf("video field must not be nil when Type is ChatMessagePartTypeVideoURL in assistant message")
			}
			if content.Video.Base64Data != nil {
				data, err := decodeBase64DataURL(*content.Video.Base64Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
				}
				if content.Video.MIMEType == "" {
					return nil, fmt.Errorf("MIMEType is required for video parts with Base64Data")
				}
				result = append(result, genai.NewPartFromBytes(data, content.Video.MIMEType))
			} else if content.Video.URL != nil {
				return nil, fmt.Errorf("gemini: URL is not supported for video parts, please use Base64Data instead")
			}
		}
	}
	return result, nil
}

func (cm *ChatModel) convMedia(contents []schema.ChatMessagePart) ([]*genai.Part, error) {
	result := make([]*genai.Part, 0, len(contents))
	for _, content := range contents {
		switch content.Type {
		case schema.ChatMessagePartTypeText:
			result = append(result, genai.NewPartFromText(content.Text))
		case schema.ChatMessagePartTypeImageURL:
			if content.ImageURL != nil {
				if content.ImageURL.URI != "" {
					result = append(result, genai.NewPartFromURI(content.ImageURL.URI, content.ImageURL.MIMEType))
				} else {
					data, err := decodeBase64DataURL(content.ImageURL.URL)
					if err != nil {
						return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
					}
					result = append(result, genai.NewPartFromBytes(data, content.ImageURL.MIMEType))
				}
			}
		case schema.ChatMessagePartTypeAudioURL:
			if content.AudioURL != nil {
				if content.AudioURL.URI != "" {
					result = append(result, genai.NewPartFromURI(content.AudioURL.URI, content.AudioURL.MIMEType))
				} else {
					data, err := decodeBase64DataURL(content.AudioURL.URL)
					if err != nil {
						return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
					}
					result = append(result, genai.NewPartFromBytes(data, content.AudioURL.MIMEType))
				}
			}
		case schema.ChatMessagePartTypeVideoURL:
			if content.VideoURL != nil {
				if content.VideoURL.Extra != nil {
					videoMetaData := GetVideoMetaData(content.VideoURL)
					if videoMetaData != nil {
						result = append(result, &genai.Part{
							VideoMetadata: videoMetaData,
						})
					}
				}
				if content.VideoURL.URI != "" {
					result = append(result, genai.NewPartFromURI(content.VideoURL.URI, content.VideoURL.MIMEType))
				} else {
					data, err := decodeBase64DataURL(content.VideoURL.URL)
					if err != nil {
						return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
					}
					result = append(result, genai.NewPartFromBytes(data, content.VideoURL.MIMEType))
				}
			}
		case schema.ChatMessagePartTypeFileURL:
			if content.FileURL != nil {
				if content.FileURL.URI != "" {
					result = append(result, genai.NewPartFromURI(content.FileURL.URI, content.FileURL.MIMEType))
				} else {
					data, err := decodeBase64DataURL(content.FileURL.URL)
					if err != nil {
						return nil, fmt.Errorf("failed to decode base64 data URL: %w", err)
					}
					result = append(result, genai.NewPartFromBytes(data, content.FileURL.MIMEType))
				}
			}
		}
	}
	return result, nil
}

// decodeBase64DataURL decodes a base64 data URL string into raw bytes.
// It correctly handles the "data:[<mediatype>];base64," prefix.
func decodeBase64DataURL(dataURL string) ([]byte, error) {
	// Check if a web URL is passed by mistake.
	if strings.HasPrefix(dataURL, "http") {
		return nil, fmt.Errorf("invalid input: expected base64 data or data URL, but got a web URL starting with 'http'. Please fetch the content from the URL first")
	}
	// Find the comma that separates the prefix from the data
	commaIndex := strings.Index(dataURL, ",")
	if commaIndex == -1 {
		// If no comma, assume it's a raw base64 string and try to decode it directly.
		decoded, err := base64.StdEncoding.DecodeString(dataURL)
		if err != nil {
			return nil, fmt.Errorf("failed to decode raw base64 data: %w", err)
		}
		return decoded, nil
	}

	// Extract the base64 part of the data URL
	base64Data := dataURL[commaIndex+1:]

	// Decode the base64 string
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data from data URL: %w", err)
	}

	return decoded, nil
}

func (cm *ChatModel) convResponse(resp *genai.GenerateContentResponse) (*schema.Message, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini result is empty")
	}

	message, err := cm.convCandidate(resp.Candidates[0])
	if err != nil {
		return nil, fmt.Errorf("convert candidate fail: %w", err)
	}

	if resp.UsageMetadata != nil {
		if message.ResponseMeta == nil {
			message.ResponseMeta = &schema.ResponseMeta{}
		}
		message.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens: int(resp.UsageMetadata.PromptTokenCount),
			PromptTokenDetails: schema.PromptTokenDetails{
				CachedTokens: int(resp.UsageMetadata.CachedContentTokenCount),
			},
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}
	return message, nil
}

func (cm *ChatModel) convCandidate(candidate *genai.Candidate) (*schema.Message, error) {
	result := &schema.Message{}
	result.ResponseMeta = &schema.ResponseMeta{
		FinishReason: string(candidate.FinishReason),
	}
	if candidate.Content != nil {
		if candidate.Content.Role == roleModel {
			result.Role = schema.Assistant
		} else {
			result.Role = schema.User
		}

		var (
			texts          []string
			outParts       []schema.MessageOutputPart
			contentBuilder strings.Builder
		)
		for _, part := range candidate.Content.Parts {
			if part.Thought {
				result.ReasoningContent = part.Text
			} else if len(part.Text) > 0 {
				texts = append(texts, part.Text)
				contentBuilder.WriteString(part.Text)
				outParts = append(outParts, schema.MessageOutputPart{
					Type: schema.ChatMessagePartTypeText,
					Text: part.Text,
				})
			}
			if part.FunctionCall != nil {
				fc, err := convFC(part.FunctionCall)
				if err != nil {
					return nil, err
				}
				result.ToolCalls = append(result.ToolCalls, *fc)
			}
			if part.CodeExecutionResult != nil {
				texts = append(texts, part.CodeExecutionResult.Output)
				outParts = append(outParts, schema.MessageOutputPart{
					Type: schema.ChatMessagePartTypeText,
					Text: part.CodeExecutionResult.Output,
				})
			}
			if part.ExecutableCode != nil {
				texts = append(texts, part.ExecutableCode.Code)
				outParts = append(outParts, schema.MessageOutputPart{
					Type: schema.ChatMessagePartTypeText,
					Text: part.ExecutableCode.Code,
				})
			}
			if part.InlineData != nil && part.InlineData.Data != nil {
				outPart, err := toMultiOutPart(part)
				if err != nil {
					return nil, err
				}
				outParts = append(outParts, outPart)
			}
		}
		result.Content = contentBuilder.String()
		if len(texts) > 1 {
			for _, text := range texts {
				result.MultiContent = append(result.MultiContent, schema.ChatMessagePart{
					Type: schema.ChatMessagePartTypeText,
					Text: text,
				})
			}
		}
		if len(outParts) > 0 {
			result.AssistantGenMultiContent = outParts
		}
	}
	return result, nil
}

func toMultiOutPart(part *genai.Part) (schema.MessageOutputPart, error) {
	if part == nil {
		return schema.MessageOutputPart{}, nil
	}
	res := schema.MessageOutputPart{}
	if part.InlineData != nil {
		mimeType := part.InlineData.MIMEType
		multiMediaData := part.InlineData.Data
		encodedStr := base64.StdEncoding.EncodeToString(multiMediaData)
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			res.Type = schema.ChatMessagePartTypeImageURL
			res.Image = &schema.MessageOutputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &encodedStr,
					MIMEType:   mimeType,
				},
			}
		default:
			return schema.MessageOutputPart{}, fmt.Errorf("unsupported media type from Gemini model response: MIMEType=%s", mimeType)
		}
	}
	return res, nil
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

func (cm *ChatModel) convCallbackOutput(message *schema.Message, conf *model.Config) *model.CallbackOutput {
	callbackOutput := &model.CallbackOutput{
		Message: message,
		Config:  conf,
	}
	if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
		callbackOutput.TokenUsage = &model.TokenUsage{
			PromptTokens: message.ResponseMeta.Usage.PromptTokens,
			PromptTokenDetails: model.PromptTokenDetails{
				CachedTokens: message.ResponseMeta.Usage.PromptTokenDetails.CachedTokens,
			},
			CompletionTokens: message.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      message.ResponseMeta.Usage.TotalTokens,
		}
	}
	return callbackOutput
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

type GeminiResponseModality string

const (
	GeminiResponseModalityText  GeminiResponseModality = "TEXT"
	GeminiResponseModalityImage GeminiResponseModality = "IMAGE"
	GeminiResponseModalityAudio GeminiResponseModality = "AUDIO"
)

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

const typ = "Gemini"

func (cm *ChatModel) GetType() string {
	return typ
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
