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

package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/cloudwego/eino/components"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

// NewChatModel creates a new Claude chat model instance
//
// Parameters:
//   - ctx: The context for the operation
//   - conf: Configuration for the Claude model
//
// Returns:
//   - model.ChatModel: A chat model interface implementation
//   - error: Any error that occurred during creation
//
// Example:
//
//	model, err := claude.NewChatModel(ctx, &claude.Config{
//	    APIKey: "your-api-key",
//	    Model:  "claude-3-opus-20240229",
//	    MaxTokens: 2000,
//	})
func NewChatModel(ctx context.Context, config *Config) (*ChatModel, error) {
	var cli anthropic.Client
	if !config.ByBedrock {
		var opts []option.RequestOption

		opts = append(opts, option.WithAPIKey(config.APIKey))

		if config.BaseURL != nil {
			opts = append(opts, option.WithBaseURL(*config.BaseURL))
		}

		if config.HTTPClient != nil {
			opts = append(opts, option.WithHTTPClient(config.HTTPClient))
		}

		cli = anthropic.NewClient(opts...)
	} else {
		var opts []func(*awsConfig.LoadOptions) error
		if config.Region != "" {
			opts = append(opts, awsConfig.WithRegion(config.Region))
		}
		if config.SecretAccessKey != "" && config.AccessKey != "" {
			opts = append(opts, awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				config.AccessKey,
				config.SecretAccessKey,
				config.SessionToken,
			)))
		} else if config.Profile != "" {
			opts = append(opts, awsConfig.WithSharedConfigProfile(config.Profile))
		}

		if config.HTTPClient != nil {
			opts = append(opts, awsConfig.WithHTTPClient(config.HTTPClient))
		}
		cli = anthropic.NewClient(bedrock.WithLoadDefaultConfig(ctx, opts...))
	}
	return &ChatModel{
		cli:                    cli,
		maxTokens:              config.MaxTokens,
		model:                  config.Model,
		stopSequences:          config.StopSequences,
		temperature:            config.Temperature,
		thinking:               config.Thinking,
		topK:                   config.TopK,
		topP:                   config.TopP,
		disableParallelToolUse: config.DisableParallelToolUse,
	}, nil
}

// Config contains the configuration options for the Claude model
type Config struct {
	// ByBedrock indicates whether to use Bedrock Service
	// Required for Bedrock
	ByBedrock bool

	// AccessKey is your Bedrock API Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional for Bedrock
	AccessKey string

	// SecretAccessKey is your Bedrock API Secret Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional for Bedrock
	SecretAccessKey string

	// SessionToken is your Bedrock API Session Token
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional for Bedrock
	SessionToken string

	// Profile is your Bedrock API AWS profile
	// This parameter is ignored if AccessKey and SecretAccessKey are provided
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional for Bedrock
	Profile string

	// Region is your Bedrock API region
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional for Bedrock
	Region string

	// BaseURL is the custom API endpoint URL
	// Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
	// Optional. Example: "https://custom-claude-api.example.com"
	BaseURL *string

	// APIKey is your Anthropic API key
	// Obtain from: https://console.anthropic.com/account/keys
	// Required
	APIKey string

	// Model specifies which Claude model to use
	// Required
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Range: 1 to model's context length
	// Required. Example: 2000 for a medium-length response
	MaxTokens int

	// Temperature controls randomness in responses
	// Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
	// Optional. Example: float32(0.7)
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: float32(0.95)
	TopP *float32

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: int32(40)
	TopK *int32

	// StopSequences specifies custom stop sequences
	// The model will stop generating when it encounters any of these sequences
	// Optional. Example: []string{"\n\nHuman:", "\n\nAssistant:"}
	StopSequences []string

	Thinking *Thinking

	// HTTPClient specifies the client to send HTTP requests.
	HTTPClient *http.Client `json:"http_client"`

	DisableParallelToolUse *bool `json:"disable_parallel_tool_use"`
}

type Thinking struct {
	Enable       bool `json:"enable"`
	BudgetTokens int  `json:"budget_tokens"`
}

type ChatModel struct {
	cli anthropic.Client

	maxTokens              int
	model                  string
	stopSequences          []string
	temperature            *float32
	topK                   *int32
	topP                   *float32
	thinking               *Thinking
	tools                  []anthropic.ToolUnionParam
	origTools              []*schema.ToolInfo
	toolChoice             *schema.ToolChoice
	disableParallelToolUse *bool
}

func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (message *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	ctx = callbacks.OnStart(ctx, cm.getCallbackInput(input, opts...))
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	msgParam, err := cm.genMessageNewParams(input, opts...)
	if err != nil {
		return nil, err
	}

	resp, err := cm.cli.Messages.New(ctx, msgParam)
	if err != nil {
		return nil, fmt.Errorf("create new message fail: %w", err)
	}

	message, err = convOutputMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("convert response to schema message fail: %w", err)
	}

	callbacks.OnEnd(ctx, cm.getCallbackOutput(message))

	return message, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (result *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)
	ctx = callbacks.OnStart(ctx, cm.getCallbackInput(input, opts...))
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	msgParam, err := cm.genMessageNewParams(input, opts...)
	if err != nil {
		return nil, err
	}
	stream := cm.cli.Messages.NewStreaming(ctx, msgParam)
	// the stream error that occurred at this time should be terminated and returned.
	if stream.Err() != nil {
		return nil, fmt.Errorf("create new streaming message fail: %w", stream.Err())
	}

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = stream.Close()
			sw.Close()
		}()
		var waitList []*schema.Message
		streamCtx := &streamContext{}
		for stream.Next() {
			message, err_ := convStreamEvent(stream.Current(), streamCtx)
			if err_ != nil {
				_ = sw.Send(nil, fmt.Errorf("convert response chunk to schema message fail: %w", err_))
				return
			}
			if message == nil {
				continue
			}
			if isMessageEmpty(message) {
				waitList = append(waitList, message)
				continue
			}

			if len(waitList) != 0 {
				message, err = schema.ConcatMessages(append(waitList, message))
				if err != nil {
					_ = sw.Send(nil, fmt.Errorf("concat empty message fail: %w", err))
					return
				}
				waitList = []*schema.Message{}
			}

			closed := sw.Send(cm.getCallbackOutput(message), nil)
			if closed {
				return
			}
		}

		if len(waitList) > 0 {
			message, err_ := schema.ConcatMessages(waitList)
			if err_ != nil {
				_ = sw.Send(nil, fmt.Errorf("concat empty message fail: %w", err_))
				return
			}

			closed := sw.Send(cm.getCallbackOutput(message), nil)
			if closed {
				return
			}
		}

		// the loop may terminate due to a stream error.
		if stream.Err() != nil {
			_ = sw.Send(nil, stream.Err())
			return
		}

	}()
	_, sr = callbacks.OnEndWithStreamOutput(ctx, sr)
	return schema.StreamReaderWithConvert(sr, func(t *model.CallbackOutput) (*schema.Message, error) {
		return t.Message, nil
	}), nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	aTools, err := toAnthropicToolParam(tools)
	if err != nil {
		return nil, fmt.Errorf("to anthropic tool param fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = aTools
	ncm.toolChoice = &tc
	ncm.origTools = tools
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	result, err := toAnthropicToolParam(tools)
	if err != nil {
		return err
	}

	cm.tools = result
	cm.origTools = tools
	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	return nil
}

func (cm *ChatModel) BindForcedTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	result, err := toAnthropicToolParam(tools)
	if err != nil {
		return err
	}

	cm.tools = result
	cm.origTools = tools
	tc := schema.ToolChoiceForced
	cm.toolChoice = &tc
	return nil
}

func toAnthropicToolParam(tools []*schema.ToolInfo) ([]anthropic.ToolUnionParam, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		s, err := tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("convert to openapi v3 schema fail: %w", err)
		}

		var inputSchema anthropic.ToolInputSchemaParam
		if s != nil {
			inputSchema = anthropic.ToolInputSchemaParam{
				Properties: s.Properties,
				Required:   s.Required,
			}
		}

		toolParam := &anthropic.ToolParam{
			Name:        tool.Name,
			Description: param.NewOpt(tool.Desc),
			InputSchema: inputSchema,
		}

		if isBreakpointTool(tool) {
			toolParam.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}

		result = append(result, anthropic.ToolUnionParam{OfTool: toolParam})
	}

	return result, nil
}

func preProcessMessages(input []*schema.Message) ([]*schema.Message, []*schema.Message, error) {
	userMsgIdx := -1
	for i, msg := range input {
		if msg.Role != schema.System {
			if msg.Role != schema.User {
				// claude requires first message to be user msg
				// as specified in https://docs.anthropic.com/en/api/messages:
				// 'You can specify a single user-role message,
				// or you can include multiple user and assistant messages.'
				return nil, nil, errors.New("first non-system message should be user message")
			}
			userMsgIdx = i
			break
		}
	}

	if userMsgIdx == -1 {
		return nil, nil, errors.New("only system message in input, require at least 1 user message")
	}

	return input[:userMsgIdx], input[userMsgIdx:], nil
}

func (cm *ChatModel) genMessageNewParams(input []*schema.Message, opts ...model.Option) (
	anthropic.MessageNewParams, error) {
	if len(input) == 0 {
		return anthropic.MessageNewParams{}, fmt.Errorf("input is empty")
	}

	system, msgs, err := preProcessMessages(input)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}

	commonOptions := model.GetCommonOptions(&model.Options{
		Model:       &cm.model,
		Temperature: cm.temperature,
		MaxTokens:   &cm.maxTokens,
		TopP:        cm.topP,
		Stop:        cm.stopSequences,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)
	specOptions := model.GetImplSpecificOptions(&options{
		TopK:                   cm.topK,
		Thinking:               cm.thinking,
		DisableParallelToolUse: cm.disableParallelToolUse}, opts...)

	params := anthropic.MessageNewParams{}
	if commonOptions.Model != nil {
		params.Model = anthropic.Model(*commonOptions.Model)
	}
	if commonOptions.MaxTokens != nil {
		params.MaxTokens = int64(*commonOptions.MaxTokens)
	}
	if commonOptions.Temperature != nil {
		params.Temperature = param.NewOpt(float64(*commonOptions.Temperature))
	}
	if commonOptions.TopP != nil {
		params.TopP = param.NewOpt(float64(*commonOptions.TopP))
	}
	if len(commonOptions.Stop) > 0 {
		params.StopSequences = commonOptions.Stop
	}
	if specOptions.TopK != nil {
		params.TopK = param.NewOpt(int64(*specOptions.TopK))
	}

	if specOptions.Thinking != nil && specOptions.Thinking.Enable {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				Type:         "enabled",
				BudgetTokens: int64(specOptions.Thinking.BudgetTokens),
			},
		}
	}

	if err = cm.populateTools(&params, commonOptions, specOptions); err != nil {
		return anthropic.MessageNewParams{}, err
	}

	if err = cm.populateInput(&params, system, msgs, specOptions); err != nil {
		return anthropic.MessageNewParams{}, err
	}

	return params, nil
}

func (cm *ChatModel) populateInput(params *anthropic.MessageNewParams, system []*schema.Message, msgs []*schema.Message, specOptions *options) error {
	// populate system messages
	hasSetSysBreakPoint := false
	for _, m := range system {
		block := anthropic.TextBlockParam{Text: m.Content}
		if isBreakpointMessage(m) {
			hasSetSysBreakPoint = true
			block.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		params.System = append(params.System, block)
	}

	// if no breakpoint has been set, a breakpoint will be set for the last system message
	if len(params.System) > 0 && !hasSetSysBreakPoint && fromOrDefault(specOptions.EnableAutoCache, false) {
		params.System[len(params.System)-1].CacheControl = anthropic.NewCacheControlEphemeralParam()
	}

	msgParams := make([]anthropic.MessageParam, 0, len(msgs))
	hasSetMsgBreakPoint := false

	for _, msg := range msgs {
		msgParam, err := convSchemaMessage(msg)
		if err != nil {
			return fmt.Errorf("convert schema message fail: %w", err)
		}

		if ctrl := msgParam.Content[len(msgParam.Content)-1].GetCacheControl(); ctrl != nil && ctrl.Type != "" {
			hasSetMsgBreakPoint = true
		}

		msgParams = append(msgParams, msgParam)
	}

	if !hasSetMsgBreakPoint && fromOrDefault(specOptions.EnableAutoCache, false) {
		lastMsgParam := msgParams[len(msgParams)-1]
		lastBlock := lastMsgParam.Content[len(lastMsgParam.Content)-1]
		populateContentBlockBreakPoint(lastBlock)
	}

	params.Messages = msgParams

	return nil
}

func (cm *ChatModel) populateTools(params *anthropic.MessageNewParams, commonOptions *model.Options, specOptions *options) error {
	tools := cm.tools

	if commonOptions.Tools != nil {
		var err error
		if tools, err = toAnthropicToolParam(commonOptions.Tools); err != nil {
			return err
		}
	}

	if len(tools) > 0 && fromOrDefault(specOptions.EnableAutoCache, false) {
		hasBreakpoint := false
		for _, tool := range tools {
			if ctrl := tool.GetCacheControl(); ctrl != nil && ctrl.Type != "" {
				hasBreakpoint = true
				break
			}
		}
		// if no breakpoint has been set, a breakpoint will be set for the last tool
		if !hasBreakpoint {
			tools[len(tools)-1].OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
	}

	params.Tools = tools

	if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			params.Tools = []anthropic.ToolUnionParam{} // act like forbid tools
		case schema.ToolChoiceAllowed:
			p := &anthropic.ToolChoiceAutoParam{}
			if specOptions.DisableParallelToolUse != nil {
				p.DisableParallelToolUse = param.NewOpt[bool](*specOptions.DisableParallelToolUse)
			}
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfAuto: p,
			}
		case schema.ToolChoiceForced:
			if len(tools) == 0 {
				return fmt.Errorf("tool choice is forced but tool is not provided")
			} else if len(tools) == 1 {
				params.ToolChoice = anthropic.ToolChoiceParamOfTool(*tools[0].GetName())
			} else {
				p := &anthropic.ToolChoiceAnyParam{}
				if specOptions.DisableParallelToolUse != nil {
					p.DisableParallelToolUse = param.NewOpt[bool](*specOptions.DisableParallelToolUse)
				}
				params.ToolChoice = anthropic.ToolChoiceUnionParam{
					OfAny: p,
				}
			}
		default:
			return fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}

	return nil
}

func (cm *ChatModel) getCallbackInput(input []*schema.Message, opts ...model.Option) *model.CallbackInput {
	result := &model.CallbackInput{
		Messages: input,
		Tools: model.GetCommonOptions(&model.Options{
			Tools: cm.origTools,
		}, opts...).Tools,
		Config: cm.getConfig(),
	}
	return result
}

func (cm *ChatModel) getCallbackOutput(output *schema.Message) *model.CallbackOutput {
	result := &model.CallbackOutput{
		Message: output,
		Config:  cm.getConfig(),
	}
	if output.ResponseMeta != nil && output.ResponseMeta.Usage != nil {
		result.TokenUsage = &model.TokenUsage{
			PromptTokens: output.ResponseMeta.Usage.PromptTokens,
			PromptTokenDetails: model.PromptTokenDetails{
				CachedTokens: output.ResponseMeta.Usage.PromptTokenDetails.CachedTokens,
			},
			CompletionTokens: output.ResponseMeta.Usage.CompletionTokens,
			TotalTokens:      output.ResponseMeta.Usage.TotalTokens,
		}
	}
	return result
}

func (cm *ChatModel) getConfig() *model.Config {
	result := &model.Config{
		Model:     cm.model,
		MaxTokens: cm.maxTokens,
		Stop:      cm.stopSequences,
	}
	if cm.temperature != nil {
		result.Temperature = *cm.temperature
	}
	if cm.topP != nil {
		result.TopP = *cm.topP
	}
	return result
}

func (cm *ChatModel) GetType() string {
	return "Claude"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func convSchemaMessage(message *schema.Message) (mp anthropic.MessageParam, err error) {
	var messageParams []anthropic.ContentBlockParamUnion

	if message.Role == schema.Assistant {
		thinkingContent, hasThinking := GetThinking(message)
		if hasThinking && thinkingContent != "" {
			signature, hasSignature := getThinkingSignature(message)
			if hasSignature && signature != "" {
				messageParams = append(messageParams, anthropic.NewThinkingBlock(signature, thinkingContent))
			}
		}
	}

	if len(message.UserInputMultiContent) > 0 && len(message.AssistantGenMultiContent) > 0 {
		return mp, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}

	if len(message.Content) > 0 {
		if len(message.ToolCallID) > 0 {
			messageParams = append(messageParams, anthropic.NewToolResultBlock(message.ToolCallID, message.Content, false))
		} else {
			messageParams = append(messageParams, anthropic.NewTextBlock(message.Content))
		}
	} else if len(message.UserInputMultiContent) > 0 {
		if message.Role != schema.User {
			return mp, fmt.Errorf("user input multi content only support user role, got %s", message.Role)
		}
		for i := range message.UserInputMultiContent {
			switch message.UserInputMultiContent[i].Type {
			case schema.ChatMessagePartTypeText:
				messageParams = append(messageParams, anthropic.NewTextBlock(message.UserInputMultiContent[i].Text))
			case schema.ChatMessagePartTypeImageURL:
				if message.UserInputMultiContent[i].Image == nil {
					return mp, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in user message")
				}
				image := message.UserInputMultiContent[i].Image
				if image.URL != nil && *image.URL != "" {
					messageParams = append(messageParams, anthropic.NewImageBlock(anthropic.URLImageSourceParam{
						URL: *image.URL,
					}))
				} else if image.Base64Data != nil && *image.Base64Data != "" {
					if image.MIMEType == "" {
						return mp, fmt.Errorf("image part must have MIMEType when use Base64Data")
					}
					if strings.HasPrefix(*image.Base64Data, "data:") {
						return mp, fmt.Errorf("Base64Data should be a raw base64 string, but it has a 'data:' prefix")
					}
					messageParams = append(messageParams, anthropic.NewImageBlockBase64(image.MIMEType, *image.Base64Data))
				} else {
					return mp, fmt.Errorf("image part must have either a URL or Base64Data")
				}
			default:
				return mp, fmt.Errorf("anthropic message type not supported: %s", message.UserInputMultiContent[i].Type)
			}
		}
	} else if len(message.AssistantGenMultiContent) > 0 {
		if message.Role != schema.Assistant {
			return mp, fmt.Errorf("assistant gen multi content only support assistant role, got %s", message.Role)
		}
		for i := range message.AssistantGenMultiContent {
			switch message.AssistantGenMultiContent[i].Type {
			case schema.ChatMessagePartTypeText:
				messageParams = append(messageParams, anthropic.NewTextBlock(message.AssistantGenMultiContent[i].Text))
			case schema.ChatMessagePartTypeImageURL:
				if message.AssistantGenMultiContent[i].Image == nil {
					return mp, fmt.Errorf("image field must not be nil when Type is ChatMessagePartTypeImageURL in assistant message")
				}
				image := message.AssistantGenMultiContent[i].Image
				if image.URL != nil && *image.URL != "" {
					messageParams = append(messageParams, anthropic.NewImageBlock(anthropic.URLImageSourceParam{
						URL: *image.URL,
					}))
				} else if image.Base64Data != nil && *image.Base64Data != "" {
					if image.MIMEType == "" {
						return mp, fmt.Errorf("image part must have MIMEType when use Base64Data")
					}
					if strings.HasPrefix(*image.Base64Data, "data:") {
						return mp, fmt.Errorf("Base64Data should be a raw base64 string, but it has a 'data:' prefix")
					}
					messageParams = append(messageParams, anthropic.NewImageBlockBase64(image.MIMEType, *image.Base64Data))
				} else {
					return mp, fmt.Errorf("image part must have either a URL or Base64Data")
				}
			default:
				return mp, fmt.Errorf("anthropic message type not supported: %s", message.AssistantGenMultiContent[i].Type)
			}
		}
	} else {
		// The `MultiContent` field is deprecated. In its design, the `URL` field of `ImageURL`
		// could contain either an HTTP URL or a Base64-encoded DATA URL. This is different from the new
		// `UserInputMultiContent` and `AssistantGenMultiContent` fields, where `URL` and `Base64Data` are separate.
		log.Printf("MultiContent is deprecated, please use UserInputMultiContent or AssistantGenMultiContent instead")
		for i := range message.MultiContent {
			switch message.MultiContent[i].Type {
			case schema.ChatMessagePartTypeText:
				messageParams = append(messageParams, anthropic.NewTextBlock(message.MultiContent[i].Text))
			case schema.ChatMessagePartTypeImageURL:
				if message.MultiContent[i].ImageURL == nil {
					continue
				}
				if strings.HasPrefix(message.MultiContent[i].ImageURL.URL, "http") {
					messageParams = append(messageParams, anthropic.NewImageBlock(anthropic.URLImageSourceParam{
						URL: message.MultiContent[i].ImageURL.URL,
					}))
					continue
				}
				mediaType, data, err_ := convImageBase64(message.MultiContent[i].ImageURL.URL)
				if err_ != nil {
					return mp, fmt.Errorf("extract base64 image fail: %w", err_)
				}
				messageParams = append(messageParams, anthropic.NewImageBlockBase64(mediaType, data))
			default:
				return mp, fmt.Errorf("anthropic message type not supported: %s", message.MultiContent[i].Type)
			}
		}
	}

	for i := range message.ToolCalls {
		tc := message.ToolCalls[i]

		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		// Arguments are limited to object type.
		// Since json marshaling will be performed before the request,
		// and arguments are already a json string, marshaling should not be performed,
		// so it needs to be forcibly converted to json.RawMessage.
		messageParams = append(messageParams, anthropic.NewToolUseBlock(tc.ID, json.RawMessage(args), tc.Function.Name))
	}

	if len(messageParams) > 0 && isBreakpointMessage(message) {
		populateContentBlockBreakPoint(messageParams[len(messageParams)-1])
	}

	switch message.Role {
	case schema.Assistant:
		mp = anthropic.NewAssistantMessage(messageParams...)
	case schema.User:
		mp = anthropic.NewUserMessage(messageParams...)
	default:
		mp = anthropic.NewUserMessage(messageParams...)
	}

	return mp, nil
}

func populateContentBlockBreakPoint(block anthropic.ContentBlockParamUnion) {
	if block.OfText != nil {
		block.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
	if block.OfImage != nil {
		block.OfImage.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
	if block.OfToolResult != nil {
		block.OfToolResult.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
	if block.OfToolUse != nil {
		block.OfToolUse.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
}

func convOutputMessage(resp *anthropic.Message) (*schema.Message, error) {
	promptTokens := int(resp.Usage.InputTokens + resp.Usage.CacheReadInputTokens + resp.Usage.CacheCreationInputTokens)

	message := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(resp.StopReason),
			Usage: &schema.TokenUsage{
				PromptTokens: promptTokens,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: int(resp.Usage.CacheReadInputTokens),
				},
				CompletionTokens: int(resp.Usage.OutputTokens),
				TotalTokens:      promptTokens + int(resp.Usage.OutputTokens),
			},
		},
	}

	streamCtx := &streamContext{}
	for _, item := range resp.Content {
		err := convContentBlockToEinoMsg(item.AsAny(), message, streamCtx)
		if err != nil {
			return nil, err
		}
	}

	return message, nil
}

type streamContext struct {
	toolIndex *int
}

func convContentBlockToEinoMsg(
	contentBlock any, dstMsg *schema.Message, streamCtx *streamContext) error {
	//	case anthropic.TextBlock:
	//	case anthropic.ToolUseBlock:
	//	case anthropic.ServerToolUseBlock:
	//	case anthropic.WebSearchToolResultBlock:
	//	case anthropic.ThinkingBlock:
	//	case anthropic.RedactedThinkingBlock:
	switch block := contentBlock.(type) {
	case anthropic.TextBlock:
		dstMsg.Content += block.Text
	case anthropic.ToolUseBlock:
		dstMsg.ToolCalls = append(dstMsg.ToolCalls,
			toolEvent(true, block.ID, block.Name, block.Input, streamCtx))
	case anthropic.ServerToolUseBlock:
		return fmt.Errorf("server_tool_use not supported")
	case anthropic.WebSearchToolResultBlock:
		return fmt.Errorf("web_search tool not supported")
	case anthropic.ThinkingBlock:
		setThinking(dstMsg, block.Thinking)
		dstMsg.ReasoningContent = block.Thinking
		setThinkingSignature(dstMsg, block.Signature)
	case anthropic.RedactedThinkingBlock:
	default:
		return fmt.Errorf("unknown anthropic content block type: %T", block)
	}

	return nil
}

func convStreamEvent(event anthropic.MessageStreamEventUnion, streamCtx *streamContext) (*schema.Message, error) {
	result := &schema.Message{
		Role:  schema.Assistant,
		Extra: make(map[string]any),
	}

	//	case anthropic.MessageStartEvent:
	//	case anthropic.MessageDeltaEvent:
	//	case anthropic.MessageStopEvent:
	//	case anthropic.ContentBlockStartEvent:
	//	case anthropic.ContentBlockDeltaEvent:
	//	case anthropic.ContentBlockStopEvent:
	switch e := event.AsAny().(type) {
	case anthropic.MessageStartEvent:
		return convOutputMessage(&e.Message)
	case anthropic.MessageDeltaEvent:
		result.ResponseMeta = &schema.ResponseMeta{
			FinishReason: string(e.Delta.StopReason),
			Usage: &schema.TokenUsage{
				CompletionTokens: int(e.Usage.OutputTokens),
			},
		}
		return result, nil

	case anthropic.MessageStopEvent, anthropic.ContentBlockStopEvent:
		return nil, nil
	case anthropic.ContentBlockStartEvent:
		//	case anthropic.TextBlock:
		//	case anthropic.ToolUseBlock:
		//	case anthropic.ServerToolUseBlock:
		//	case anthropic.WebSearchToolResultBlock:
		//	case anthropic.ThinkingBlock:
		//	case anthropic.RedactedThinkingBlock:
		err := convContentBlockToEinoMsg(e.ContentBlock.AsAny(), result, streamCtx)
		if err != nil {
			return nil, err
		}
		return result, nil

	case anthropic.ContentBlockDeltaEvent:
		//	case anthropic.TextDelta:
		//	case anthropic.InputJSONDelta:
		//	case anthropic.CitationsDelta:
		//	case anthropic.ThinkingDelta:
		//	case anthropic.SignatureDelta:
		switch delta := e.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			result.Content = delta.Text
		case anthropic.ThinkingDelta:
			setThinking(result, delta.Thinking)
			result.ReasoningContent = delta.Thinking
		case anthropic.InputJSONDelta:
			result.ToolCalls = append(result.ToolCalls,
				toolEvent(false, "", "", delta.PartialJSON, streamCtx))
		case anthropic.SignatureDelta:
			if currentSig, hasSig := getThinkingSignature(result); hasSig {
				setThinkingSignature(result, currentSig+delta.Signature)
			} else {
				setThinkingSignature(result, delta.Signature)
			}
		}

		return result, nil

	default:
		return nil, fmt.Errorf("unknown stream event type: %T", e)
	}
}

func convImageBase64(data string) (string, string, error) {
	if !strings.HasPrefix(data, "data:") {
		return "", "", fmt.Errorf("invalid base64 image: %s", data)
	}
	contents := strings.SplitN(data[5:], ",", 2)
	if len(contents) != 2 {
		return "", "", fmt.Errorf("invalid base64 image: %s", data)
	}
	headParts := strings.Split(contents[0], ";")
	bBase64 := false
	for _, part := range headParts {
		if part == "base64" {
			bBase64 = true
		}
	}
	if !bBase64 {
		return "", "", fmt.Errorf("invalid base64 image: %s", data)
	}
	return headParts[0], contents[1], nil
}

func isMessageEmpty(message *schema.Message) bool {
	_, ok := GetThinking(message)
	if len(message.Content) == 0 && len(message.ToolCalls) == 0 && len(message.MultiContent) == 0 && !ok {
		return true
	}
	return false
}

func toolEvent(isStart bool, toolCallID, toolName string, input any, sc *streamContext) schema.ToolCall {
	// count tool call index for stream
	if isStart {
		if sc.toolIndex == nil {
			sc.toolIndex = of(-1)
		}
		*sc.toolIndex++
	} else if sc.toolIndex == nil {
		sc.toolIndex = of(0)
	}

	toolIndex := sc.toolIndex

	arguments := ""
	if rm, ok := input.(json.RawMessage); ok {
		arguments = string(rm)
	} else if arg, ok_ := input.(string); ok_ {
		arguments = arg
	}

	// If the arguments of the tool call are empty,
	// Claude will repeatedly output multiple identical streaming chunks, and the arguments are all "{}"
	// There will be problems when concat streaming chunks.
	if arguments == "{}" {
		arguments = ""
	}

	return schema.ToolCall{
		Index: toolIndex,
		ID:    toolCallID,
		Function: schema.FunctionCall{
			Name:      toolName,
			Arguments: arguments,
		},
	}
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
