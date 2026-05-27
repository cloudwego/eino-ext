/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticgemini

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/eino-contrib/jsonschema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const implType = "AgenticGemini"

var _ model.AgenticModel = (*Model)(nil)

// Config contains the configuration options for the Gemini agentic model
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

	// SafetySettings configures content filtering for different harm categories
	// Controls the model's filtering behavior for potentially harmful content
	// Optional.
	SafetySettings []*genai.SafetySetting

	ThinkingConfig *genai.ThinkingConfig

	// ImageConfig is the image generation configuration.
	// Note: an error will be returned if this field is set for a model that does not support the configuration options.
	// Optional.
	ImageConfig *genai.ImageConfig

	// ResponseModalities specifies the modalities the model can return.
	// Optional.
	ResponseModalities []genai.Modality

	MediaResolution genai.MediaResolution

	// CacheExpiration configures the expiration policy for prefix cache resources.
	// Optional.
	CacheExpiration *CacheExpiration
}

// CacheExpiration configures the expiration policy for prefix cache resources.
// Exactly one field should be set.
type CacheExpiration struct {
	// TTL specifies how long prefix cache resources remain valid (now + TTL).
	TTL *time.Duration
	// ExpireTime sets the absolute expiration timestamp for prefix cache resources.
	ExpireTime *time.Time
}

type ServerToolConfig struct {
	CodeExecution         *genai.ToolCodeExecution
	GoogleSearch          *genai.GoogleSearch
	GoogleSearchRetrieval *genai.GoogleSearchRetrieval
	URLContext            *genai.URLContext
	FileSearch            *genai.FileSearch
	GoogleMaps            *genai.GoogleMaps
}

// New creates a new Gemini agentic model instance
func New(_ context.Context, cfg *Config) (*Model, error) {
	return &Model{
		cli: cfg.Client,

		model:              cfg.Model,
		maxTokens:          cfg.MaxTokens,
		temperature:        cfg.Temperature,
		topP:               cfg.TopP,
		topK:               cfg.TopK,
		responseJSONSchema: cfg.ResponseJSONSchema,
		safetySettings:     cfg.SafetySettings,
		thinkingConfig:     cfg.ThinkingConfig,
		imageConfig:        cfg.ImageConfig,
		responseModalities: cfg.ResponseModalities,
		mediaResolution:    cfg.MediaResolution,
		cacheExpiration:    cfg.CacheExpiration,
	}, nil
}

type Model struct {
	cli *genai.Client

	model              string
	maxTokens          *int
	topP               *float32
	temperature        *float32
	topK               *int32
	responseJSONSchema *jsonschema.Schema
	tools              []*genai.FunctionDeclaration
	origTools          []*schema.ToolInfo
	toolChoice         *schema.AgenticToolChoice
	safetySettings     []*genai.SafetySetting
	thinkingConfig     *genai.ThinkingConfig
	imageConfig        *genai.ImageConfig
	responseModalities []genai.Modality
	mediaResolution    genai.MediaResolution
	cacheExpiration    *CacheExpiration
}

func toServerTools(serverTools []*ServerToolConfig) ([]*genai.Tool, error) {
	tools := make([]*genai.Tool, len(serverTools))

	for i := range serverTools {
		ti := serverTools[i]
		if ti == nil {
			return nil, fmt.Errorf("unknown server tool type")
		}
		switch {
		case ti.CodeExecution != nil:
			tools[i] = &genai.Tool{
				CodeExecution: ti.CodeExecution,
			}
		case ti.GoogleSearch != nil:
			tools[i] = &genai.Tool{
				GoogleSearch: ti.GoogleSearch,
			}
		case ti.GoogleSearchRetrieval != nil:
			tools[i] = &genai.Tool{
				GoogleSearchRetrieval: ti.GoogleSearchRetrieval,
			}
		case ti.URLContext != nil:
			tools[i] = &genai.Tool{
				URLContext: ti.URLContext,
			}
		case ti.FileSearch != nil:
			tools[i] = &genai.Tool{
				FileSearch: ti.FileSearch,
			}
		case ti.GoogleMaps != nil:
			tools[i] = &genai.Tool{
				GoogleMaps: ti.GoogleMaps,
			}
		default:
			return nil, fmt.Errorf("unknown server tool type")
		}
	}

	return tools, nil
}

// CreatePrefixCache assembles inputs the same as Generate/Stream and writes
// the final system instruction, tools, and messages into a reusable prefix cache.
func (g *Model) CreatePrefixCache(ctx context.Context, prefixMsgs []*schema.AgenticMessage, opts ...model.Option) (
	*genai.CachedContent, error) {

	modelName, inputMsgs, genaiConf, _, err := g.genInputAndConf(prefixMsgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for CreatePrefixCache failed: %w", err)
	}

	contents, err := convAgenticMessages(inputMsgs)
	if err != nil {
		return nil, err
	}

	createCfg := &genai.CreateCachedContentConfig{
		Contents:          contents,
		SystemInstruction: genaiConf.SystemInstruction,
		Tools:             genaiConf.Tools,
		ToolConfig:        genaiConf.ToolConfig,
	}
	if g.cacheExpiration != nil {
		if g.cacheExpiration.TTL != nil {
			createCfg.TTL = *g.cacheExpiration.TTL
		}
		if g.cacheExpiration.ExpireTime != nil {
			createCfg.ExpireTime = *g.cacheExpiration.ExpireTime
		}
	}

	cachedContent, err := g.cli.Caches.Create(ctx, modelName, createCfg)
	if err != nil {
		return nil, fmt.Errorf("create cache failed: %w", err)
	}

	return cachedContent, nil
}

func (g *Model) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	ctx = callbacks.EnsureRunInfo(ctx, g.GetType(), components.ComponentOfChatModel)

	modelName, nInput, genaiConf, cbConf, err := g.genInputAndConf(input, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for Generate failed: %w", err)
	}

	co := model.GetCommonOptions(&model.Options{
		Tools: g.origTools,
	}, opts...)
	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    co.Tools,
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
	contents, err := convAgenticMessages(nInput)
	if err != nil {
		return nil, err
	}

	result, err := g.cli.Models.GenerateContent(ctx, modelName, contents, genaiConf)
	if err != nil {
		return nil, fmt.Errorf("send message fail: %w", err)
	}

	message, err := convAgenticResponse(result, "")
	if err != nil {
		return nil, fmt.Errorf("convert response fail: %w", err)
	}

	callbacks.OnEnd(ctx, convCallbackOutput(message, cbConf))
	return message, nil
}

func (g *Model) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	ctx = callbacks.EnsureRunInfo(ctx, g.GetType(), components.ComponentOfChatModel)

	modelName, nInput, genaiConf, cbConf, err := g.genInputAndConf(input, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for Stream failed: %w", err)
	}

	co := model.GetCommonOptions(&model.Options{
		Tools: g.origTools,
	}, opts...)
	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    co.Tools,
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

	contents, err := convAgenticMessages(nInput)
	if err != nil {
		return nil, fmt.Errorf("convert schema message fail: %w", err)
	}
	resultIter := g.cli.Models.GenerateContentStream(ctx, modelName, contents, genaiConf)

	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](1)
	go func() {
		defer func() {
			pe := recover()

			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}
			sw.Close()
		}()
		var curIndex int
		var lastType schema.ContentBlockType
		for resp, err_ := range resultIter {
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			message, err_ := convAgenticResponse(resp, lastType)
			if err_ != nil {
				sw.Send(nil, err_)
				return
			}
			curIndex, lastType = populateStreamingMeta(message.ContentBlocks, curIndex, lastType)
			closed := sw.Send(convCallbackOutput(message, cbConf), nil)
			if closed {
				return
			}
		}
	}()
	srList := sr.Copy(2)
	callbacks.OnEndWithStreamOutput(ctx, srList[0])
	return schema.StreamReaderWithConvert(srList[1], func(t *model.AgenticCallbackOutput) (*schema.AgenticMessage, error) {
		return t.Message, nil
	}), nil
}

func (g *Model) GetType() string          { return implType }
func (g *Model) IsCallbacksEnabled() bool { return true }

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
