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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestAttack_ParallelSameNameResultsKeepIDs(t *testing.T) {
	candidate := &genai.Candidate{
		Content: &genai.Content{
			Role: roleModel,
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{ID: "call-a", Name: "same_tool", Args: map[string]any{"key": "alpha"}}},
				{FunctionCall: &genai.FunctionCall{ID: "call-b", Name: "same_tool", Args: map[string]any{"key": "beta"}}},
			},
		},
	}

	message, err := convAgenticCandidate(candidate, "")
	require.NoError(t, err)
	require.Len(t, message.ContentBlocks, 2)
	require.Equal(t, "call-a", message.ContentBlocks[0].FunctionToolCall.CallID)
	require.Equal(t, "call-b", message.ContentBlocks[1].FunctionToolCall.CallID)

	result := func(callID, value string) *schema.AgenticMessage {
		return &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: callID,
					Name:   "same_tool",
					Content: []*schema.FunctionToolResultContentBlock{{
						Type: schema.FunctionToolResultContentBlockTypeText,
						Text: &schema.UserInputText{Text: `{"value":"` + value + `"}`},
					}},
				}),
			},
		}
	}
	contents, err := convAgenticMessages([]*schema.AgenticMessage{
		message,
		result("call-b", "B"),
		result("call-a", "A"),
	})
	require.NoError(t, err)
	require.Len(t, contents, 2)
	require.Len(t, contents[1].Parts, 2)
	require.Equal(t, "call-b", contents[1].Parts[0].FunctionResponse.ID)
	require.Equal(t, "B", contents[1].Parts[0].FunctionResponse.Response["value"])
	require.Equal(t, "call-a", contents[1].Parts[1].FunctionResponse.ID)
	require.Equal(t, "A", contents[1].Parts[1].FunctionResponse.Response["value"])
}

func TestAttack_LegacyFallbackIDsAreUnique(t *testing.T) {
	first, err := convAgenticFC(&genai.FunctionCall{Name: "same_tool", Args: map[string]any{"key": "alpha"}})
	require.NoError(t, err)
	second, err := convAgenticFC(&genai.FunctionCall{Name: "same_tool", Args: map[string]any{"key": "beta"}})
	require.NoError(t, err)

	firstID := first.FunctionToolCall.CallID
	secondID := second.FunctionToolCall.CallID
	require.NotEqual(t, firstID, secondID)
	_, err = uuid.Parse(firstID)
	require.NoError(t, err)
	_, err = uuid.Parse(secondID)
	require.NoError(t, err)
}
