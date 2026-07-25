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

package openai

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	openai "github.com/meguminnnnnnnnn/go-openai"
	"github.com/stretchr/testify/assert"
)

func TestToEinoTokenUsageCachedTokens(t *testing.T) {
	tests := []struct {
		name  string
		usage *openai.Usage
		want  int
	}{
		{
			name: "OpenAI prompt token details",
			usage: &openai.Usage{
				PromptTokensDetails: &openai.PromptTokensDetails{CachedTokens: 64},
				ExtraFields: map[string]json.RawMessage{
					"prompt_cache_hit_tokens": json.RawMessage(`32`),
				},
			},
			want: 64,
		},
		{
			name: "DeepSeek top-level cache hit extension",
			usage: &openai.Usage{
				ExtraFields: map[string]json.RawMessage{
					"prompt_cache_hit_tokens": json.RawMessage(`48`),
				},
			},
			want: 48,
		},
		{
			name: "malformed extension",
			usage: &openai.Usage{
				ExtraFields: map[string]json.RawMessage{
					"prompt_cache_hit_tokens": json.RawMessage(`"invalid"`),
				},
			},
			want: 0,
		},
		{
			name:  "missing cache details",
			usage: &openai.Usage{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toEinoTokenUsage(tt.usage)
			assert.Equal(t, schema.PromptTokenDetails{CachedTokens: tt.want}, got.PromptTokenDetails)
		})
	}
}
