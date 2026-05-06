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

package hunyuan

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.ToolCallingChatModel = (*ChatModel)(nil)

const (
	roleAssistant = "assistant"
	roleSystem    = "system"
	roleUser      = "user"
	roleTool      = "tool"
)

const (
	contentTypeText       = "text"
	contentTypeImage      = "image_url"
	contentTypeVideoURL   = "video_url"
	contentTypeVideoFrame = "video_frames"
)

const (
	typ = "Hunyuan"
)

var (
	toolChoiceNone     = "none"
	toolChoiceAuto     = "auto"
	toolChoiceRequired = "custom"
)

var (
	defaultRegion = regions.Guangzhou
)

// ChatModelConfig contains configuration options for creating a Hunyuan model.
type ChatModelConfig struct {
	// SecretId for Hunyuan API authentication.
	SecretId string
	// SecretKey for Hunyuan API authentication.
	SecretKey string
	// Region specifies the region where Hunyuan service is located
	Region string
	// Model specifies the ID of the model to use
	// Required
	Model string
	// Temperature specifies what sampling temperature to use
	// Generally recommend altering this or TopP but not both.
	// Range: [0.0, 2.0]. Higher values make output more random
	// Optional. Default: 1.0
	Temperature float32
	// TopP controls diversity via nucleus sampling
	// Generally recommend altering this or Temperature but not both.
	// Range: [0.0, 1.0]. Lower values make output more focused
	// Optional. Default: 1.0
	TopP float32
	// Stop sequences where the API will stop generating further tokens
	// Optional. Example: []string{"\n", "User:"}
	Stop []string
	// Language specifies the language of the model
	Language string
}

type ChatModel struct {
	cli  *hunyuan.Client
	conf *ChatModelConfig

	tools      []*hunyuan.Tool
	rawTools   []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func NewChatModel(_ context.Context, conf *ChatModelConfig) (*ChatModel, error) {
	if len(conf.Model) == 0 {
		return nil, fmt.Errorf("model is required")
	}
	credential := common.NewCredential(
		conf.SecretId,
		conf.SecretKey,
	)

	region := defaultRegion
	if len(conf.Region) > 0 {
		region = conf.Region
	}
	pro := profile.NewClientProfile()
	if len(conf.Language) > 0 {
		pro.Language = conf.Language
	}
	cli, err := hunyuan.NewClient(credential, region, pro)
	if err != nil {
		return nil, err
	}

	return &ChatModel{
		cli:  cli,
		conf: conf,
	}, nil
}

func (cm *ChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools to bind")
	}
	hunyuanTools, err := toTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to hunyuan tools fail: %w", err)
	}

	tc := schema.ToolChoiceAllowed
	ncm := *cm
	ncm.tools = hunyuanTools
	ncm.rawTools = tools
	ncm.toolChoice = &tc
	return &ncm, nil
}

func (cm *ChatModel) BindTools(tools []*schema.ToolInfo) error {
	if len(tools) == 0 {
		return fmt.Errorf("no tools to bind")
	}
	var err error
	cm.tools, err = toTools(tools)
	if err != nil {
		return err
	}

	tc := schema.ToolChoiceAllowed
	cm.toolChoice = &tc
	cm.rawTools = tools
	return nil
}

func (cm *ChatModel) GetType() string {
	return typ
}

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (outMsg *schema.Message, err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.buildRequest(in, false, opts...)
	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := cm.cli.ChatCompletionsWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	outMsg = convertResponse(resp.Response)

	callbacks.OnEnd(ctx, &model.CallbackOutput{
		Message:    outMsg,
		Config:     cbInput.Config,
		TokenUsage: toCallbackUsage(outMsg.ResponseMeta.Usage),
	})
	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	req, cbInput, err := cm.buildRequest(in, true, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate stream request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, cbInput)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := cm.cli.ChatCompletionsWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			panicErr := recover()

			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
		}()

		for event := range resp.Events {
			if event.Err != nil {
				sw.Send(nil, event.Err)
				return
			}
			if len(event.Data) == 0 {
				continue
			}
			var dat hunyuan.ChatCompletionsResponseParams
			if err_ := json.Unmarshal(event.Data, &dat); err != nil {
				sw.Send(nil, err_)
				return
			}
			msg := convertResponse(&dat)
			closed := sw.Send(&model.CallbackOutput{
				Message:    msg,
				Config:     cbInput.Config,
				TokenUsage: toCallbackUsage(msg.ResponseMeta.Usage),
			}, nil)
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

func (cm *ChatModel) buildRequest(inputs []*schema.Message, stream bool, opts ...model.Option) (*hunyuan.ChatCompletionsRequest, *model.CallbackInput, error) {
	options := model.GetCommonOptions(&model.Options{
		Model:       &cm.conf.Model,
		Temperature: &cm.conf.Temperature,
		TopP:        &cm.conf.TopP,
		Stop:        cm.conf.Stop,
		Tools:       nil,
		ToolChoice:  cm.toolChoice,
	}, opts...)

	req := &hunyuan.ChatCompletionsRequest{
		Model:  options.Model,
		Stream: toPtr(stream),
	}
	if len(options.Stop) > 0 {
		stop := make([]*string, 0, len(options.Stop))
		for _, s := range options.Stop {
			stop = append(stop, toPtr(s))
		}
		req.Stop = stop
	}
	cbInput := &model.CallbackInput{
		Messages:   inputs,
		Tools:      cm.rawTools,
		ToolChoice: options.ToolChoice,
		Config: &model.Config{
			Model: *req.Model,
		},
	}
	if options.Temperature != nil {
		req.Temperature = toPtr(float64(*options.Temperature))
		cbInput.Config.Temperature = *options.Temperature
	}
	if options.TopP != nil {
		req.TopP = toPtr(float64(*options.TopP))
		cbInput.Config.TopP = *options.TopP
	}

	tools := cm.tools
	if options.Tools != nil {
		var err error
		if tools, err = toTools(options.Tools); err != nil {
			return nil, nil, err
		}
		cbInput.Tools = options.Tools
	}

	req.Tools = make([]*hunyuan.Tool, len(tools))
	copy(req.Tools, tools)

	err := populateToolChoice(req, options.ToolChoice, options.AllowedToolNames)
	if err != nil {
		return nil, nil, err
	}
	msgs := make([]*hunyuan.Message, 0, len(inputs))
	for _, inMsg := range inputs {
		msg, e := convertMessage(inMsg)
		if e != nil {
			return nil, nil, e
		}

		msgs = append(msgs, msg)
	}
	req.Messages = msgs
	return req, cbInput, nil
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
