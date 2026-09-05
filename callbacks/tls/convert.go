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
		Name:             message.Name,
		ToolCalls:        make([]*sem_ai.ModelToolCall, len(message.ToolCalls)),
		ToolCallID:       message.ToolCallID,
	}

	// MultiContent is retained for older Eino clients.  Newer Eino versions
	// split input and output multimodal parts into dedicated fields; prefer
	// those fields when present so a message is represented exactly once.
	if len(message.UserInputMultiContent) > 0 {
		for _, part := range message.UserInputMultiContent {
			msg.Parts = append(msg.Parts, convertInputMessagePart(part))
		}
	} else if len(message.AssistantGenMultiContent) > 0 {
		for _, part := range message.AssistantGenMultiContent {
			msg.Parts = append(msg.Parts, convertOutputMessagePart(part))
		}
	} else {
		for _, part := range message.MultiContent {
			msg.Parts = append(msg.Parts, convertLegacyMessagePart(part))
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

func convertInputMessagePart(part schema.MessageInputPart) *sem_ai.ModelMessagePart {
	result := &sem_ai.ModelMessagePart{Type: sem_ai.ModelMessagePartType(part.Type), Text: part.Text}
	switch part.Type {
	case schema.ChatMessagePartTypeImageURL:
		if part.Image != nil {
			result.ImageURL = &sem_ai.ModelImageURL{
				URL:      derefString(part.Image.URL),
				Detail:   string(part.Image.Detail),
				MIMEType: part.Image.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio != nil {
			result.AudioURL = &sem_ai.ModelMediaURL{
				URL:      derefString(part.Audio.URL),
				MIMEType: part.Audio.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video != nil {
			result.VideoURL = &sem_ai.ModelMediaURL{
				URL:      derefString(part.Video.URL),
				MIMEType: part.Video.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeFileURL:
		if part.File != nil {
			result.FileURL = &sem_ai.ModelFileURL{
				Name:     part.File.Name,
				URL:      derefString(part.File.URL),
				MIMEType: part.File.MIMEType,
			}
		}
	}
	return result
}

func convertOutputMessagePart(part schema.MessageOutputPart) *sem_ai.ModelMessagePart {
	result := &sem_ai.ModelMessagePart{Type: sem_ai.ModelMessagePartType(part.Type), Text: part.Text}
	switch part.Type {
	case schema.ChatMessagePartTypeImageURL:
		if part.Image != nil {
			result.ImageURL = &sem_ai.ModelImageURL{
				URL:      derefString(part.Image.URL),
				MIMEType: part.Image.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio != nil {
			result.AudioURL = &sem_ai.ModelMediaURL{
				URL:      derefString(part.Audio.URL),
				MIMEType: part.Audio.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video != nil {
			result.VideoURL = &sem_ai.ModelMediaURL{
				URL:      derefString(part.Video.URL),
				MIMEType: part.Video.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeReasoning:
		if part.Reasoning != nil {
			result.Text = part.Reasoning.Text
		}
	}
	return result
}

func convertLegacyMessagePart(part schema.ChatMessagePart) *sem_ai.ModelMessagePart {
	result := &sem_ai.ModelMessagePart{Type: sem_ai.ModelMessagePartType(part.Type), Text: part.Text}
	switch part.Type {
	case schema.ChatMessagePartTypeImageURL:
		if part.ImageURL != nil {
			result.ImageURL = &sem_ai.ModelImageURL{
				URL:      firstNonEmpty(part.ImageURL.URL, part.ImageURL.URI),
				Detail:   string(part.ImageURL.Detail),
				MIMEType: part.ImageURL.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeAudioURL:
		if part.AudioURL != nil {
			result.AudioURL = &sem_ai.ModelMediaURL{
				URL:      firstNonEmpty(part.AudioURL.URL, part.AudioURL.URI),
				MIMEType: part.AudioURL.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeVideoURL:
		if part.VideoURL != nil {
			result.VideoURL = &sem_ai.ModelMediaURL{
				URL:      firstNonEmpty(part.VideoURL.URL, part.VideoURL.URI),
				MIMEType: part.VideoURL.MIMEType,
			}
		}
	case schema.ChatMessagePartTypeFileURL:
		if part.FileURL != nil {
			result.FileURL = &sem_ai.ModelFileURL{
				Name:     part.FileURL.Name,
				URL:      firstNonEmpty(part.FileURL.URL, part.FileURL.URI),
				MIMEType: part.FileURL.MIMEType,
			}
		}
	}
	return result
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
