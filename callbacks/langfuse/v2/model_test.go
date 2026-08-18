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
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestModelUsageUsesInclusiveOTelCounts(t *testing.T) {
	tests := map[string]*modelOutput{
		"callback usage": {
			Usage: &model.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 30,
				TotalTokens:      130,
				PromptTokenDetails: model.PromptTokenDetails{
					CachedTokens: 20,
				},
				CompletionTokensDetails: model.CompletionTokensDetails{
					ReasoningTokens: 10,
				},
			},
		},
		"response metadata usage": {
			Message: &schema.Message{ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 30,
				TotalTokens:      130,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: 20,
				},
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: 10,
				},
			}}},
		},
	}
	want := &tokenUsage{
		Input:           100,
		Output:          30,
		InputCached:     20,
		OutputReasoning: 10,
	}
	for name, output := range tests {
		t.Run(name, func(t *testing.T) {
			if got := modelUsage(output); !reflect.DeepEqual(got, want) {
				t.Fatalf("modelUsage() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestInclusiveTokenUsageClampsInvalidDetails(t *testing.T) {
	got := inclusiveTokenUsage(5, 3, 10, 4)
	want := &tokenUsage{Input: 5, Output: 3, InputCached: 5, OutputReasoning: 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inclusiveTokenUsage() = %#v, want %#v", got, want)
	}
}

func TestSetUsageUsesOTelGenAIAttributes(t *testing.T) {
	got := make(map[string]int)
	setUsage(func(key string, value int) {
		got[key] = value
	}, &tokenUsage{Input: 100, Output: 30, InputCached: 20, OutputReasoning: 10})

	want := map[string]int{
		"gen_ai.usage.input_tokens":            100,
		"gen_ai.usage.output_tokens":           30,
		"gen_ai.usage.cache_read.input_tokens": 20,
		"gen_ai.usage.reasoning.output_tokens": 10,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("usage attributes = %#v, want %#v", got, want)
	}
}
