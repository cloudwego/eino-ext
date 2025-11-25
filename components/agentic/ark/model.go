/*
 * Copyright 2025 CloudWeGo Authors
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

package ark

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bytedance/sonic"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/agentic"
	"github.com/cloudwego/eino/schema"
)

var _ agentic.Model = (*Model)(nil)

type Config struct {
	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional.
	Timeout *time.Duration

	// HTTPClient specifies the HTTP client used to send requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client

	// RetryTimes specifies the number of retry attempts for failed API calls.
	// Optional.
	RetryTimes *int

	// BaseURL specifies the base URL for the Ark service endpoint.
	// Optional.
	BaseURL string

	// Region specifies the geographic region where the Ark service is located.
	// Optional.
	Region string

	// APIKey specifies the API key for authentication.
	// Either APIKey or both AccessKey and SecretKey must be provided.
	// APIKey takes precedence if both authentication methods are provided.
	// For details, see: https://www.volcengine.com/docs/82379/1298459
	APIKey string

	// AccessKey specifies the access key for authentication.
	// Must be used together with SecretKey.
	AccessKey string

	// SecretKey specifies the secret key for authentication.
	// Must be used together with AccessKey.
	SecretKey string

	// Model specifies the identifier of the model endpoint on the Ark platform.
	// For details, see: https://www.volcengine.com/docs/82379/1298454
	// Required.
	Model string

	// MaxOutputTokens specifies the maximum number of tokens to generate in the response.
	// Optional.
	MaxOutputTokens *int64

	// Temperature controls the randomness of the model's output.
	// Lower values (e.g., 0.2) make the output more focused and deterministic.
	// Higher values (e.g., 1.0) make the output more creative and varied.
	// Range: 0.0 to 2.0.
	// Optional.
	Temperature *float64

	// TopP controls diversity via nucleus sampling, an alternative to Temperature.
	// TopP specifies the cumulative probability threshold for token selection.
	// For example, 0.1 means only tokens comprising the top 10% probability mass are considered.
	// We recommend using either Temperature or TopP, but not both.
	// Range: 0.0 to 1.0.
	// Optional.
	TopP *float64

	// ServiceTier specifies the service tier to use for the request.
	// Optional.
	ServiceTier *responses.ResponsesServiceTier_Enum

	// Text specifies text generation configuration options.
	// Optional.
	Text *responses.ResponsesText

	// Thinking controls whether the model uses deep thinking mode.
	// Optional.
	Thinking *responses.ResponsesThinking

	// Reasoning specifies the effort level for the model's reasoning process.
	// Optional.
	Reasoning *responses.ResponsesReasoning

	// MaxToolCalls specifies the maximum number of tool calls the model can make in a single response.
	// Optional.
	MaxToolCalls *int64

	// ParallelToolCalls determines whether the model can invoke multiple tools simultaneously.
	// Optional.
	ParallelToolCalls *bool

	// ServerTools specifies server-side tools available to the model.
	// Optional.
	ServerTools []*ServerToolConfig

	// MCPTools specifies Model Context Protocol tools available to the model.
	// Optional.
	MCPTools []*responses.ToolMcp

	// Cache specifies response caching configuration for the session.
	// Optional.
	Cache *CacheConfig

	// CustomHeader specifies custom HTTP headers to include in API requests.
	// CustomHeader allows passing additional metadata or authentication information.
	// Optional.
	CustomHeader map[string]string
}

type CacheConfig struct {
	// SessionCache can be overridden by [WithCache].
	// Optional.
	SessionCache *SessionCacheConfig
}

type SessionCacheConfig struct {
	// EnableCache controls whether session caching is active.
	// When enabled, the model stores both inputs and responses for each conversation turn,
	// allowing them to be retrieved later via API.
	// Response IDs are saved in output messages and can be accessed using GetResponseID.
	// For multi-turn conversations, the ARK ChatModel automatically identifies the most recent
	// cached message from all inputs and passes its response ID to model to maintain context continuity.
	// This message and all previous ones are trimmed before being sent to the model.
	// When both HeadPreviousResponseID and cached message exist, the message's response ID takes precedence.
	// Use InvalidateMessageCaches to disables caching for the specified messages.
	EnableCache bool

	ExpireAtSec int64
}

type ServerToolConfig struct {
	WebSearch *responses.ToolWebSearch
}

func New(_ context.Context, config *Config) (*Model, error) {
	if config == nil {
		config = &Config{}
	}

	c, err := buildClient(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func buildClient(config *Config) (*Model, error) {
	var opts []arkruntime.ConfigOption

	if config.Region != "" {
		opts = append(opts, arkruntime.WithRegion(config.Region))
	}
	if config.Timeout != nil {
		opts = append(opts, arkruntime.WithTimeout(*config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, arkruntime.WithHTTPClient(config.HTTPClient))
	}
	if config.RetryTimes != nil {
		opts = append(opts, arkruntime.WithRetryTimes(*config.RetryTimes))
	}
	if config.BaseURL != "" {
		opts = append(opts, arkruntime.WithBaseUrl(config.BaseURL))
	}

	var client *arkruntime.Client
	if len(config.APIKey) > 0 {
		client = arkruntime.NewClientWithApiKey(config.APIKey, opts...)
	} else if config.AccessKey != "" && config.SecretKey != "" {
		client = arkruntime.NewClientWithAkSk(config.AccessKey, config.SecretKey, opts...)
	} else {
		return nil, fmt.Errorf("new client fail, missing credentials: set 'APIKey' or both 'AccessKey' and 'SecretKey'")
	}

	cm := &Model{
		cli:               client,
		model:             config.Model,
		maxOutputTokens:   config.MaxOutputTokens,
		temperature:       config.Temperature,
		topP:              config.TopP,
		serviceTier:       config.ServiceTier,
		text:              config.Text,
		thinking:          config.Thinking,
		reasoning:         config.Reasoning,
		maxToolCalls:      config.MaxToolCalls,
		parallelToolCalls: config.ParallelToolCalls,
		serverTools:       config.ServerTools,
		mcpTools:          config.MCPTools,
		cache:             config.Cache,
		customHeader:      config.CustomHeader,
	}

	return cm, nil
}

type Model struct {
	cli *arkruntime.Client

	rawFunctionTools []*schema.ToolInfo
	functionTools    []*responses.ResponsesTool

	model             string
	maxOutputTokens   *int64
	temperature       *float64
	topP              *float64
	serviceTier       *responses.ResponsesServiceTier_Enum
	text              *responses.ResponsesText
	thinking          *responses.ResponsesThinking
	reasoning         *responses.ResponsesReasoning
	maxToolCalls      *int64
	parallelToolCalls *bool
	serverTools       []*ServerToolConfig
	mcpTools          []*responses.ToolMcp

	cache        *CacheConfig
	customHeader map[string]string
}

func (m *Model) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...agentic.Option) (
	outMsg *schema.AgenticMessage, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	responseReq, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("genRequestAndOptions failed: %w", err)
	}

	config := m.toCallbackConfig(responseReq)

	tools := m.rawFunctionTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &agentic.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	responseObject, err := m.cli.CreateResponses(ctx, responseReq, arkruntime.WithCustomHeaders(specOptions.customHeaders))
	if err != nil {
		return nil, fmt.Errorf("failed to create responses: %w", err)
	}

	outMsg, err = toOutputMessage(responseObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to message: %w", err)
	}

	callbacks.OnEnd(ctx, &agentic.CallbackOutput{
		Message: outMsg,
		Config:  config,
	})

	return outMsg, nil
}

func (m *Model) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...agentic.Option) (
	outStream *schema.StreamReader[*schema.AgenticMessage], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	responseReq, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("genRequestAndOptions failed: %w", err)
	}

	config := m.toCallbackConfig(responseReq)
	tools := m.rawFunctionTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &agentic.CallbackInput{
		Messages:   input,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	responseStreamReader, err := m.cli.CreateResponsesStream(ctx, responseReq, arkruntime.WithCustomHeaders(specOptions.customHeaders))
	if err != nil {
		return nil, fmt.Errorf("failed to create responses: %w", err)
	}

	sr, sw := schema.Pipe[*agentic.CallbackOutput](1)

	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = responseStreamReader.Close()
			sw.Close()
		}()

		receivedStreamResponse(responseStreamReader, config, sw)

	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *agentic.CallbackOutput) (callbacks.CallbackOutput, error) {
			if src.Extra == nil {
				src.Extra = make(map[string]any)
			}
			return src, nil
		},
	))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.AgenticMessage, error) {
			s := src.(*agentic.CallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}
			return s.Message, nil
		},
	)

	return outStream, err
}

func (m *Model) WithTools(functionTools []*schema.ToolInfo) (agentic.Model, error) {
	if len(functionTools) == 0 {
		return nil, errors.New("function tools are required")
	}

	fts, err := m.toFunctionTools(functionTools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert function tools: %w", err)
	}

	nam := *m
	nam.rawFunctionTools = functionTools
	nam.functionTools = fts

	return &nam, nil
}

func (m *Model) GetType() string {
	return implType
}

func (m *Model) IsCallbacksEnabled() bool {
	return true
}

type CacheInfo struct {
	// ResponseID return by ResponsesAPI, it's specifies the id of prefix that can be used with [WithCache.HeadPreviousResponseID] option.
	ResponseID string
	// Usage specifies the token usage of prefix
	Usage schema.TokenUsage
}

// CreatePrefixCache creates a prefix context on the server side.
// In each subsequent turn of conversation, use [WithCache] to pass in the ContextID.
// The server will input the prefix cached context and this turn of input into the model for processing.
// This improves efficiency by reducing token usage and request size.
//
// Parameters:
//   - ctx: The context for the request
//   - prefix: Initial messages to be cached as prefix context
//   - expireAtSec: Expiration time in seconds for the cached prefix
//
// Returns:
//   - info: Information about the created prefix cache, including the context ID and token usage
//   - err: Any error encountered during the operation
//
// ref: https://www.volcengine.com/docs/82379/1396490#_1-%E5%88%9B%E5%BB%BA%E5%89%8D%E7%BC%80%E7%BC%93%E5%AD%98
//
// Note:
//   - It is unavailable for doubao models of version 1.6 and above.
func (m *Model) CreatePrefixCache(ctx context.Context, prefix []*schema.AgenticMessage, expireAtSec *int64,
	opts ...agentic.Option) (info *CacheInfo, err error) {

	responseReq := &responses.ResponsesRequest{
		Model:    m.model,
		ExpireAt: expireAtSec,
		Store:    ptrOf(true),
		Caching: &responses.ResponsesCaching{
			Type:   responses.CacheType_enabled.Enum(),
			Prefix: ptrOf(true),
		},
	}

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	err = m.prePopulateConfig(responseReq, options, specOptions)
	if err != nil {
		return nil, err
	}

	err = m.populateInput(prefix, responseReq)
	if err != nil {
		return nil, err
	}

	err = m.populateTools(responseReq, options, specOptions)
	if err != nil {
		return nil, err
	}

	responseObj, err := m.cli.CreateResponses(ctx, responseReq)
	if err != nil {
		return nil, err
	}

	info = &CacheInfo{
		ResponseID: responseObj.Id,
		Usage:      *toTokenUsage(responseObj),
	}

	return info, nil
}

func (m *Model) toCallbackConfig(req *responses.ResponsesRequest) *agentic.Config {
	return &agentic.Config{
		Model:       req.Model,
		Temperature: float32(ptrFromOrZero(req.Temperature)),
		TopP:        float32(ptrFromOrZero(req.TopP)),
	}
}

func (m *Model) toFunctionTools(functionTools []*schema.ToolInfo) ([]*responses.ResponsesTool, error) {
	tools := make([]*responses.ResponsesTool, len(functionTools))
	for i := range functionTools {
		ti := functionTools[i]

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		b, err := sonic.Marshal(paramsJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSONSchema: %w", err)
		}

		tools[i] = &responses.ResponsesTool{
			Union: &responses.ResponsesTool_ToolFunction{
				ToolFunction: &responses.ToolFunction{
					Name:        ti.Name,
					Type:        responses.ToolType_function,
					Description: &ti.Desc,
					Parameters: &responses.Bytes{
						Value: b,
					},
				},
			},
		}
	}

	return tools, nil
}

func (m *Model) toServerTools(serverTools []*ServerToolConfig) (tools []*responses.ResponsesTool, rr error) {
	tools = make([]*responses.ResponsesTool, len(serverTools))

	for i := range serverTools {
		ti := serverTools[i]
		switch {
		case ti.WebSearch != nil:
			tools[i] = &responses.ResponsesTool{
				Union: &responses.ResponsesTool_ToolWebSearch{
					ToolWebSearch: ti.WebSearch,
				},
			}

		default:
			continue
		}
	}

	return tools, nil
}

func (m *Model) toMCPTools(mcpTools []*responses.ToolMcp) []*responses.ResponsesTool {
	tools := make([]*responses.ResponsesTool, len(mcpTools))
	for i := range mcpTools {
		tools[i] = &responses.ResponsesTool{
			Union: &responses.ResponsesTool_ToolMcp{
				ToolMcp: mcpTools[i],
			},
		}
	}
	return tools
}

func (m *Model) getOptions(opts []agentic.Option) (*agentic.Options, *arkOptions, error) {
	options := agentic.GetCommonOptions(&agentic.Options{
		Temperature: m.temperature,
		Model:       &m.model,
		TopP:        m.topP,
	}, opts...)

	arkOpts := agentic.GetImplSpecificOptions(&arkOptions{
		reasoning:         m.reasoning,
		thinking:          m.thinking,
		text:              m.text,
		maxToolCalls:      m.maxToolCalls,
		parallelToolCalls: m.parallelToolCalls,
		maxOutputTokens:   m.maxOutputTokens,
		serverTools:       m.serverTools,
		mcpTools:          m.mcpTools,
		customHeaders:     m.customHeader,
	}, opts...)

	return options, arkOpts, nil
}

func (m *Model) genRequestAndOptions(in []*schema.AgenticMessage, options *agentic.Options,
	specOptions *arkOptions) (responseReq *responses.ResponsesRequest, err error) {

	responseReq = &responses.ResponsesRequest{}

	err = m.prePopulateConfig(responseReq, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to prePopulateConfig: %w", err)
	}

	in, err = m.populateCache(in, responseReq, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to populateCache: %w", err)
	}

	err = m.populateInput(in, responseReq)
	if err != nil {
		return nil, fmt.Errorf("failed to populateInput: %w", err)
	}

	err = m.populateTools(responseReq, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to populateTools: %w", err)
	}

	return responseReq, nil
}

func (m *Model) prePopulateConfig(responseReq *responses.ResponsesRequest, options *agentic.Options,
	specOptions *arkOptions) error {

	// instance configuration
	responseReq.ServiceTier = m.serviceTier

	// options configuration
	responseReq.TopP = options.TopP
	responseReq.Temperature = options.Temperature
	if options.Model != nil {
		responseReq.Model = *options.Model
	}

	// specific options configuration
	responseReq.Thinking = specOptions.thinking
	responseReq.Reasoning = specOptions.reasoning
	responseReq.Text = specOptions.text
	responseReq.MaxOutputTokens = specOptions.maxOutputTokens
	responseReq.MaxToolCalls = specOptions.maxToolCalls
	responseReq.ParallelToolCalls = specOptions.parallelToolCalls

	return nil
}

func (m *Model) populateCache(in []*schema.AgenticMessage, responseReq *responses.ResponsesRequest,
	arkOpts *arkOptions) ([]*schema.AgenticMessage, error) {

	var (
		store       = false
		enableCache = false
		expireAtSec *int64
		headRespID  *string
	)

	if m.cache != nil {
		if sCache := m.cache.SessionCache; sCache != nil {
			if sCache.EnableCache {
				store = true
				enableCache = true
			}
			expireAtSec = &sCache.ExpireAtSec
		}
	}

	if cacheOpt := arkOpts.cache; cacheOpt != nil {
		headRespID = cacheOpt.HeadPreviousResponseID

		if sCacheOpt := cacheOpt.SessionCache; sCacheOpt != nil {
			expireAtSec = &sCacheOpt.ExpireAtSec

			if sCacheOpt.EnableCache {
				store = true
				enableCache = true
			} else {
				store = false
				enableCache = false
			}
		}
	}

	var (
		preRespID *string
		inputIdx  int
	)

	now := time.Now().Unix()

	// If the user implements session caching with ContextID,
	// ContextID and ResponseID will exist at the same time.
	// Using ContextID is prioritized to maintain compatibility with the old logic.
	// In this usage scenario, ResponseID cannot be used.
	if enableCache {
		for i := len(in) - 1; i >= 0; i-- {
			msg := in[i]
			if msg.ResponseMeta == nil {
				continue
			}

			extensions := getResponseMeta(msg.ResponseMeta)
			if extensions == nil || ptrFromOrZero(extensions.ExpireAt) <= now {
				continue
			}

			inputIdx = i
			preRespID = &extensions.ID

			break
		}
	}

	if preRespID != nil {
		if inputIdx+1 >= len(in) {
			return in, fmt.Errorf("not found incremental input after ResponseID")
		}
		in = in[inputIdx+1:]
	}

	// ResponseID has a higher priority than HeadPreviousResponseID
	if preRespID == nil {
		preRespID = headRespID
	}

	responseReq.PreviousResponseId = preRespID
	responseReq.Store = &store

	if expireAtSec != nil {
		responseReq.ExpireAt = expireAtSec
	}

	responseReq.Caching = &responses.ResponsesCaching{
		Type: func() *responses.CacheType_Enum {
			if enableCache {
				return responses.CacheType_enabled.Enum()
			}
			return responses.CacheType_disabled.Enum()
		}(),
	}

	return in, nil
}

func (m *Model) populateInput(in []*schema.AgenticMessage, responseReq *responses.ResponsesRequest) (err error) {
	if len(in) == 0 {
		return nil
	}

	itemList := make([]*responses.InputItem, 0, len(in))

	for _, msg := range in {
		var inputItems []*responses.InputItem

		switch msg.Role {
		case schema.AgenticRoleTypeUser:
			inputItems, err = toUserRoleInputItems(msg)
			if err != nil {
				return err
			}

		case schema.AgenticRoleTypeAssistant:
			inputItems, err = toAssistantRoleInputItems(msg)
			if err != nil {
				return err
			}

		case schema.AgenticRoleTypeDeveloper:
			inputItems, err = toDeveloperRoleInputItems(msg)
			if err != nil {
				return err
			}

		case schema.AgenticRoleTypeSystem:
			inputItems, err = toSystemRoleInputItems(msg)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid role: %s", msg.Role)
		}

		itemList = append(itemList, inputItems...)
	}

	responseReq.Input = &responses.ResponsesInput{
		Union: &responses.ResponsesInput_ListValue{
			ListValue: &responses.InputItemList{
				ListValue: itemList,
			},
		},
	}

	return nil
}

func (m *Model) populateTools(responseReq *responses.ResponsesRequest, options *agentic.Options, specOptions *arkOptions) (err error) {
	if responseReq.PreviousResponseId != nil {
		return nil
	}

	var functionTools []*responses.ResponsesTool
	if options.Tools != nil {
		functionTools, err = m.toFunctionTools(options.Tools)
		if err != nil {
			return err
		}
	} else {
		functionTools = m.functionTools
	}

	responseReq.Tools = append(responseReq.Tools, functionTools...)

	if options.ToolChoice != nil {
		responseReq.ToolChoice, err = toCommonToolChoice(*options.ToolChoice)
		if err != nil {
			return err
		}
	}

	serverTools, err := m.toServerTools(specOptions.serverTools)
	if err != nil {
		return err
	}

	responseReq.Tools = append(responseReq.Tools, serverTools...)

	if specOptions.forcedServerTool != nil {
		if responseReq.ToolChoice != nil {
			return fmt.Errorf("cannot specify multiple tool choice configurations simultaneously")
		}
		responseReq.ToolChoice, err = toServerToolChoice(specOptions.forcedServerTool)
		if err != nil {
			return err
		}
	}

	mcpTools := m.toMCPTools(specOptions.mcpTools)

	responseReq.Tools = append(responseReq.Tools, mcpTools...)

	if specOptions.forcedMCPTool != nil {
		if responseReq.ToolChoice != nil {
			return fmt.Errorf("cannot specify multiple tool choice configurations simultaneously")
		}
		responseReq.ToolChoice = toMCPToolChoice(specOptions.forcedMCPTool)
	}

	return nil
}

func toCommonToolChoice(toolChoice schema.ToolChoice) (*responses.ResponsesToolChoice, error) {
	var mode responses.ToolChoiceMode_Enum
	switch toolChoice {
	case schema.ToolChoiceForbidden:
		mode = responses.ToolChoiceMode_none
	case schema.ToolChoiceAllowed:
		mode = responses.ToolChoiceMode_auto
	case schema.ToolChoiceForced:
		mode = responses.ToolChoiceMode_required
	default:
		return nil, fmt.Errorf("invalid tool choice mode: %s", toolChoice)
	}
	return &responses.ResponsesToolChoice{
		Union: &responses.ResponsesToolChoice_Mode{
			Mode: mode,
		},
	}, nil
}

func toServerToolChoice(toolChoice *ForcedServerTool) (*responses.ResponsesToolChoice, error) {
	if toolChoice == nil {
		return nil, nil
	}
	switch {
	case toolChoice.WebSearch != nil:
		return &responses.ResponsesToolChoice{
			Union: &responses.ResponsesToolChoice_WebSearchToolChoice{
				WebSearchToolChoice: toolChoice.WebSearch,
			},
		}, nil
	default:
		return nil, fmt.Errorf("no valid server tool configuration found")
	}
}

func toMCPToolChoice(toolChoice *responses.McpToolChoice) *responses.ResponsesToolChoice {
	if toolChoice == nil {
		return nil
	}
	return &responses.ResponsesToolChoice{
		Union: &responses.ResponsesToolChoice_McpToolChoice{
			McpToolChoice: toolChoice,
		},
	}
}
