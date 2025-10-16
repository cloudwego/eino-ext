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

package cozeloop

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const toolTypeFunction = "function"

// ChatModel

func convertModelInput(input *model.CallbackInput) *tracespec.ModelInput {
	return &tracespec.ModelInput{
		Messages: iterSlice(input.Messages, convertModelMessage),
		Tools:    iterSlice(input.Tools, convertTool),
	}
}

func convertModelOutput(output *model.CallbackOutput) *tracespec.ModelOutput {
	if output == nil {
		return nil
	}
	return &tracespec.ModelOutput{
		Choices: []*tracespec.ModelChoice{
			{
				Index:        0,
				FinishReason: getFinishReason(output.Message),
				Message:      convertModelMessage(output.Message)},
		},
	}
}

func getFinishReason(msg *schema.Message) string {
	if msg == nil || msg.ResponseMeta == nil {
		return ""
	}

	return msg.ResponseMeta.FinishReason
}

func convertModelMessage(message *schema.Message) *tracespec.ModelMessage {
	if message == nil {
		return nil
	}

	msg := &tracespec.ModelMessage{
		Role:             string(message.Role),
		Content:          message.Content,
		Parts:            make([]*tracespec.ModelMessagePart, len(message.MultiContent)),
		Name:             message.Name,
		ToolCalls:        make([]*tracespec.ModelToolCall, len(message.ToolCalls)),
		ToolCallID:       message.ToolCallID,
		ReasoningContent: message.ReasoningContent,
	}

	for i := range message.MultiContent {
		part := message.MultiContent[i]

		msg.Parts[i] = &tracespec.ModelMessagePart{
			Type: tracespec.ModelMessagePartType(part.Type),
			Text: part.Text,
		}

		if part.ImageURL != nil {
			msg.Parts[i].ImageURL = &tracespec.ModelImageURL{
				URL:    part.ImageURL.URL,
				Detail: string(part.ImageURL.Detail),
			}
		}
	}

	for i := range message.ToolCalls {
		tc := message.ToolCalls[i]

		msg.ToolCalls[i] = &tracespec.ModelToolCall{
			ID:   tc.ID,
			Type: toolTypeFunction,
			Function: &tracespec.ModelToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}

	if message.Extra != nil {
		msg.Metadata = make(map[string]string, len(message.Extra))
		for k, v := range message.Extra {
			if sv, err := sonic.MarshalString(v); err == nil {
				msg.Metadata[k] = sv
			}
		}
	}

	return msg
}

func addToolName(ctx context.Context, message *tracespec.ModelMessage) *tracespec.ModelMessage {
	if message == nil {
		return message
	}

	toolIDNameMap := getToolIDNameMapFromCtx(ctx)
	if toolIDNameMap == nil {
		return message
	}
	toolName, ok := toolIDNameMap[message.ToolCallID]
	if !ok {
		return message
	}

	message.Name = toolName
	return message
}

func convertTool(tool *schema.ToolInfo) *tracespec.ModelTool {
	if tool == nil {
		return nil
	}

	var params []byte
	if raw, err := tool.ToJSONSchema(); err == nil && raw != nil {
		params, _ = raw.MarshalJSON()
	}

	t := &tracespec.ModelTool{
		Type: toolTypeFunction,
		Function: &tracespec.ModelToolFunction{
			Name:        tool.Name,
			Description: tool.Desc,
			Parameters:  params,
		},
	}

	return t
}

func convertModelCallOption(config *model.Config) *tracespec.ModelCallOption {
	if config == nil {
		return nil
	}

	return &tracespec.ModelCallOption{
		Temperature: config.Temperature,
		MaxTokens:   int64(config.MaxTokens),
		TopP:        config.TopP,
	}
}

// Prompt

func convertPromptInput(input *prompt.CallbackInput) *tracespec.PromptInput {
	if input == nil {
		return nil
	}

	return &tracespec.PromptInput{
		Templates: iterSlice(input.Templates, convertTemplate),
		Arguments: convertPromptArguments(input.Variables),
	}
}

func convertPromptOutput(output *prompt.CallbackOutput) *tracespec.PromptOutput {
	if output == nil {
		return nil
	}

	return &tracespec.PromptOutput{
		Prompts: iterSlice(output.Result, convertModelMessage),
	}
}

func convertTemplate(template schema.MessagesTemplate) *tracespec.ModelMessage {
	if template == nil {
		return nil
	}

	switch t := template.(type) {
	case *schema.Message:
		return convertModelMessage(t)
	default: // messagePlaceholder etc.
		return nil
	}
}

func convertPromptArguments(variables map[string]any) []*tracespec.PromptArgument {
	if variables == nil {
		return nil
	}

	resp := make([]*tracespec.PromptArgument, 0, len(variables))

	for k := range variables {
		resp = append(resp, &tracespec.PromptArgument{
			Key:   k,
			Value: variables[k],
			// Source: "",
		})
	}

	return resp
}

// Retriever

func convertRetrieverOutput(output *retriever.CallbackOutput) *tracespec.RetrieverOutput {
	if output == nil {
		return nil
	}

	return &tracespec.RetrieverOutput{
		Documents: iterSlice(output.Docs, convertDocument),
	}
}

func convertRetrieverCallOption(input *retriever.CallbackInput) *tracespec.RetrieverCallOption {
	if input == nil {
		return nil
	}

	opt := &tracespec.RetrieverCallOption{
		TopK:   int64(input.TopK),
		Filter: input.Filter,
	}

	if input.ScoreThreshold != nil {
		opt.MinScore = input.ScoreThreshold
	}

	return opt
}

func convertDocument(doc *schema.Document) *tracespec.RetrieverDocument {
	if doc == nil {
		return nil
	}

	return &tracespec.RetrieverDocument{
		ID:      doc.ID,
		Content: doc.Content,
		Score:   doc.Score(),
		// Index:   "",
		Vector: doc.DenseVector(),
	}
}

func iterSlice[A, B any](sa []A, fb func(a A) B) []B {
	r := make([]B, len(sa))
	for i := range sa {
		r[i] = fb(sa[i])
	}

	return r
}

func iterSliceWithCtx[A, B any](ctx context.Context, sa []A, fb func(ctx context.Context, a A) B) []B {
	r := make([]B, len(sa))
	for i := range sa {
		r[i] = fb(ctx, sa[i])
	}

	return r
}
