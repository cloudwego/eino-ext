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
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type integrationLookupInput struct {
	Key string `json:"key" jsonschema_description:"Lookup key"`
}

func TestGeminiToolCallIDIntegration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")
	if apiKey == "" || modelName == "" {
		t.Skip("set GEMINI_API_KEY and GEMINI_MODEL to run the Gemini integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	require.NoError(t, err)

	newModel := func() *Model {
		m, err := New(ctx, &Config{Client: client, Model: modelName})
		require.NoError(t, err)
		return m
	}
	lookupTool, err := utils.InferTool(
		"lookup_opaque_value",
		"Looks up an opaque value for one key. Each key requires a separate call.",
		func(_ context.Context, input *integrationLookupInput) (map[string]string, error) {
			return map[string]string{"opaque": "unexpected-extra-call-" + input.Key}, nil
		},
	)
	require.NoError(t, err)
	toolInfo, err := lookupTool.Info(ctx)
	require.NoError(t, err)
	tools := []*schema.ToolInfo{toolInfo}

	userMessage := schema.UserAgenticMessage(
		"Call lookup_opaque_value exactly twice in parallel: once with key alpha and once with key beta. Do not answer without both calls.",
	)
	assistantMessage, err := newModel().Generate(ctx, []*schema.AgenticMessage{userMessage}, model.WithTools(tools))
	require.NoError(t, err)

	type call struct {
		id   string
		name string
		key  string
	}
	var calls []call
	for _, block := range assistantMessage.ContentBlocks {
		if block.Type != schema.ContentBlockTypeFunctionToolCall || block.FunctionToolCall == nil {
			continue
		}
		var args struct {
			Key string `json:"key"`
		}
		require.NoError(t, json.Unmarshal([]byte(block.FunctionToolCall.Arguments), &args))
		calls = append(calls, call{
			id:   block.FunctionToolCall.CallID,
			name: block.FunctionToolCall.Name,
			key:  args.Key,
		})
	}
	require.Len(t, calls, 2, "Gemini response: %s", assistantMessage.String())
	require.NotEmpty(t, calls[0].id)
	require.NotEmpty(t, calls[1].id)
	require.NotEqual(t, calls[0].id, calls[1].id)
	require.Equal(t, calls[0].name, calls[1].name)
	require.ElementsMatch(t, []string{"alpha", "beta"}, []string{calls[0].key, calls[1].key})

	opaqueValues := map[string]string{
		"alpha": "opaque-result-alpha-7f3a",
		"beta":  "opaque-result-beta-9c2d",
	}
	resultMessage := func(c call) *schema.AgenticMessage {
		result, err := json.Marshal(map[string]string{"opaque": opaqueValues[c.key]})
		require.NoError(t, err)
		return &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: c.id,
					Name:   c.name,
					Content: []*schema.FunctionToolResultContentBlock{{
						Type: schema.FunctionToolResultContentBlockTypeText,
						Text: &schema.UserInputText{Text: string(result)},
					}},
				}),
			},
		}
	}

	// Persist tool results in completion order, deliberately opposite to call order.
	history := []*schema.AgenticMessage{
		userMessage,
		assistantMessage,
		resultMessage(calls[1]),
		resultMessage(calls[0]),
		schema.UserAgenticMessage(
			"Return one compact JSON object mapping alpha and beta to their opaque values. Do not call any tool.",
		),
	}
	agent, err := adk.NewTypedChatModelAgent(ctx, &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
		Name:        "gemini_tool_call_id_rebuild",
		Instruction: "Respect function call IDs and answer with the exact requested JSON.",
		Model:       newModel(),
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{lookupTool},
			},
		},
	})
	require.NoError(t, err)
	runner := adk.NewTypedRunner(adk.TypedRunnerConfig[*schema.AgenticMessage]{Agent: agent})
	iter := runner.Run(ctx, history)

	var finalMessage *schema.AgenticMessage
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		require.NoError(t, event.Err)
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		require.NoError(t, err)
		if message.Role == schema.AgenticRoleTypeAssistant {
			finalMessage = message
		}
	}
	require.NotNil(t, finalMessage)
	var text strings.Builder
	for _, block := range finalMessage.ContentBlocks {
		if block.Type == schema.ContentBlockTypeAssistantGenText && block.AssistantGenText != nil {
			text.WriteString(block.AssistantGenText.Text)
		}
	}
	answer := text.String()
	start, end := strings.IndexByte(answer, '{'), strings.LastIndexByte(answer, '}')
	require.GreaterOrEqual(t, start, 0, "Gemini response: %s", answer)
	require.Greater(t, end, start, "Gemini response: %s", answer)
	var mapping map[string]string
	require.NoError(t, json.Unmarshal([]byte(answer[start:end+1]), &mapping))
	require.Equal(t, opaqueValues, mapping)
}
