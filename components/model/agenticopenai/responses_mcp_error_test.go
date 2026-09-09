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
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/openai"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPErrorUnionHistoryReplay(t *testing.T) {
	for _, tc := range []struct {
		name, raw, message string
		code               *int64
	}{
		{"protocol", `{"type":"mcp_protocol_error","code":-32603,"message":"protocol failure"}`, "protocol failure", ptrOf(int64(-32603))},
		{"zero_code", `{"type":"mcp_protocol_error","code":0,"message":"failure"}`, "failure", ptrOf(int64(0))},
		{"http", `{"type":"http_error","code":503,"message":"unavailable"}`, "unavailable", ptrOf(int64(503))},
		{"execution", `{"type":"mcp_tool_execution_error","content":[{"type":"text","text":"failed"}]}`, `[{"text":"failed","type":"text"}]`, nil},
		{"future", `{"type":"future_error","detail":{"x":1}}`, `{"type":"future_error","detail":{"x":1}}`, nil},
		{"legacy", `"legacy error"`, "legacy error", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var item responses.ResponseOutputItemMcpCall
			require.NoError(t, json.Unmarshal([]byte(`{"id":"m1","type":"mcp_call","name":"lookup","server_label":"server","arguments":"{}","status":"failed","error":`+tc.raw+`}`), &item))
			blocks, err := mcpCallToContentBlocks(item)
			require.NoError(t, err)
			require.Len(t, blocks, 2)
			detail := blocks[1].MCPToolResult.Error
			require.NotNil(t, detail)
			assert.Equal(t, tc.message, detail.Message)
			assert.Equal(t, tc.code, detail.Code)
			// Check both original blocks and JSON-persisted history, whose Extra
			// values lose private Go alias types when decoded through any.
			for _, persisted := range []bool{false, true} {
				msg := &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: blocks, ResponseMeta: &schema.AgenticResponseMeta{OpenAIExtension: &openai.ResponseMetaExtension{ID: "r1"}}}
				if persisted {
					raw, err := json.Marshal(msg)
					require.NoError(t, err)
					msg = &schema.AgenticMessage{}
					require.NoError(t, json.Unmarshal(raw, msg))
				}
				items, err := toAssistantRoleInputItems(msg)
				require.NoError(t, err)
				require.Len(t, items, 1)
				raw, err := json.Marshal(items[0])
				require.NoError(t, err)
				var wire map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(raw, &wire))
				assert.JSONEq(t, tc.raw, string(wire["error"]))
			}
		})
	}
}

func TestMCPErrorAbsentAndLegacyHistory(t *testing.T) {
	for _, raw := range []string{`{}`, `{"error":null}`} {
		var item responses.ResponseOutputItemMcpCall
		require.NoError(t, json.Unmarshal([]byte(raw), &item))
		blocks, err := mcpCallToContentBlocks(item)
		require.NoError(t, err)
		assert.Nil(t, blocks[1].MCPToolResult.Error)
		input, err := mcpToolResultToInputItem(blocks[1])
		require.NoError(t, err)
		assert.True(t, param.IsOmitted(input.OfMcpCall.Error))
	}
	b := schema.NewContentBlock(&schema.MCPToolResult{Name: "lookup", ServerLabel: "s", Error: &schema.MCPToolCallError{Message: "legacy"}})
	input, err := mcpToolResultToInputItem(b)
	require.NoError(t, err)
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &wire))
	assert.JSONEq(t, `"legacy"`, string(wire["error"]))
}

func TestFunctionResultCallIDWire(t *testing.T) {
	item, err := functionToolResultToInputItem(&schema.FunctionToolResult{CallID: "call_1", Content: []*schema.FunctionToolResultContentBlock{{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: "ok"}}}})
	require.NoError(t, err)
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &wire))
	assert.JSONEq(t, `"call_1"`, string(wire["call_id"]))
}

func TestMCPErrorReplayAfterStreamingConcat(t *testing.T) {
	var item responses.ResponseOutputItemMcpCall
	rawError := `{"type":"http_error","code":503,"message":"unavailable"}`
	require.NoError(t, json.Unmarshal([]byte(`{"id":"m1","name":"lookup","server_label":"s","error":`+rawError+`}`), &item))
	blocks, err := mcpCallToContentBlocks(item)
	require.NoError(t, err)
	result := blocks[1]
	result.StreamingMeta = &schema.StreamingMeta{Index: 1}
	// A provider may repeat terminal metadata; the concat registration must
	// retain one valid JSON payload rather than concatenate the two strings.
	msg, err := schema.ConcatAgenticMessages([]*schema.AgenticMessage{
		{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: []*schema.ContentBlock{result}},
		{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: []*schema.ContentBlock{result}},
	})
	require.NoError(t, err)
	require.Len(t, msg.ContentBlocks, 1)
	input, err := mcpToolResultToInputItem(msg.ContentBlocks[0])
	require.NoError(t, err)
	raw, err := json.Marshal(input.OfMcpCall.Error)
	require.NoError(t, err)
	assert.JSONEq(t, rawError, string(raw))
}
