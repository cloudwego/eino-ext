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
	claudeOptions := model.GetImplSpecificOptions(&options{
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
	if claudeOptions.TopK != nil {
		params.TopK = param.NewOpt(int64(*claudeOptions.TopK))
	}

	if claudeOptions.Thinking != nil && claudeOptions.Thinking.Enable {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				Type:         "enabled",
				BudgetTokens: int64(claudeOptions.Thinking.BudgetTokens),
			},
		}
	}

	tools := cm.tools
	if commonOptions.Tools != nil {
		var err error
		if tools, err = toAnthropicToolParam(commonOptions.Tools); err != nil {
			return anthropic.MessageNewParams{}, err
		}
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	if commonOptions.ToolChoice != nil {
		switch *commonOptions.ToolChoice {
		case schema.ToolChoiceForbidden:
			params.Tools = []anthropic.ToolUnionParam{} // act like forbid tools
		case schema.ToolChoiceAllowed:
			p := &anthropic.ToolChoiceAutoParam{}
			if claudeOptions.DisableParallelToolUse != nil {
				p.DisableParallelToolUse = param.NewOpt[bool](*claudeOptions.DisableParallelToolUse)
			}
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfAuto: p,
			}
		case schema.ToolChoiceForced:
			if len(tools) == 0 {
				return anthropic.MessageNewParams{}, fmt.Errorf("tool choice is forced but tool is not provided")
			} else if len(tools) == 1 {
				params.ToolChoice = anthropic.ToolChoiceParamOfTool(*tools[0].GetName())
			} else {
				p := &anthropic.ToolChoiceAnyParam{}
				if claudeOptions.DisableParallelToolUse != nil {
					p.DisableParallelToolUse = param.NewOpt[bool](*claudeOptions.DisableParallelToolUse)
				}
				params.ToolChoice = anthropic.ToolChoiceUnionParam{
					OfAny: p,
				}
			}
		default:
			return anthropic.MessageNewParams{}, fmt.Errorf("tool choice=%s not support", *commonOptions.ToolChoice)
		}
	}

	// Convert messages
	var systemTextBlocks []anthropic.TextBlockParam
	for _, m := range system {
		block := anthropic.TextBlockParam{Text: m.Content}
		if isBreakpointMessage(m) {
			block.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		systemTextBlocks = append(systemTextBlocks, block)
	}
	if len(systemTextBlocks) > 0 {
		params.System = systemTextBlocks
	}

	messages := make([]anthropic.MessageParam, 0, len(msgs))
	for _, msg := range msgs {
		message, err := convSchemaMessage(msg)
		if err != nil {
			return anthropic.MessageNewParams{}, fmt.Errorf("convert schema message fail: %w", err)
		}
		messages = append(messages, message)
	}
	params.Messages = messages

	return params, nil
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
			signature, hasSignature := GetThinkingSignature(message)
			if hasSignature && signature != "" {
				messageParams = append(messageParams, anthropic.NewThinkingBlock(signature, thinkingContent))
			}
		}
	}

	if len(message.Content) > 0 {
		if len(message.ToolCallID) > 0 {
			messageParams = append(messageParams, anthropic.NewToolResultBlock(message.ToolCallID, message.Content, false))
		} else {
			messageParams = append(messageParams, anthropic.NewTextBlock(message.Content))
		}
	} else {
		for i := range message.MultiContent {
			switch message.MultiContent[i].Type {
			case schema.ChatMessagePartTypeText:
				messageParams = append(messageParams, anthropic.NewTextBlock(message.MultiContent[i].Text))
			case schema.ChatMessagePartTypeImageURL:
				if message.MultiContent[i].ImageURL == nil {
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
		messageParams = append(messageParams, anthropic.NewToolUseBlock(message.ToolCalls[i].ID,
			json.RawMessage(message.ToolCalls[i].Function.Arguments),
			message.ToolCalls[i].Function.Name))
	}

	switch message.Role {
	case schema.Assistant:
		mp = anthropic.NewAssistantMessage(messageParams...)
	case schema.User:
		mp = anthropic.NewUserMessage(messageParams...)
	default:
		mp = anthropic.NewUserMessage(messageParams...)
	}

	if isBreakpointMessage(message) && len(messageParams) > 0 {
		injectContentBlockBreakPoint(messageParams[len(messageParams)-1])
	}

	return mp, nil
}

func injectContentBlockBreakPoint(lastBlock anthropic.ContentBlockParamUnion) {
	if lastBlock.OfText != nil {
		lastBlock.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
	if lastBlock.OfImage != nil {
		lastBlock.OfImage.CacheControl = anthropic.NewCacheControlEphemeralParam()
		return
	}
	if lastBlock.OfToolResult != nil {
		lastBlock.OfToolResult.CacheControl = anthropic.NewCacheControlEphemeralParam()
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
		SetThinkingSignature(dstMsg, block.Signature)
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
			if currentSig, hasSig := GetThinkingSignature(result); hasSig {
				SetThinkingSignature(result, currentSig+delta.Signature)
			} else {
				SetThinkingSignature(result, delta.Signature)
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
