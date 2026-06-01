/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticclaude

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.AgenticModel = (*Model)(nil)

type Config struct {
	// HTTPClient specifies the client to send HTTP requests.
	// It is not applied when using Google Vertex AI.
	// Optional.
	HTTPClient *http.Client

	// ByBedrock specifies the configuration for using AWS Bedrock.
	// Optional.
	ByBedrock *BedrockConfig

	// ByGoogleVertexAI specifies the configuration for using Google Vertex AI.
	// Optional.
	ByGoogleVertexAI *GoogleVertexAIConfig

	// BaseURL is the custom API endpoint URL
	// Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
	// Optional.
	BaseURL string

	// APIKey is your Anthropic API key
	// Obtain from: https://console.anthropic.com/account/keys
	// Required for direct Anthropic API requests.
	APIKey string

	// Model specifies which Claude model to use.
	// Required.
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Range: 1 to model's context length
	// Required.
	MaxTokens int

	// StopSequences specifies custom stop sequences
	// The model will stop generating when it encounters any of these sequences
	// Optional.
	StopSequences []string

	// DisableParallelToolUse specifies whether to disable parallel tool use.
	// It only takes effect when AgenticToolChoice is set.
	// Optional.
	DisableParallelToolUse *bool

	// Thinking specifies the configuration for Claude thinking mode.
	// Optional.
	Thinking *anthropic.ThinkingConfigParamUnion

	// CustomHeaders specifies custom HTTP headers to include in API requests.
	// CustomHeaders allows passing additional metadata or authentication information.
	// Optional.
	CustomHeaders map[string]string

	// ExtraFields specifies extra fields to include in the request body.
	// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
	// Optional.
	//
	// Example:
	//
	//	ExtraFields: map[string]any{
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default",
	//	}
	//
	// The resulting request body will be:
	//
	//	{
	//	    "model": "o1",
	//	    "messages": [...],
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default"
	//	}
	ExtraFields map[string]any

	// CacheControl configures automatic prompt caching behavior.
	// When non-nil, automatically applies a cache_control marker to the last
	// cacheable block in the request.
	// Optional.
	CacheControl *anthropic.CacheControlEphemeralParam
}

type GoogleVertexAIConfig struct {
	// ProjectID is your Google Cloud project ID.
	// Required for Google Vertex AI requests. If not set, automatically detected
	// from ANTHROPIC_VERTEX_PROJECT_ID, GOOGLE_CLOUD_PROJECT, or GCLOUD_PROJECT
	// environment variables.
	ProjectID string

	// Region is the Vertex AI region (e.g., "us-east5").
	// Required for Google Vertex AI requests. If not set, automatically detected
	// from CLOUD_ML_REGION environment variable.
	// See: https://claude.ai/docs/en/google-vertex-ai
	Region string
}

type BedrockConfig struct {
	// AccessKey is your Bedrock API Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional.
	AccessKey string

	// SecretAccessKey is your Bedrock API Secret Access key
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional.
	SecretAccessKey string

	// SessionToken is your Bedrock API Session Token
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional.
	SessionToken string

	// Profile is your Bedrock API AWS profile
	// This parameter is ignored if AccessKey and SecretAccessKey are provided
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional.
	Profile string

	// Region is your Bedrock API region
	// Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
	// Optional.
	Region string
}

type Model struct {
	cli anthropic.Client

	model                  string
	maxTokens              int
	stopSequences          []string
	disableParallelToolUse *bool
	customHeaders          map[string]string
	extraFields            map[string]any
	thinking               *anthropic.ThinkingConfigParamUnion
	cacheControl           *anthropic.CacheControlEphemeralParam
}

type ServerToolConfig struct {
	// WebSearch20260209 specifies the web search server tool with 20260209 version.
	WebSearch20260209 *anthropic.WebSearchTool20260209Param
	// WebFetch20260309 specifies the web fetch server tool with 20260309 version.
	WebFetch20260309 *anthropic.WebFetchTool20260309Param
	// CodeExecution20260120 specifies the code execution server tool with 20260120 version.
	// This single request-side tool enables the runtime server tool names
	// "code_execution", "bash_code_execution", and "text_editor_code_execution".
	CodeExecution20260120 *anthropic.CodeExecutionTool20260120Param
	// ToolSearchToolBm25_20251119 specifies the BM25 tool search server tool with
	// 20251119 version.
	ToolSearchToolBm25_20251119 *anthropic.ToolSearchToolBm25_20251119Param
	// ToolSearchToolRegex20251119 specifies the regex tool search server tool with
	// 20251119 version.
	ToolSearchToolRegex20251119 *anthropic.ToolSearchToolRegex20251119Param
}

func (cfg *Config) check() error {
	if cfg.Model == "" {
		return errors.New("model is required")
	}
	if cfg.ByBedrock == nil && cfg.ByGoogleVertexAI == nil && cfg.APIKey == "" {
		return errors.New("api key is required for direct Anthropic API requests")
	}
	if cfg.MaxTokens <= 0 {
		return errors.New("max_tokens must be positive")
	}
	return nil
}

func New(ctx context.Context, cfg *Config) (*Model, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}

	cli, err := newClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Model{
		cli:                    cli,
		model:                  cfg.Model,
		maxTokens:              cfg.MaxTokens,
		stopSequences:          cfg.StopSequences,
		disableParallelToolUse: cfg.DisableParallelToolUse,
		customHeaders:          cfg.CustomHeaders,
		extraFields:            cfg.ExtraFields,
		thinking:               cfg.Thinking,
		cacheControl:           cfg.CacheControl,
	}, nil
}

func newClient(ctx context.Context, cfg *Config) (anthropic.Client, error) {
	switch {
	case cfg.ByGoogleVertexAI != nil:
		projectID := cfg.ByGoogleVertexAI.ProjectID
		if projectID == "" {
			projectID = getEnvWithFallbacks("ANTHROPIC_VERTEX_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT")
		}
		if projectID == "" {
			return anthropic.Client{}, errors.New("ByGoogleVertexAI is set but no project ID provided; set ProjectID or ANTHROPIC_VERTEX_PROJECT_ID")
		}

		region := cfg.ByGoogleVertexAI.Region
		if region == "" {
			region = os.Getenv("CLOUD_ML_REGION")
		}
		if region == "" {
			return anthropic.Client{}, errors.New("ByGoogleVertexAI is set but no region provided; set Region or CLOUD_ML_REGION")
		}

		return anthropic.NewClient(vertex.WithGoogleAuth(ctx, region, projectID)), nil

	case cfg.ByBedrock != nil:
		var opts []func(*config.LoadOptions) error
		if cfg.ByBedrock.Region != "" {
			opts = append(opts, config.WithRegion(cfg.ByBedrock.Region))
		}
		if cfg.ByBedrock.AccessKey != "" && cfg.ByBedrock.SecretAccessKey != "" {
			opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.ByBedrock.AccessKey,
				cfg.ByBedrock.SecretAccessKey,
				cfg.ByBedrock.SessionToken,
			)))
		} else if cfg.ByBedrock.Profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(cfg.ByBedrock.Profile))
		}
		if cfg.HTTPClient != nil {
			opts = append(opts, config.WithHTTPClient(cfg.HTTPClient))
		}

		return anthropic.NewClient(bedrock.WithLoadDefaultConfig(ctx, opts...)), nil

	default:
		var opts []option.RequestOption
		if cfg.APIKey != "" {
			opts = append(opts, option.WithAPIKey(cfg.APIKey))
		}
		if cfg.BaseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.BaseURL))
		}
		if cfg.HTTPClient != nil {
			opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
		}
		return anthropic.NewClient(opts...), nil
	}
}

func (m *Model) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (outMsg *schema.AgenticMessage, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("generate request failed: %w", err)
	}

	callbackConfig := toCallbackConfig(req)

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    options.Tools,
		Config:   callbackConfig,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := m.cli.Messages.New(ctx, *req, reqOpts...)
	if err != nil {
		return nil, fmt.Errorf("create message failed: %w", err)
	}

	outMsg, err = toAgenticMessage(resp)
	if err != nil {
		return nil, fmt.Errorf("convert response to agentic message failed: %w", err)
	}

	callbacks.OnEnd(ctx, &model.AgenticCallbackOutput{
		Message:    outMsg,
		Config:     callbackConfig,
		TokenUsage: toModelTokenUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (m *Model) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (outStream *schema.StreamReader[*schema.AgenticMessage], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("generate request failed: %w", err)
	}

	callbackConfig := toCallbackConfig(req)

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    options.Tools,
		Config:   callbackConfig,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	stream := m.cli.Messages.NewStreaming(ctx, *req, reqOpts...)
	if stream.Err() != nil {
		return nil, fmt.Errorf("create message stream failed: %w", stream.Err())
	}

	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](1)
	go func() {
		defer func() {
			if pe := recover(); pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}
			_ = stream.Close()
			sw.Close()
		}()

		converter := newStreamConverter()
		for stream.Next() {
			chunk, chunkErr := converter.toMessageStreamingChunk(stream.Current())
			if chunkErr != nil {
				_ = sw.Send(nil, fmt.Errorf("convert stream event to agentic message failed: %w", chunkErr))
				return
			}
			if chunk == nil {
				continue
			}

			closed := sw.Send(&model.AgenticCallbackOutput{
				Message: chunk,
				Config:  callbackConfig,
			}, nil)
			if closed {
				return
			}
		}

		if stream.Err() != nil {
			_ = sw.Send(nil, stream.Err())
		}
	}()

	_, callbackStream := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.AgenticCallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		},
	))

	outStream = schema.StreamReaderWithConvert(callbackStream, func(src callbacks.CallbackOutput) (*schema.AgenticMessage, error) {
		output := src.(*model.AgenticCallbackOutput)
		if output.Message == nil {
			return nil, schema.ErrNoValue
		}
		return output.Message, nil
	})

	return outStream, nil
}

func (m *Model) GetType() string {
	return implType
}

func (m *Model) IsCallbacksEnabled() bool {
	return true
}

func (m *Model) genRequestAndOptions(input []*schema.AgenticMessage, options *model.Options,
	specOptions *claudeOptions) (req *anthropic.MessageNewParams, reqOpts []option.RequestOption, err error) {

	req = &anthropic.MessageNewParams{}
	if options.Model != nil {
		req.Model = *options.Model
	}
	if options.MaxTokens != nil {
		req.MaxTokens = int64(*options.MaxTokens)
	}
	if options.Temperature != nil {
		req.Temperature = param.NewOpt(float64(*options.Temperature))
	}
	if options.TopP != nil {
		req.TopP = param.NewOpt(float64(*options.TopP))
	}
	if len(options.Stop) > 0 {
		req.StopSequences = options.Stop
	}
	if m.thinking != nil {
		req.Thinking = *m.thinking
	}

	system, messages, err := toAnthropicMessages(input)
	if err != nil {
		return nil, nil, fmt.Errorf("convert input messages failed: %w", err)
	}
	req.System = system
	req.Messages = messages

	err = m.populateTools(req, options, specOptions)
	if err != nil {
		return nil, nil, err
	}

	if err = m.populateToolChoice(req, options); err != nil {
		return nil, nil, err
	}

	if m.cacheControl != nil {
		req.CacheControl = *m.cacheControl
	}

	reqOpts = appendCustomHeaders(reqOpts, specOptions.serverTools, specOptions.customHeaders)
	for k, v := range specOptions.extraFields {
		reqOpts = append(reqOpts, option.WithJSONSet(k, v))
	}

	return req, reqOpts, nil
}

func appendCustomHeaders(reqOpts []option.RequestOption, serverTools []*ServerToolConfig, customHeaders map[string]string) []option.RequestOption {
	// non-beta headers
	for k, v := range customHeaders {
		if k != headerAnthropicBeta {
			reqOpts = append(reqOpts, option.WithHeaderAdd(k, v))
		}
	}

	// beta headers: merge server tool betas and custom betas, custom takes precedence
	serverToolBetas := collectServerToolBetaHeaders(serverTools)
	customBetas := splitHeaderValues(customHeaders[headerAnthropicBeta])
	for _, beta := range serverToolBetas {
		customBetas[beta] = struct{}{}
	}
	for beta := range customBetas {
		reqOpts = append(reqOpts, option.WithHeaderAdd(headerAnthropicBeta, beta))
	}

	return reqOpts
}

func collectServerToolBetaHeaders(serverTools []*ServerToolConfig) []string {
	selectedTools := selectServerTools(serverTools)
	betas := make([]string, 0, len(selectedTools))
	for _, tool := range selectedTools {
		switch tool {
		case serverToolVersionWebFetch20260309:
			betas = append(betas, betaHeaderWebFetch20260309)
		}
	}
	return betas
}

func splitHeaderValues(header string) map[string]struct{} {
	values := make(map[string]struct{})
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values[part] = struct{}{}
	}
	return values
}

func (m *Model) getOptions(opts []model.Option) (*model.Options, *claudeOptions, error) {
	options := model.GetCommonOptions(&model.Options{
		Model:     &m.model,
		MaxTokens: &m.maxTokens,
		Stop:      m.stopSequences,
	}, opts...)
	specOptions := model.GetImplSpecificOptions(&claudeOptions{
		customHeaders: m.customHeaders,
		extraFields:   m.extraFields,
	}, opts...)

	if options.ToolChoice != nil {
		return nil, nil, fmt.Errorf("'ToolChoice' option is not supported, please use 'AgenticToolChoice'")
	}
	if len(options.AllowedToolNames) > 0 {
		return nil, nil, fmt.Errorf("'AllowedToolNames' option is not supported by AgenticClaude")
	}

	return options, specOptions, nil
}

func (m *Model) populateTools(req *anthropic.MessageNewParams, options *model.Options, specOptions *claudeOptions) error {
	var tools []anthropic.ToolUnionParam

	if options.Tools != nil {
		functionTools, err := toFunctionTools(options.Tools)
		if err != nil {
			return fmt.Errorf("convert call-time function tools failed: %w", err)
		}
		tools = append(tools, functionTools...)
	}

	if options.DeferredTools != nil {
		deferredTools, err := toDeferredFunctionTools(options.DeferredTools)
		if err != nil {
			return fmt.Errorf("convert call-time deferred tools failed: %w", err)
		}
		tools = append(tools, deferredTools...)
	}

	if options.ToolSearchTool != nil {
		toolSearchTools, err := toFunctionTools([]*schema.ToolInfo{options.ToolSearchTool})
		if err != nil {
			return fmt.Errorf("convert call-time tool search tool failed: %w", err)
		}
		tools = append(tools, toolSearchTools...)
	}

	serverTools, err := toServerTools(specOptions.serverTools)
	if err != nil {
		return err
	}
	tools = append(tools, serverTools...)

	req.Tools = tools
	return nil
}

func (m *Model) populateToolChoice(req *anthropic.MessageNewParams, options *model.Options) error {
	if options.AgenticToolChoice == nil {
		return nil
	}

	toolChoice := options.AgenticToolChoice

	allowToolNames, err := getAllowedToolNames(toolChoice)
	if err != nil {
		return err
	}

	switch toolChoice.Type {
	case schema.ToolChoiceForbidden:
		ofNone := anthropic.NewToolChoiceNoneParam()
		req.ToolChoice = anthropic.ToolChoiceUnionParam{OfNone: &ofNone}
		return nil

	case schema.ToolChoiceAllowed:
		if len(allowToolNames) > 0 {
			return fmt.Errorf("tool choice 'allowed' with specific tools is not supported")
		}
		req.ToolChoice = anthropic.ToolChoiceUnionParam{
			OfAuto: &anthropic.ToolChoiceAutoParam{
				DisableParallelToolUse: newClaudeOpt(m.disableParallelToolUse),
			},
		}
		return nil

	case schema.ToolChoiceForced:
		if len(req.Tools) == 0 {
			return fmt.Errorf("tool choice is forced but no tools are provided")
		}
		if len(allowToolNames) > 1 {
			return fmt.Errorf("tool choice 'forced' with multiple specific tools is not supported")
		}

		var forcedToolName string
		if len(allowToolNames) == 1 {
			forcedToolName = allowToolNames[0]
		} else if len(allowToolNames) == 0 && len(req.Tools) == 1 {
			forcedToolName = getToolName(req.Tools[0])
		}

		if forcedToolName != "" {
			if !supportsForcedTool(req.Tools, forcedToolName) {
				return fmt.Errorf("forced tool %q is not found in the provided tools", forcedToolName)
			}
			req.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfTool: &anthropic.ToolChoiceToolParam{
					Name:                   forcedToolName,
					DisableParallelToolUse: newClaudeOpt(m.disableParallelToolUse),
				},
			}
			return nil
		}

		req.ToolChoice = anthropic.ToolChoiceUnionParam{
			OfAny: &anthropic.ToolChoiceAnyParam{
				DisableParallelToolUse: newClaudeOpt(m.disableParallelToolUse),
			},
		}

		return nil

	default:
		return fmt.Errorf("invalid tool choice: %s", toolChoice.Type)
	}
}

func supportsForcedTool(tools []anthropic.ToolUnionParam, forcedToolName string) bool {
	for _, tool := range tools {
		if supportsForcedToolName(tool, forcedToolName) {
			return true
		}
	}
	return false
}

func getToolName(tool anthropic.ToolUnionParam) string {
	if n := tool.GetName(); n != nil && *n != "" {
		return *n
	}

	switch {
	case tool.OfWebSearchTool20250305 != nil, tool.OfWebSearchTool20260209 != nil:
		return string(ServerToolNameWebSearch)
	case tool.OfWebFetchTool20250910 != nil, tool.OfWebFetchTool20260209 != nil, tool.OfWebFetchTool20260309 != nil:
		return string(ServerToolNameWebFetch)
	case tool.OfCodeExecutionTool20250522 != nil, tool.OfCodeExecutionTool20250825 != nil, tool.OfCodeExecutionTool20260120 != nil:
		return string(ServerToolNameCodeExecution)
	case tool.OfToolSearchToolBm25_20251119 != nil:
		return string(ServerToolNameToolSearchToolBm25)
	case tool.OfToolSearchToolRegex20251119 != nil:
		return string(ServerToolNameToolSearchToolRegex)
	default:
		return ""
	}
}

func supportsForcedToolName(tool anthropic.ToolUnionParam, forcedToolName string) bool {
	if getToolName(tool) == forcedToolName {
		return true
	}

	if forcedToolName != string(ServerToolNameBashCodeExecution) &&
		forcedToolName != string(ServerToolNameTextEditorCodeExecution) {
		return false
	}

	return getToolName(tool) == string(ServerToolNameCodeExecution)
}

func getAllowedToolNames(toolChoice *schema.AgenticToolChoice) ([]string, error) {
	var allowedTools []*schema.AllowedTool
	switch toolChoice.Type {
	case schema.ToolChoiceAllowed:
		if toolChoice.Allowed != nil {
			allowedTools = toolChoice.Allowed.Tools
		}
	case schema.ToolChoiceForced:
		if toolChoice.Forced != nil {
			allowedTools = toolChoice.Forced.Tools
		}
	}

	allowToolNames := make([]string, 0, len(allowedTools))
	seen := make(map[string]struct{}, len(allowedTools))
	for _, tool := range allowedTools {
		if tool.ServerTool != nil {
			if tool.ServerTool.Name == "" {
				return nil, fmt.Errorf("server tool name is empty")
			}
			if _, ok := seen[tool.ServerTool.Name]; ok {
				continue
			}
			seen[tool.ServerTool.Name] = struct{}{}
			allowToolNames = append(allowToolNames, tool.ServerTool.Name)
			continue
		}
		if tool.MCPTool != nil {
			return nil, fmt.Errorf("mcp tools are not supported yet")
		}
		if tool.FunctionName == "" {
			return nil, fmt.Errorf("allowed function tool name is empty")
		}
		if _, ok := seen[tool.FunctionName]; ok {
			continue
		}
		seen[tool.FunctionName] = struct{}{}
		allowToolNames = append(allowToolNames, tool.FunctionName)
	}

	return allowToolNames, nil
}

func toServerTools(serverTools []*ServerToolConfig) ([]anthropic.ToolUnionParam, error) {
	selectedTools := selectServerTools(serverTools)
	tools := make([]anthropic.ToolUnionParam, 0, len(selectedTools))
	for _, selected := range selectedTools {
		tool, ok := getSelectedServerToolParam(selected, serverTools)
		if !ok {
			continue
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func selectServerTools(serverTools []*ServerToolConfig) []string {
	var (
		webSearch       string
		webFetch        string
		codeExecution   string
		toolSearchBm25  string
		toolSearchRegex string
	)
	for _, tool := range serverTools {
		if tool.WebSearch20260209 != nil {
			webSearch = serverToolVersionWebSearch20260209
		}
		if tool.WebFetch20260309 != nil {
			webFetch = serverToolVersionWebFetch20260309
		}
		if tool.CodeExecution20260120 != nil {
			codeExecution = serverToolVersionCodeExecution20260120
		}
		if tool.ToolSearchToolBm25_20251119 != nil {
			toolSearchBm25 = serverToolVersionToolSearchToolBm25_20251119
		}
		if tool.ToolSearchToolRegex20251119 != nil {
			toolSearchRegex = serverToolVersionToolSearchToolRegex20251119
		}
	}

	selected := make([]string, 0, 5)
	if webSearch != "" {
		selected = append(selected, webSearch)
	}
	if webFetch != "" {
		selected = append(selected, webFetch)
	}
	if codeExecution != "" {
		selected = append(selected, codeExecution)
	}
	if toolSearchBm25 != "" {
		selected = append(selected, toolSearchBm25)
	}
	if toolSearchRegex != "" {
		selected = append(selected, toolSearchRegex)
	}

	return selected
}

func getSelectedServerToolParam(selected string, serverTools []*ServerToolConfig) (anthropic.ToolUnionParam, bool) {
	for i := len(serverTools) - 1; i >= 0; i-- {
		tool := serverTools[i]
		switch selected {
		case serverToolVersionWebSearch20260209:
			if tool.WebSearch20260209 == nil {
				continue
			}
			return anthropic.ToolUnionParam{OfWebSearchTool20260209: tool.WebSearch20260209}, true

		case serverToolVersionWebFetch20260309:
			if tool.WebFetch20260309 == nil {
				continue
			}
			return anthropic.ToolUnionParam{OfWebFetchTool20260309: tool.WebFetch20260309}, true

		case serverToolVersionCodeExecution20260120:
			if tool.CodeExecution20260120 == nil {
				continue
			}
			return anthropic.ToolUnionParam{OfCodeExecutionTool20260120: tool.CodeExecution20260120}, true

		case serverToolVersionToolSearchToolBm25_20251119:
			if tool.ToolSearchToolBm25_20251119 == nil {
				continue
			}
			return anthropic.ToolUnionParam{OfToolSearchToolBm25_20251119: tool.ToolSearchToolBm25_20251119}, true

		case serverToolVersionToolSearchToolRegex20251119:
			if tool.ToolSearchToolRegex20251119 == nil {
				continue
			}
			return anthropic.ToolUnionParam{OfToolSearchToolRegex20251119: tool.ToolSearchToolRegex20251119}, true
		}
	}
	return anthropic.ToolUnionParam{}, false
}

func toCallbackConfig(req *anthropic.MessageNewParams) *model.AgenticConfig {
	config := &model.AgenticConfig{
		Model:     string(req.Model),
		MaxTokens: int(req.MaxTokens),
	}
	if req.Temperature.Valid() {
		config.Temperature = float32(req.Temperature.Value)
	}
	if req.TopP.Valid() {
		config.TopP = float32(req.TopP.Value)
	}
	return config
}

func toModelTokenUsage(meta *schema.AgenticResponseMeta) *model.TokenUsage {
	if meta == nil || meta.TokenUsage == nil {
		return nil
	}
	usage := meta.TokenUsage
	return &model.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
		TotalTokens: usage.TotalTokens,
	}
}
