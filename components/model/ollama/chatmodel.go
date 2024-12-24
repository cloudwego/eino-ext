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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/utils/safe"
)

var CallbackMetricsExtraKey = "ollama_metrics"

// ChatModelConfig stores configuration options specific to Ollama
type ChatModelConfig struct {
	BaseURL string        `json:"base_url"`
	Timeout time.Duration `json:"timeout"` // request timeout for http client

	Model     string         `json:"model"`
	Format    string         `json:"format"` // "json" or ""
	KeepAlive *time.Duration `json:"keep_alive"`

	Options *api.Options `json:"options"`
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

	httpClient := http.Client{
		Timeout: config.Timeout,
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	cli := api.NewClient(baseURL, &httpClient)

	return &ChatModel{
		cli:    cli,
		config: config,

		tools: make([]*schema.ToolInfo, 0),
	}, nil
}
func (cm *ChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	var req *api.ChatRequest
	var reqConf *model.Config
	req, reqConf, err = cm.genRequest(ctx, false, input, opts...)
	if err != nil {
		return nil, fmt.Errorf("error generating request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: input,
		Tools:    append([]*schema.ToolInfo{}, cm.tools...),
		Config:   reqConf,
	})

	cbOutput := &model.CallbackOutput{
		Message: outMsg,
		Config:  reqConf,
		Extra:   map[string]any{},
	}

	err = cm.cli.Chat(ctx, req, func(resp api.ChatResponse) error {
		outMsg = toEinoMessage(resp)
		cbOutput.Extra[CallbackMetricsExtraKey] = resp.Metrics
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during Chat request: %w", err)
	}

	_ = callbacks.OnEnd(ctx, cbOutput)

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	var req *api.ChatRequest
	var reqConf *model.Config
	req, reqConf, err = cm.genRequest(ctx, true, input, opts...)
	if err != nil {
		return nil, fmt.Errorf("error generating request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, &model.CallbackInput{
		Messages: append([]*schema.Message{}, input...),
		Tools:    append([]*schema.ToolInfo{}, cm.tools...),
		Config:   reqConf,
	})

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func(ctx context.Context, conf *model.Config) {
		defer func() {
			panicErr := recover()

			if panicErr != nil {
				_ = sw.Send(nil, safe.NewPanicErr(panicErr, debug.Stack()))
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
	}(ctx, reqConf)

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

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	cm.tools = tools
	return nil
}

func (cm *ChatModel) GetType() string {
	return "Ollama"
}

func (cm *ChatModel) IsCallbacksEnabled() bool {
	return true
}

func (cm *ChatModel) genRequest(ctx context.Context, stream bool, in []*schema.Message, opts ...model.Option) (req *api.ChatRequest, modelConfig *model.Config, err error) {
	commonOptions := model.GetCommonOptions(&model.Options{}, opts...)
	specificOptions := model.GetImplSpecificOptions(&options{}, opts...)

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

	modelName := cm.config.Model
	if commonOptions.Model != nil {
		modelName = *commonOptions.Model
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

	req = &api.ChatRequest{
		Model:    modelName,
		Messages: msgs,
		Stream:   ptrOf(stream),
		Format:   cm.config.Format,

		Tools: toOllamaTools(cm.tools),

		Options: reqOptions,
	}

	if cm.config.KeepAlive != nil {
		req.KeepAlive = &api.Duration{Duration: *cm.config.KeepAlive}
	}

	modelConfig = &model.Config{
		Model:       req.Model,
		Temperature: ollamaOptions.Temperature,
		TopP:        ollamaOptions.TopP,
		Stop:        ollamaOptions.Stop,
	}

	return req, modelConfig, nil
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
				Arguments: api.ToolCallFunctionArguments(args),
			},
		})
	}

	// Notice: not support ToolCallID, MultiContent
	return api.Message{
		Role:      string(einoMsg.Role),
		Content:   einoMsg.Content,
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
		Role:      schema.RoleType(resp.Message.Role),
		Content:   resp.Message.Content,
		ToolCalls: toolCalls,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: resp.DoneReason,
			Usage:        nil,
		},
	}
}

func parseJSONToObject(jsonStr string) (map[string]any, error) {
	result := make(map[string]interface{})

	err := json.Unmarshal([]byte(jsonStr), &result) // nolint: byted_json_accuracyloss_unknowstruct
	return result, err
}

func toOllamaTools(einoTools []*schema.ToolInfo) []api.Tool {
	var ollamaTools []api.Tool
	for _, einoTool := range einoTools {
		properties := make(map[string]struct {
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Enum        []string `json:"enum,omitempty"`
		})

		var required []string
		for name, param := range einoTool.ParamsOneOf.Params {
			properties[name] = struct {
				Type        string   `json:"type"`
				Description string   `json:"description"`
				Enum        []string `json:"enum,omitempty"`
			}{
				Type:        string(param.Type),
				Description: param.Desc,
				Enum:        param.Enum,
			}
			if param.Required {
				required = append(required, name)
			}
		}

		ollamaTool := api.Tool{
			Type: "function", // Assuming default type
			Function: api.ToolFunction{
				Name:        einoTool.Name,
				Description: einoTool.Desc,
				Parameters: struct {
					Type       string   `json:"type"`
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type        string   `json:"type"`
						Description string   `json:"description"`
						Enum        []string `json:"enum,omitempty"`
					} `json:"properties"`
				}{
					Type:       "object",
					Required:   required,
					Properties: properties,
				},
			},
		}
		ollamaTools = append(ollamaTools, ollamaTool)
	}
	return ollamaTools
}