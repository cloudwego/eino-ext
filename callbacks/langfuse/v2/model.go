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

package langfuse

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type stringAttributeSetter func(key, value string) string
type intAttributeSetter func(key string, value int)

type modelOutput struct {
	Message *schema.Message
	Config  *model.Config
	Usage   *model.TokenUsage
}

type tokenUsage struct {
	Input           int
	Output          int
	InputCached     int
	InputCacheWrite int
	Total           int
	OutputReasoning int
}

func extractModelInput(inputs []callbacks.CallbackInput) (*model.CallbackInput, error) {
	result := &model.CallbackInput{}
	var messageArrays [][]*schema.Message
	for _, input := range inputs {
		converted := model.ConvCallbackInput(input)
		if converted == nil {
			continue
		}
		if converted.Config != nil {
			result.Config = converted.Config
		}
		if len(converted.Messages) > 0 {
			messageArrays = append(messageArrays, converted.Messages)
		}
		if converted.Extra != nil {
			result.Extra = converted.Extra
		}
		if converted.Tools != nil {
			result.Tools = converted.Tools
		}
		if converted.ToolChoice != nil {
			result.ToolChoice = converted.ToolChoice
		}
	}
	if len(messageArrays) > 0 {
		messages, err := concatMessageArrays(messageArrays)
		if err != nil {
			return nil, err
		}
		result.Messages = messages
	}
	return result, nil
}

type toolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func toolDefinitions(tools []*schema.ToolInfo) ([]toolDefinition, error) {
	definitions := make([]toolDefinition, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		definition := toolDefinition{Name: tool.Name, Description: tool.Desc}
		parameters, err := tool.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("tool %q parameters: %w", tool.Name, err)
		}
		if parameters != nil {
			definition.Parameters = parameters
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func extractModelOutput(outputs []callbacks.CallbackOutput) (*modelOutput, map[string]any, error) {
	result := &modelOutput{}
	var messages []*schema.Message
	var extra map[string]any
	for _, output := range outputs {
		converted := model.ConvCallbackOutput(output)
		if converted == nil {
			continue
		}
		if converted.Message != nil {
			messages = append(messages, converted.Message)
		}
		if converted.Config != nil {
			result.Config = converted.Config
		}
		if converted.TokenUsage != nil {
			result.Usage = converted.TokenUsage
		}
		if converted.Extra != nil {
			extra = converted.Extra
		}
	}
	if len(messages) > 0 {
		message, err := schema.ConcatMessages(messages)
		if err != nil {
			return nil, nil, fmt.Errorf("concat model output: %w", err)
		}
		result.Message = message
	}
	return result, extra, nil
}

func concatMessageArrays(arrays [][]*schema.Message) ([]*schema.Message, error) {
	expected := len(arrays[0])
	parts := make([][]*schema.Message, expected)
	for _, messages := range arrays {
		if len(messages) != expected {
			return nil, fmt.Errorf("unexpected streamed message count: got %d, want %d", len(messages), expected)
		}
		for index, message := range messages {
			if message != nil {
				parts[index] = append(parts[index], message)
			}
		}
	}
	result := make([]*schema.Message, expected)
	for index, messages := range parts {
		switch len(messages) {
		case 0:
		case 1:
			result[index] = messages[0]
		default:
			message, err := schema.ConcatMessages(messages)
			if err != nil {
				return nil, fmt.Errorf("concat streamed input message %d: %w", index, err)
			}
			result[index] = message
		}
	}
	return result, nil
}

func setModelConfig(setAttribute stringAttributeSetter, config *model.Config, toolChoice *schema.ToolChoice) {
	if config == nil && toolChoice == nil {
		return
	}
	if config == nil {
		config = &model.Config{}
	}
	if config.Model != "" {
		setAttribute("langfuse.observation.model.name", config.Model)
	}
	parameters := make(map[string]any, 5)
	if toolChoice != nil {
		parameters["tool_choice"] = string(*toolChoice)
		setAttribute("gen_ai.request.tool_choice", string(*toolChoice))
	}
	if config.MaxTokens != 0 {
		parameters["max_tokens"] = config.MaxTokens
	}
	if config.Temperature != 0 {
		parameters["temperature"] = config.Temperature
	}
	if config.TopP != 0 {
		parameters["top_p"] = config.TopP
	}
	if len(config.Stop) > 0 {
		parameters["stop"] = config.Stop
	}
	if len(parameters) == 0 {
		return
	}
	encoded, err := json.Marshal(parameters)
	if err == nil {
		setAttribute("langfuse.observation.model.parameters", string(encoded))
	}
}

// setUsage emits inclusive OTel GenAI counters. Langfuse normalizes the cache
// and reasoning detail counts into mutually exclusive usage buckets on ingest.
func setUsage(setAttribute intAttributeSetter, usage *tokenUsage) {
	if usage == nil {
		return
	}
	setAttribute("gen_ai.usage.input_tokens", usage.Input)
	setAttribute("gen_ai.usage.output_tokens", usage.Output)
	setAttribute("gen_ai.usage.total_tokens", usage.Total)
	setAttribute("gen_ai.usage.cache_creation.input_tokens", usage.InputCacheWrite)
	setAttribute("gen_ai.usage.cache_read.input_tokens", usage.InputCached)
	setAttribute("gen_ai.usage.reasoning.output_tokens", usage.OutputReasoning)
}

func modelUsage(output *modelOutput) *tokenUsage {
	if output == nil {
		return nil
	}
	if output.Usage != nil {
		return inclusiveTokenUsage(
			output.Usage.PromptTokens,
			output.Usage.CompletionTokens,
			output.Usage.TotalTokens,
			output.Usage.PromptTokenDetails.CachedTokens,
			output.Usage.PromptTokenDetails.CacheWriteTokens,
			output.Usage.CompletionTokensDetails.ReasoningTokens,
		)
	}
	if output.Message != nil && output.Message.ResponseMeta != nil {
		usage := output.Message.ResponseMeta.Usage
		if usage != nil {
			return inclusiveTokenUsage(
				usage.PromptTokens,
				usage.CompletionTokens,
				usage.TotalTokens,
				usage.PromptTokenDetails.CachedTokens,
				usage.PromptTokenDetails.CacheWriteTokens,
				usage.CompletionTokensDetails.ReasoningTokens,
			)
		}
	}
	return nil
}

func inclusiveTokenUsage(input, output, total, inputCached, inputCacheWrite, outputReasoning int) *tokenUsage {
	input = max(input, 0)
	output = max(output, 0)
	inputCached = min(max(inputCached, 0), input)
	inputCacheWrite = min(max(inputCacheWrite, 0), input-inputCached)
	outputReasoning = min(max(outputReasoning, 0), output)
	return &tokenUsage{
		Input:           input,
		Output:          output,
		InputCached:     inputCached,
		InputCacheWrite: inputCacheWrite,
		Total:           max(total, 0),
		OutputReasoning: outputReasoning,
	}
}

func setMetadata(setAttribute stringAttributeSetter, metadata map[string]any, losses *lossReporter) {
	for key, value := range metadata {
		var serialized string
		if text, ok := value.(string); ok {
			serialized = text
		} else {
			encoded, err := json.Marshal(value)
			if err != nil {
				losses.Add(lossMetadataSerialization, 1)
				continue
			}
			serialized = string(encoded)
		}
		setAttribute("langfuse.observation.metadata."+metadataKey(key), serialized)
	}
}
