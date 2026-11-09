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

package ark

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func (cm *ChatModel) getOptions(opts ...model.Option) (*model.Options, *arkOptions) {
	common := model.GetCommonOptions(&model.Options{
		Temperature: cm.config.Temperature,
		MaxTokens:   cm.config.MaxTokens,
		Model:       &cm.config.Model,
		TopP:        cm.config.TopP,
		Stop:        cm.config.Stop,
		ToolChoice:  cm.toolChoice,
	}, opts...)
	ark := model.GetImplSpecificOptions(&arkOptions{
		customHeaders:                cm.config.CustomHeader,
		thinking:                     cm.config.Thinking,
		reasoningEffort:              cm.config.ReasoningEffort,
		maxCompletionTokens:          cm.config.MaxCompletionTokens,
		parallelToolCalls:            cm.config.ParallelToolCalls,
		requestPayloadModifier:       cm.config.RequestPayloadModifier,
		responseMessageModifier:      cm.config.ResponseMessageModifier,
		responseChunkMessageModifier: cm.config.ResponseChunkMessageModifier,
	}, opts...)
	return common, ark
}

// WithThinking configures thinking for Chat Completions.
func WithThinking(thinking *chat.Thinking) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.thinking = thinking })
}

// WithReasoningEffort configures reasoning effort for Chat Completions.
func WithReasoningEffort(effort chat.ReasoningEffort) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.reasoningEffort = &effort })
}

func WithMaxCompletionTokens(n int) model.Option {
	return model.WrapImplSpecificOptFn(func(o *arkOptions) { o.maxCompletionTokens = &n })
}
