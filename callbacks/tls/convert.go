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

package tls

import (
	"context"

	"github.com/bytedance/sonic"
	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

const toolTypeFunction = "function"

func convertModelInput(input *model.CallbackInput) *sem_ai.ModelInput {
	if input == nil {
		return nil
	}

	return &sem_ai.ModelInput{
		Messages: iterSlice(input.Messages, convertModelMessage),
		Tools:    iterSlice(input.Tools, convertTool),
	}
}

func convertModelOutput(output *model.CallbackOutput) *sem_ai.ModelOutput {
	if output == nil {
		return nil
	}
	return &sem_ai.ModelOutput{
		Choices: []*sem_ai.ModelChoice{
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

func convertModelMessage(message *schema.Message) *sem_ai.ModelMessage {
	if message == nil {
		return nil
	}

	msg := &sem_ai.ModelMessage{
		Role:             string(message.Role),
		Content:          message.Content,
		ReasoningContent: message.ReasoningContent,
		Parts:            make([]*sem_ai.ModelMessagePart, len(message.MultiContent)),
		Name:             message.Name,
		ToolCalls:        make([]*sem_ai.ModelToolCall, len(message.ToolCalls)),
		ToolCallID:       message.ToolCallID,
	}

	for i := range message.MultiContent {
		part := message.MultiContent[i]

		msg.Parts[i] = &sem_ai.ModelMessagePart{
			Type: sem_ai.ModelMessagePartType(part.Type),
			Text: part.Text,
		}

		if part.ImageURL != nil {
			msg.Parts[i].ImageURL = &sem_ai.ModelImageURL{
				URL:    part.ImageURL.URL,
				Detail: string(part.ImageURL.Detail),
			}
		}
		if part.FileURL != nil {
			msg.Parts[i].FileURL = &sem_ai.ModelFileURL{
				Name: part.FileURL.Name,
				URL:  part.FileURL.URL,
			}
		}
	}

	for i := range message.ToolCalls {
		tc := message.ToolCalls[i]

		msg.ToolCalls[i] = &sem_ai.ModelToolCall{
			ID:   tc.ID,
			Type: toolTypeFunction,
			Function: &sem_ai.ModelToolCallFunction{
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

func addToolName(ctx context.Context, message *sem_ai.ModelMessage) *sem_ai.ModelMessage {
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

func convertTool(tool *schema.ToolInfo) *sem_ai.ModelTool {
	if tool == nil {
		return nil
	}

	var params []byte
	if raw, err := tool.ToJSONSchema(); err == nil && raw != nil {
		params, _ = raw.MarshalJSON()
	}

	t := &sem_ai.ModelTool{
		Type: toolTypeFunction,
		Function: &sem_ai.ModelToolFunction{
			Name:        tool.Name,
			Description: tool.Desc,
			Parameters:  params,
		},
	}

	return t
}

func convertModelCallOption(config *model.Config) *sem_ai.ModelCallOption {
	if config == nil {
		return nil
	}

	return &sem_ai.ModelCallOption{
		Temperature: config.Temperature,
		MaxTokens:   int64(config.MaxTokens),
		TopP:        config.TopP,
	}
}

// Prompt

func convertPromptInput(input *prompt.CallbackInput) *sem_ai.PromptInput {
	if input == nil {
		return nil
	}

	return &sem_ai.PromptInput{
		Templates: iterSlice(input.Templates, convertTemplate),
		Arguments: convertPromptArguments(input.Variables),
	}
}

func convertPromptOutput(output *prompt.CallbackOutput) *sem_ai.PromptOutput {
	if output == nil {
		return nil
	}

	return &sem_ai.PromptOutput{
		Prompts: iterSlice(output.Result, convertModelMessage),
	}
}

func convertTemplate(template schema.MessagesTemplate) *sem_ai.ModelMessage {
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

func convertPromptArguments(variables map[string]any) []*sem_ai.PromptArgument {
	if variables == nil {
		return nil
	}

	resp := make([]*sem_ai.PromptArgument, 0, len(variables))

	for k := range variables {
		resp = append(resp, &sem_ai.PromptArgument{
			Key:   k,
			Value: variables[k],
			// Source: "",
		})
	}

	return resp
}

// Retriever

func convertRetrieverOutput(output *retriever.CallbackOutput) *sem_ai.RetrieverOutput {
	if output == nil {
		return nil
	}

	return &sem_ai.RetrieverOutput{
		Documents: iterSlice(output.Docs, convertDocument),
	}
}

func convertRetrieverCallOption(input *retriever.CallbackInput) *sem_ai.RetrieverCallOption {
	if input == nil {
		return nil
	}

	opt := &sem_ai.RetrieverCallOption{
		TopK:   int64(input.TopK),
		Filter: input.Filter,
	}

	if input.ScoreThreshold != nil {
		opt.MinScore = input.ScoreThreshold
	}

	return opt
}

func convertDocument(doc *schema.Document) *sem_ai.RetrieverDocument {
	if doc == nil {
		return nil
	}

	return &sem_ai.RetrieverDocument{
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
