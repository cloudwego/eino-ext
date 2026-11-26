/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	openaischema "github.com/cloudwego/eino/schema/openai"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/require"
)

func TestWebSearchCheckpointRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{
			name:   "search",
			action: `{"type":"search","queries":["eino"],"sources":[{"type":"url","url":"https://example.com"}]}`,
		},
		{
			name:   "open_page",
			action: `{"type":"open_page","url":"https://example.com"}`,
		},
		{
			name:   "find_in_page",
			action: `{"type":"find_in_page","url":"https://example.com","pattern":"checkpoint"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item responses.ResponseFunctionWebSearch
			require.NoError(t, json.Unmarshal([]byte(
				`{"type":"web_search_call","id":"ws_checkpoint","status":"completed","action":`+tt.action+`}`,
			), &item))

			blocks, err := webSearchToContentBlocks(item)
			require.NoError(t, err)
			require.Len(t, blocks, 2)
			message := &schema.AgenticMessage{
				Role:          schema.AgenticRoleTypeAssistant,
				ContentBlocks: blocks,
				ResponseMeta: &schema.AgenticResponseMeta{
					OpenAIExtension: &openaischema.ResponseMetaExtension{},
				},
			}

			var checkpoint bytes.Buffer
			require.NoError(t, gob.NewEncoder(&checkpoint).Encode(message))
			var restored schema.AgenticMessage
			require.NoError(t, gob.NewDecoder(&checkpoint).Decode(&restored))
			require.Equal(t, message, &restored)

			items, err := toAssistantRoleInputItems(&restored)
			require.NoError(t, err)
			require.Len(t, items, 1)
			require.NotNil(t, items[0].OfWebSearchCall)
			require.Equal(t, "ws_checkpoint", items[0].OfWebSearchCall.ID)
			require.Equal(t, responses.ResponseFunctionWebSearchStatus("completed"), items[0].OfWebSearchCall.Status)
			actionJSON, err := json.Marshal(items[0].OfWebSearchCall.Action)
			require.NoError(t, err)
			require.JSONEq(t, tt.action, string(actionJSON))
		})
	}
}
