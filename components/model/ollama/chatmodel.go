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

package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"

	"github.com/ollama/ollama/api"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)
var CallbackMetricsExtraKey = "ollama_metrics"

// ChatModelConfig stores configuration options specific to Ollama
type ChatModelConfig struct {
	BaseURL string        `json:"base_url"`
	Timeout time.Duration `json:"timeout"` // request timeout for http client

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	Model     string          `json:"model"`
	Format    json.RawMessage `json:"format"`
	KeepAlive *time.Duration  `json:"keep_alive"`

	Options *api.Options `json:"options"`

	Thinking *api.ThinkValue `json:"thinking"`
}

// Check if ChatModel implements model.ChatModel
var _ model.ChatModel = (*ChatModel)(nil)

// ChatModel implements the model.ChatModel interface using Ollama's API.
type ChatModel struct {
	cli    *api.Client
	config *ChatModelConfig

	tools []*schema.ToolInfo
}

// NewChatModel initializes a new instance of ChatModel with provided configuration.
func NewChatModel(ctx context.Context, config *ChatModelConfig) (*ChatModel, error) {
	if config == nil {
		return nil, errors.New("config must not be nil")
	}

	var httpClient *http.Client

	if config.HTTPClient != nil {
		httpClient = config.HTTPClient
	} else {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	cli := api.NewClient(baseURL, httpClient)

	return &ChatModel{
		cli:    cli,
		config: config,

		tools: make([]*schema.ToolInfo, 0),
	}, nil
}
func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	var req *api.ChatRequest
	var cbInput *model.CallbackInput
	req, cbInput, err = cm.genRequest(ctx, false, input, opts...)
	if err != nil {
		return nil, fmt.Errorf("error generating request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	var cbOutput *model.CallbackOutput

	err = cm.cli.Chat(ctx, req, func(resp api.ChatResponse) error {
		outMsg = toEinoMessage(resp)
		cbOutput = &model.CallbackOutput{
			Message: outMsg,
			Config:  cbInput.Config,
			TokenUsage: &model.TokenUsage{
				PromptTokens:     resp.PromptEvalCount,
				CompletionTokens: resp.EvalCount,
				TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
			},
			Extra: map[string]any{
				CallbackMetricsExtraKey: resp.Metrics,
			},
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during Chat request: %w", err)
	}

	_ = callbacks.OnEnd(ctx, cbOutput)

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	var req *api.ChatRequest
	var cbInput *model.CallbackInput
	req, cbInput, err = cm.genRequest(ctx, true, input, opts...)
	if err != nil {
		return nil, fmt.Errorf("error generating request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func(ctx context.Context, conf *model.Config) {
		defer func() {
			panicErr := recover()

			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
		}()

		reqErr := cm.cli.Chat(ctx, req, func(resp api.ChatResponse) error {
			outMsg := toEinoMessage(resp)

			cbOutput := &model.CallbackOutput{
				// Notice: no token usage
				Message: outMsg,
				Config:  conf,
			}

			if resp.Done {
				cbOutput.Extra = map[string]any{
					CallbackMetricsExtraKey: resp.Metrics,
				}
			}

			sw.Send(cbOutput, nil)
			return nil
		})

		if reqErr != nil {
			sw.Send(nil, reqErr)
		}
	}(ctx, cbInput.Config)

	ctx, s := callbacks.OnEndWithStreamOutput(ctx, sr)

	outStream = schema.StreamReaderWithConvert(s,
		func(src *model.CallbackOutput) (*schema.Message, error) {
			if src.Message == nil {
				return nil, schema.ErrNoValue
			}

			return src.Message, nil
		})

	return outStream, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	ncm := *cm
	ncm.tools = tools
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return errors.New("no tools to bind")
	}
	cm.tools = tools
	return nil
}

func (cm *ChatModel) GetType() string {
	return "Ollama"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func (cm *ChatModel) genRequest(ctx context.Context, stream bool, in []*schema.Message, opts ...model.Option) (
	req *api.ChatRequest, cbInput *model.CallbackInput, err error) {

	var (
		o  = &options{}
		mo = &model.Options{
			Model: &cm.config.Model,
			Tools: cm.tools,
		}
	)
	if cm.config.Options != nil {
		mo.Temperature = &cm.config.Options.Temperature
		mo.TopP = &cm.config.Options.TopP
		mo.Stop = cm.config.Options.Stop
		o.Seed = &cm.config.Options.Seed
	}

	commonOptions := model.GetCommonOptions(mo, opts...)
	specificOptions := model.GetImplSpecificOptions(o, opts...)

	ollamaOptions := &api.Options{}
	conf := cm.config.Options
	if conf != nil {
		*ollamaOptions = *conf
	}

	if commonOptions.Temperature != nil {
		ollamaOptions.Temperature = *commonOptions.Temperature
	}
	if commonOptions.TopP != nil {
		ollamaOptions.TopP = *commonOptions.TopP
	}
	if len(commonOptions.Stop) > 0 {
		ollamaOptions.Stop = commonOptions.Stop
	}
	if specificOptions.Seed != nil {
		ollamaOptions.Seed = *specificOptions.Seed
	}

	reqOptions := make(map[string]any, 5)
	optBytes, err := json.Marshal(ollamaOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshal options: %w", err)
	}
	err = json.Unmarshal(optBytes, &reqOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshal options: %w", err)
	}

	msgs, err := toOllamaMessages(in)
	if err != nil {
		return nil, nil, fmt.Errorf("error convert messages: %w", err)
	}

	tools, err := toOllamaTools(mo.Tools)
	if err != nil {
		return nil, nil, fmt.Errorf("error convert tools: %w", err)
	}

	req = &api.ChatRequest{
		Model:    *commonOptions.Model,
		Messages: msgs,
		Stream:   ptrOf(stream),
		Format:   cm.config.Format,

		Tools: tools,

		Options: reqOptions,
		Think:   cm.config.Thinking,
	}

	if cm.config.KeepAlive != nil {
		req.KeepAlive = &api.Duration{Duration: *cm.config.KeepAlive}
	}

	cbInput = &model.CallbackInput{
		Messages: in,
		Tools:    commonOptions.Tools,
		Config: &model.Config{
			Model:       req.Model,
			Temperature: ollamaOptions.Temperature,
			TopP:        ollamaOptions.TopP,
			Stop:        ollamaOptions.Stop,
		},
	}

	return req, cbInput, nil
}

func toOllamaMessages(messages []*schema.Message) ([]api.Message, error) {
	var ollamaMessages []api.Message
	for _, msg := range messages {
		ollamaMsg, err := toOllamaMessage(msg)
		if err != nil {
			return nil, err
		}

		ollamaMessages = append(ollamaMessages, ollamaMsg)
	}
	return ollamaMessages, nil
}

func toOllamaMessage(einoMsg *schema.Message) (api.Message, error) {
	var toolCalls []api.ToolCall
	for _, toolCall := range einoMsg.ToolCalls {
		args, err := parseJSONToObject(toolCall.Function.Arguments)
		if err != nil {
			return api.Message{}, fmt.Errorf("error parsing JSON to object: %w", err)
		}

		toolCalls = append(toolCalls, api.ToolCall{
			Function: api.ToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: args,
			},
		})
	}
	om := api.Message{
		Role:      string(einoMsg.Role),
		Thinking:  einoMsg.ReasoningContent,
		ToolCalls: toolCalls,
	}
	if len(einoMsg.MultiContent) == 0 {
		om.Content = einoMsg.Content
		return om, nil
	}

	content := ""
	var images []api.ImageData
	for _, mc := range einoMsg.MultiContent {
		switch mc.Type {
		case schema.ChatMessagePartTypeText:
			content += mc.Text
		case schema.ChatMessagePartTypeImageURL:
			if mc.ImageURL == nil {
				return api.Message{}, errors.New("image url is required")
			}
			images = append(images, api.ImageData(mc.ImageURL.URL))
		default:
			return api.Message{}, fmt.Errorf("unsupported content type: %s", mc.Type)
		}
	}

	return api.Message{
		Role:      string(einoMsg.Role),
		Content:   content,
		Images:    images,
		Thinking:  einoMsg.ReasoningContent,
		ToolCalls: toolCalls,
	}, nil
}

func toEinoMessage(resp api.ChatResponse) *schema.Message {
	var toolCalls []schema.ToolCall
	for _, toolCall := range resp.Message.ToolCalls {
		arguments := toolCall.Function.Arguments.String()
		toolCalls = append(toolCalls, schema.ToolCall{
			Type: "function",
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: arguments,
			},
		})
	}

	// Notice: not support Images
	return &schema.Message{
		Role:             schema.RoleType(resp.Message.Role),
		Content:          resp.Message.Content,
		ReasoningContent: resp.Message.Thinking,
		ToolCalls:        toolCalls,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: resp.DoneReason,
			Usage: &schema.TokenUsage{
				PromptTokens:     resp.PromptEvalCount,
				CompletionTokens: resp.EvalCount,
				TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
			},
		},
	}
}

func parseJSONToObject(jsonStr string) (map[string]any, error) {
	result := make(map[string]interface{})

	err := json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

func toOllamaTools(einoTools []*schema.ToolInfo) ([]api.Tool, error) {
	var ollamaTools []api.Tool
	for _, einoTool := range einoTools {
		properties := make(map[string]api.ToolProperty)
		var required []string

		openTool, err := einoTool.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, err
		}

		if openTool != nil {
			required = openTool.Required

			for pair := openTool.Properties.Oldest(); pair != nil; pair = pair.Next() {
				var typ []string
				if pair.Value.TypeEnhanced != nil {
					typ = pair.Value.TypeEnhanced
				} else {
					typ = []string{pair.Value.Type}
				}
				properties[pair.Key] = api.ToolProperty{
					Type:        typ,
					Description: pair.Value.Description,
					Enum:        pair.Value.Enum,
				}
			}
		}

		ollamaTool := api.Tool{
			Type: "function", // Assuming default type
			Function: api.ToolFunction{
				Name:        einoTool.Name,
				Description: einoTool.Desc,
				Parameters: struct {
					Type       string                      `json:"type"`
					Defs       any                         `json:"$defs,omitempty"`
					Items      any                         `json:"items,omitempty"`
					Required   []string                    `json:"required"`
					Properties map[string]api.ToolProperty `json:"properties"`
				}{
					Type:       "object",
					Required:   required,
					Properties: properties,
				},
			},
		}
		ollamaTools = append(ollamaTools, ollamaTool)
	}
	return ollamaTools, nil
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
