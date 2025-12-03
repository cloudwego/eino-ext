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

package gemini

import (
	"fmt"

	"google.golang.org/genai"

	"github.com/cloudwego/eino/components/agentic"
	"github.com/cloudwego/eino/schema"
)

// genInputAndConf generates input messages and configuration for Gemini API
func (g *gemini) genInputAndConf(input []*schema.AgenticMessage, opts ...agentic.Option) (string, []*schema.AgenticMessage, *genai.GenerateContentConfig, *agentic.Config, error) {
	commonOptions := agentic.GetCommonOptions(&agentic.Options{
		Temperature: g.temperature,
		TopP:        g.topP,
		Tools:       nil,
		ToolChoice:  g.toolChoice,
	}, opts...)
	geminiOptions := agentic.GetImplSpecificOptions(&options{
		TopK:               g.topK,
		ResponseJSONSchema: g.responseJSONSchema,
		ResponseModalities: g.responseModalities,
	}, opts...)
	conf := &agentic.Config{}

	m := &genai.GenerateContentConfig{
		SafetySettings: g.safetySettings,
	}
	if commonOptions.Model != nil {
		conf.Model = *commonOptions.Model
	} else {
		conf.Model = g.model
	}

	tools := g.tools
	if commonOptions.Tools != nil {
		var err error
		tools, err = toGeminiTools(commonOptions.Tools)
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
	if g.enableCodeExecution {
		m.Tools = append(m.Tools, &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		})
	}

	m.MediaResolution = g.mediaResolution

	if g.maxTokens != nil {
		m.MaxOutputTokens = int32(*g.maxTokens)
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

	nInput := make([]*schema.AgenticMessage, len(input))
	copy(nInput, input)
	if len(input) > 1 && input[0].Role == schema.AgenticRoleTypeSystem {
		var err error
		m.SystemInstruction, err = g.convAgenticMessage(input[0])
		if err != nil {
			return "", nil, nil, nil, fmt.Errorf("failed to convert system instruction: %w", err)
		}
		nInput = input[1:]
	}

	m.ThinkingConfig = g.thinkingConfig
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

func convCallbackOutput(message *schema.AgenticMessage, conf *agentic.Config) *agentic.CallbackOutput {
	callbackOutput := &agentic.CallbackOutput{
		Message: message,
		Config:  conf,
	}
	return callbackOutput
}
