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
	"errors"
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

var _ model.AgenticModel = (*Gemini)(nil)

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

	// EnableCodeExecution allows the model to use the server tool CodeExecution
	// Optional.
	EnableCodeExecution *genai.ToolCodeExecution
	// EnableGoogleSearch allows the model to use the server tool GoogleSearch
	// Optional.
	EnableGoogleSearch *genai.GoogleSearch
	// EnableGoogleSearchRetrieval allows the model to use the server tool GoogleSearchRetrieval
	// Optional.
	EnableGoogleSearchRetrieval *genai.GoogleSearchRetrieval
	// EnableComputerUse allows the model to use the server tool ComputerUse
	// Optional.
	EnableComputerUse *genai.ComputerUse
	// EnableURLContext allows the model to use the server tool URLContext
	// Optional.
	EnableURLContext *genai.URLContext
	// EnableFileSearch allows the model to use the server tool FileSearch
	// Optional.
	EnableFileSearch *genai.FileSearch
	// EnableGoogleMaps allows the model to use the server tool GoogleMaps
	// Optional.
	EnableGoogleMaps *genai.GoogleMaps

	// SafetySettings configures content filtering for different harm categories
	// Controls the model's filtering behavior for potentially harmful content
	// Optional.
	SafetySettings []*genai.SafetySetting

	ThinkingConfig *genai.ThinkingConfig

	// ResponseModalities specifies the modalities the model can return.
	// Optional.
	ResponseModalities []ResponseModality

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

type ResponseModality string

const (
	ResponseModalityText  ResponseModality = "TEXT"
	ResponseModalityImage ResponseModality = "IMAGE"
	ResponseModalityAudio ResponseModality = "AUDIO"
)

// NewAgenticModel creates a new Gemini agentic model instance
func NewAgenticModel(_ context.Context, cfg *Config) (*Gemini, error) {
	return &Gemini{
		cli: cfg.Client,

		model:                       cfg.Model,
		maxTokens:                   cfg.MaxTokens,
		temperature:                 cfg.Temperature,
		topP:                        cfg.TopP,
		topK:                        cfg.TopK,
		responseJSONSchema:          cfg.ResponseJSONSchema,
		enableCodeExecution:         cfg.EnableCodeExecution,
		enableGoogleSearch:          cfg.EnableGoogleSearch,
		enableGoogleSearchRetrieval: cfg.EnableGoogleSearchRetrieval,
		enableComputerUse:           cfg.EnableComputerUse,
		enableURLContext:            cfg.EnableURLContext,
		enableFileSearch:            cfg.EnableFileSearch,
		enableGoogleMaps:            cfg.EnableGoogleMaps,
		safetySettings:              cfg.SafetySettings,
		thinkingConfig:              cfg.ThinkingConfig,
		responseModalities:          cfg.ResponseModalities,
		mediaResolution:             cfg.MediaResolution,
		cache:                       cfg.Cache,
	}, nil
}

type Gemini struct {
	cli *genai.Client

	model                       string
	maxTokens                   *int
	topP                        *float32
	temperature                 *float32
	topK                        *int32
	responseJSONSchema          *jsonschema.Schema
	tools                       []*genai.FunctionDeclaration
	origTools                   []*schema.ToolInfo
	toolChoice                  *schema.AgenticToolChoice
	enableCodeExecution         *genai.ToolCodeExecution
	enableGoogleSearch          *genai.GoogleSearch
	enableGoogleSearchRetrieval *genai.GoogleSearchRetrieval
	enableComputerUse           *genai.ComputerUse
	enableURLContext            *genai.URLContext
	enableFileSearch            *genai.FileSearch
	enableGoogleMaps            *genai.GoogleMaps
	safetySettings              []*genai.SafetySetting
	thinkingConfig              *genai.ThinkingConfig
	responseModalities          []ResponseModality
	mediaResolution             genai.MediaResolution
	cache                       *CacheConfig
}

// CreatePrefixCache assembles inputs the same as Generate/Stream and writes
// the final system instruction, tools, and messages into a reusable prefix cache.
func (g *Gemini) CreatePrefixCache(ctx context.Context, prefixMsgs []*schema.AgenticMessage, opts ...model.Option) (
	*genai.CachedContent, error) {

	modelName, inputMsgs, genaiConf, _, err := g.genInputAndConf(prefixMsgs, opts...)
	if err != nil {
		return nil, fmt.Errorf("genInputAndConf for CreatePrefixCache failed: %w", err)
	}

	contents, err := convAgenticMessages(inputMsgs)
	if err != nil {
		return nil, err
	}

	cachedContent, err := g.cli.Caches.Create(ctx, modelName, &genai.CreateCachedContentConfig{
		Contents:          contents,
		SystemInstruction: genaiConf.SystemInstruction,
		Tools:             genaiConf.Tools,
		ToolConfig:        genaiConf.ToolConfig,
		TTL: func() time.Duration {
			if g.cache != nil {
				return g.cache.TTL
			}
			return 0
		}(),
		ExpireTime: func() time.Time {
			if g.cache != nil {
				return g.cache.ExpireTime
			}
			return time.Time{}
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("create cache failed: %w", err)
	}

	return cachedContent, nil
}

func (g *Gemini) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
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

func (g *Gemini) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
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

func (g *Gemini) WithTools(tools []*schema.ToolInfo) (model.AgenticModel, error) {
	if len(tools) == 0 {
		return nil, errors.New("no tools to bind")
	}
	gTools, err := toGeminiTools(tools)
	if err != nil {
		return nil, fmt.Errorf("convert to gemini tools fail: %w", err)
	}

	ng := *g
	ng.toolChoice = &schema.AgenticToolChoice{
		Type: schema.ToolChoiceAllowed,
	}
	ng.tools = gTools
	ng.origTools = tools
	return &ng, nil
}

func (g *Gemini) GetType() string          { return "Gemini" }
func (g *Gemini) IsCallbacksEnabled() bool { return true }

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
