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
	"time"

	"github.com/bytedance/sonic"
	"github.com/eino-contrib/jsonschema"
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
		responseJSONSchema:  cfg.ResponseJSONSchema,
		enableCodeExecution: cfg.EnableCodeExecution,
		safetySettings:      cfg.SafetySettings,
		thinkingConfig:      cfg.ThinkingConfig,
		responseModalities:  cfg.ResponseModalities,
		mediaResolution:     cfg.MediaResolution,
		cache:               cfg.Cache,
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

	// ResponseJSONSchema defines the structure for JSON responses
	// Optional. Used when you want structured output in JSON format
	ResponseJSONSchema *jsonschema.Schema

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

	MediaResolution genai.MediaResolution

	// Cache controls prefix cache settings for the model.
	// Optional. used to CreatePrefixCache for reused inputs.
	Cache *CacheConfig
}

// CacheConfig controls prefix cache settings for the model.
type CacheConfig struct {
	// TTL specifies how long cached resources remain valid (now + TTL).
	TTL time.Duration `json:"ttl,omitempty"`
	// ExpireTime sets the absolute expiration timestamp for cached resources.
	ExpireTime time.Time `json:"expireTime,omitempty"`
}

type ChatModel struct {
	cli *genai.Client

	model               string
	maxTokens           *int
	topP                *float32
	temperature         *float32
	topK                *int32
	responseJSONSchema  *jsonschema.Schema
	tools               []*genai.FunctionDeclaration
	origTools           []*schema.ToolInfo
	toolChoice          *schema.ToolChoice
	enableCodeExecution bool
	safetySettings      []*genai.SafetySetting
	thinkingConfig      *genai.ThinkingConfig
	responseModalities  []GeminiResponseModality
	mediaResolution     genai.MediaResolution
	cache               *CacheConfig
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	modelName, nInput, genaiConf, cbConf, err := cm.genInputAndConf(input, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for Generate failed: %w", err)
	}

	co := model.GetCommonOptions(&model.Options{
		Tools:      cm.origTools,
		ToolChoice: cm.toolChoice,
	}, opts...)
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      co.Tools,
		ToolChoice: co.ToolChoice,
		Config:     cbConf,
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
		return nil, fmt.Errorf("genInputAndConf for Stream failed: %w", err)
	}

	co := model.GetCommonOptions(&model.Options{
		Tools:      cm.origTools,
		ToolChoice: cm.toolChoice,
	}, opts...)
	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages:   input,
		Tools:      co.Tools,
		ToolChoice: co.ToolChoice,
		Config:     cbConf,
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

// CreatePrefixCache assembles inputs the same as Generate/Stream and writes
// the final system instruction, tools, and messages into a reusable prefix cache.
func (cm *ChatModel) CreatePrefixCache(ctx context.Context, prefixMsgs []*schema.Message, opts ...model.Option) (
	*genai.CachedContent, error) {

	modelName, inputMsgs, genaiConf, _, err := cm.genInputAndConf(prefixMsgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for CreatePrefixCache failed: %w", err)
	}

	contents, err := cm.convSchemaMessages(inputMsgs)
	if err != nil {
		return nil, err
	}

	cachedContent, err := cm.cli.Caches.Create(ctx, modelName, &genai.CreateCachedContentConfig{
		Contents:          contents,
		SystemInstruction: genaiConf.SystemInstruction,
		Tools:             genaiConf.Tools,
		ToolConfig:        genaiConf.ToolConfig,
		TTL: func() time.Duration {
			if cm.cache != nil {
				return cm.cache.TTL
			}
			return 0
		}(),
		ExpireTime: func() time.Time {
			if cm.cache != nil {
				return cm.cache.ExpireTime
			}
			return time.Time{}
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("create cache failed: %w", err)
	}

	return cachedContent, nil
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
		ResponseJSONSchema: cm.responseJSONSchema,
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

	m.MediaResolution = cm.mediaResolution

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

	if geminiOptions.ResponseJSONSchema != nil {
		m.ResponseMIMEType = "application/json"
		m.ResponseJsonSchema = geminiOptions.ResponseJSONSchema
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

	if len(geminiOptions.CachedContentName) > 0 {
		m.CachedContent = geminiOptions.CachedContentName
		// remove system instruction and tools when using cached content
		m.SystemInstruction = nil
		m.Tools = nil
		m.ToolConfig = nil
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

		var err error
		funcDecl.ParametersJsonSchema, err = tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("convert to json schema fail: %w", err)
		}

		gTools[i] = funcDecl
	}

	return gTools, nil
}

// convToolMessageToPart converts a tool response message into a Gemini part.
func (cm *ChatModel) convToolMessageToPart(toolName, content string) (*genai.Part, error) {
	response := make(map[string]any)
	err := sonic.UnmarshalString(content, &response)
	if err != nil {
		response = map[string]any{"output": content}
	}

	return genai.NewPartFromFunctionResponse(toolName, response), nil
}

func (cm *ChatModel) convSchemaMessages(messages []*schema.Message) ([]*genai.Content, error) {
	var result []*genai.Content

	for i := 0; i < len(messages); i++ {
		message := messages[i]
		if message == nil {
			continue
		}

		content, err := cm.convSchemaMessage(message)
		if err != nil {
			return nil, fmt.Errorf("convert schema message fail at index %d: %w", i, err)
		}
		if content != nil {
			result = append(result, content)
		}
	}

	return mergeAdjacentToolContents(result), nil
}

// mergeAdjacentToolContents merges adjacent tool response contents into a single content.
// Gemini requires all tool responses to be in a single message when responding to parallel tool calls.
func mergeAdjacentToolContents(contents []*genai.Content) []*genai.Content {
	if len(contents) <= 1 {
		return contents
	}

	result := make([]*genai.Content, 0, len(contents))

	for _, content := range contents {
		// Check if current content is a tool response (has FunctionResponse parts)
		if len(result) > 0 && isToolResponseContent(content) && isToolResponseContent(result[len(result)-1]) {
			// Merge into the previous content
			result[len(result)-1].Parts = append(result[len(result)-1].Parts, content.Parts...)
		} else {
			result = append(result, content)
		}
	}

	return result
}

// isToolResponseContent checks if a content contains tool response parts.
func isToolResponseContent(content *genai.Content) bool {
	if content == nil || len(content.Parts) == 0 {
		return false
	}
	// Check if the first part is a FunctionResponse
	return content.Parts[0].FunctionResponse != nil
}

func (cm *ChatModel) convSchemaMessage(message *schema.Message) (*genai.Content, error) {
	if message == nil {
		return nil, nil
	}

	if message.Role == schema.Tool {
		toolName := message.ToolName
		if len(toolName) == 0 {
			// For compatibility with Gemini, which does not provide a tool call id, this wrapper assigns the tool name to the tool call id field,
			// falling back to the original toolCallId if tool name is empty.
			toolName = message.ToolCallID
		}
		part, err := cm.convToolMessageToPart(toolName, message.Content)
		if err != nil {
			return nil, err
		}
		return &genai.Content{
			Role:  roleUser,
			Parts: []*genai.Part{part},
		}, nil
	}

	content := &genai.Content{
		Role: toGeminiRole(message.Role),
	}

	// Restore reasoning content as a thought part (required for gemini-3-pro-preview and later)
	if message.ReasoningContent != "" {
		thoughtPart := &genai.Part{
			Text:    message.ReasoningContent,
			Thought: true,
		}
		content.Parts = append(content.Parts, thoughtPart)
	}

	for i := range message.ToolCalls {
		call := &message.ToolCalls[i]
		args := make(map[string]any)
		err := sonic.UnmarshalString(call.Function.Arguments, &args)
		if err != nil {
			return nil, fmt.Errorf("unmarshal schema tool call arguments to map[string]any fail: %w", err)
		}

		part := genai.NewPartFromFunctionCall(call.Function.Name, args)
		// Restore thought signature on the functionCall part if present.
		// Per Gemini docs (https://cloud.google.com/vertex-ai/generative-ai/docs/thought-signatures):
		// - Signatures must be returned exactly as received on functionCall parts
		// - For parallel calls: only first functionCall has signature
		// - For sequential calls: each functionCall has its own signature
		// - Omitting required signature causes 400 error on Gemini 3 Pro
		if sig := getToolCallThoughtSignature(call); len(sig) > 0 {
			part.ThoughtSignature = sig
		}
		content.Parts = append(content.Parts, part)
	}

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
		textPart := genai.NewPartFromText(message.Content)
		// For non-functionCall responses, restore thought signature on the final text part.
		// Per Gemini docs (https://cloud.google.com/vertex-ai/generative-ai/docs/thought-signatures):
		// - The final Part (text, inlineData, etc.) may contain a thought_signature
		// - Returning this signature is recommended for best performance but not strictly required
		if len(message.ToolCalls) == 0 {
			if sig := getMessageThoughtSignature(message); len(sig) > 0 {
				textPart.ThoughtSignature = sig
			}
		}
		content.Parts = append(content.Parts, textPart)
	}
	if len(message.MultiContent) > 0 {
		log.Printf("MultiContent field is deprecated, please use UserInputMultiContent or AssistantGenMultiContent instead")
		parts, err := cm.convMedia(message.MultiContent)
		if err != nil {
			return nil, err
		}
		content.Parts = parts
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Image.URL, MIMEType: content.Image.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Audio.URL, MIMEType: content.Audio.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Video.URL, MIMEType: content.Video.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.File.URL, MIMEType: content.File.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Image.URL, MIMEType: content.Image.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Audio.URL, MIMEType: content.Audio.MIMEType}))
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
				result = append(result, genai.NewPartFromFile(genai.File{URI: *content.Video.URL, MIMEType: content.Video.MIMEType}))
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
			CompletionTokensDetails: schema.CompletionTokensDetails{
				ReasoningTokens: int(resp.UsageMetadata.ThoughtsTokenCount),
			},
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
		if candidate.Content.Role == roleUser {
			result.Role = schema.User
		} else {
			result.Role = schema.Assistant
		}

		var (
			texts          []string
			outParts       []schema.MessageOutputPart
			contentBuilder strings.Builder
		)
		// Process parts and extract thought signatures per Gemini docs:
		// https://cloud.google.com/vertex-ai/generative-ai/docs/thought-signatures
		//
		// Signature placement rules:
		// - functionCall parts: signature stored on ToolCall.Extra (required for Gemini 3 Pro)
		// - non-functionCall parts (text, thought, inlineData): signature stored on Message.Extra
		for _, part := range candidate.Content.Parts {
			// Store thought signature at message level for non-functionCall parts
			if len(part.ThoughtSignature) > 0 && part.FunctionCall == nil {
				setMessageThoughtSignature(result, part.ThoughtSignature)
			}

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
				fc, err := convFC(part)
				if err != nil {
					return nil, err
				}
				// Store thought signature on the tool call if present
				// Per Gemini docs: for parallel calls, only first functionCall has signature;
				// for sequential calls, each functionCall has its own signature
				if len(part.ThoughtSignature) > 0 {
					setToolCallThoughtSignature(fc, part.ThoughtSignature)
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

func convFC(part *genai.Part) (*schema.ToolCall, error) {
	if part == nil || part.FunctionCall == nil {
		return nil, fmt.Errorf("part or function call is nil")
	}

	tp := part.FunctionCall
	args, err := sonic.MarshalString(tp.Args)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini tool call arguments fail: %w", err)
	}

	toolCall := &schema.ToolCall{
		ID: tp.Name,
		Function: schema.FunctionCall{
			Name:      tp.Name,
			Arguments: args,
		},
	}

	return toolCall, nil
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
			CompletionTokensDetails: model.CompletionTokensDetails{
				ReasoningTokens: message.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens,
			},
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
