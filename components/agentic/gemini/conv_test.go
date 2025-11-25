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

package gemini

import (
	"encoding/base64"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestConvAgenticMessage_Text(t *testing.T) {
	g := &gemini{}

	tests := []struct {
		name     string
		message  *schema.AgenticMessage
		validate func(t *testing.T, content *genai.Content, err error)
	}{
		{
			name: "user input text",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeUserInputText,
						UserInputText: &schema.UserInputText{
							Text: "Hello, world!",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Equal(t, roleUser, content.Role)
				assert.Len(t, content.Parts, 1)
				assert.Equal(t, "Hello, world!", content.Parts[0].Text)
			},
		},
		{
			name: "assistant generated text",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeAssistantGenText,
						AssistantGenText: &schema.AssistantGenText{
							Text: "I can help you with that.",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Equal(t, roleModel, content.Role)
				assert.Len(t, content.Parts, 1)
				assert.Equal(t, "I can help you with that.", content.Parts[0].Text)
			},
		},
		{
			name: "reasoning block",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeReasoning,
						Reasoning: &schema.Reasoning{
							Summary: []*schema.ReasoningSummary{
								{Text: "First, I need to "},
								{Text: "analyze the problem."},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.Equal(t, "First, I need to analyze the problem.", content.Parts[0].Text)
				assert.True(t, content.Parts[0].Thought)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := g.convAgenticMessage(tt.message)
			tt.validate(t, content, err)
		})
	}
}

func TestConvAgenticMessage_Multimedia(t *testing.T) {
	g := &gemini{}
	testImageData := []byte("fake-image-data")
	testImageB64 := base64.StdEncoding.EncodeToString(testImageData)

	tests := []struct {
		name     string
		message  *schema.AgenticMessage
		validate func(t *testing.T, content *genai.Content, err error)
	}{
		{
			name: "image with URL",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeUserInputImage,
						UserInputImage: &schema.UserInputImage{
							URL:      "https://example.com/image.jpg",
							MIMEType: "image/jpeg",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FileData)
				assert.Equal(t, "https://example.com/image.jpg", content.Parts[0].FileData.FileURI)
				assert.Equal(t, "image/jpeg", content.Parts[0].FileData.MIMEType)
			},
		},
		{
			name: "image with base64 data",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeUserInputImage,
						UserInputImage: &schema.UserInputImage{
							Base64Data: testImageB64,
							MIMEType:   "image/png",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].InlineData)
				assert.Equal(t, testImageData, content.Parts[0].InlineData.Data)
				assert.Equal(t, "image/png", content.Parts[0].InlineData.MIMEType)
			},
		},
		{
			name: "audio with URL",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeUserInputAudio,
						UserInputAudio: &schema.UserInputAudio{
							URL:      "https://example.com/audio.mp3",
							MIMEType: "audio/mpeg",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FileData)
				assert.Equal(t, "https://example.com/audio.mp3", content.Parts[0].FileData.FileURI)
			},
		},
		{
			name: "video with base64 data",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeUserInputVideo,
						UserInputVideo: &schema.UserInputVideo{
							Base64Data: testImageB64,
							MIMEType:   "video/mp4",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].InlineData)
				assert.Equal(t, "video/mp4", content.Parts[0].InlineData.MIMEType)
			},
		},
		{
			name: "assistant generated image",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeAssistantGenImage,
						AssistantGenImage: &schema.AssistantGenImage{
							URL:      "https://example.com/generated.jpg",
							MIMEType: "image/jpeg",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FileData)
			},
		},
		{
			name: "assistant generated audio",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeAssistantGenAudio,
						AssistantGenAudio: &schema.AssistantGenAudio{
							URL:      "https://example.com/generated.jpg",
							MIMEType: "audio/mpeg",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FileData)
			},
		},
		{
			name: "assistant generated video",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeAssistantGenVideo,
						AssistantGenVideo: &schema.AssistantGenVideo{
							URL:      "https://example.com/generated.jpg",
							MIMEType: "video/mp4",
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FileData)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := g.convAgenticMessage(tt.message)
			tt.validate(t, content, err)
		})
	}
}

func TestConvAgenticMessage_Tools(t *testing.T) {
	g := &gemini{}

	tests := []struct {
		name     string
		message  *schema.AgenticMessage
		validate func(t *testing.T, content *genai.Content, err error)
	}{
		{
			name: "function tool call",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeFunctionToolCall,
						FunctionToolCall: &schema.FunctionToolCall{
							Name:      "get_weather",
							Arguments: `{"location":"San Francisco"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FunctionCall)
				assert.Equal(t, "get_weather", content.Parts[0].FunctionCall.Name)
				assert.Equal(t, "San Francisco", content.Parts[0].FunctionCall.Args["location"])
			},
		},
		{
			name: "function tool result",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeFunctionToolResult,
						FunctionToolResult: &schema.FunctionToolResult{
							Name:   "get_weather",
							Result: `{"temperature":72,"condition":"sunny"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 1)
				assert.NotNil(t, content.Parts[0].FunctionResponse)
				assert.Equal(t, "get_weather", content.Parts[0].FunctionResponse.Name)
				assert.Equal(t, float64(72), content.Parts[0].FunctionResponse.Response["temperature"])
			},
		},
		{
			name: "server tool call - code execution",
			message: &schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					{
						Type: schema.ContentBlockTypeServerToolCall,
						ServerToolCall: &schema.ServerToolCall{
							Name: ServerToolNameCodeExecution,
							Arguments: &ExecutableCode{
								Code:     "print('hello')",
								Language: LanguagePython,
							},
						},
					},
					{
						Type: schema.ContentBlockTypeServerToolResult,
						ServerToolResult: &schema.ServerToolResult{
							Name: ServerToolNameCodeExecution,
							Result: &CodeExecutionResult{
								Outcome: OutcomeOK,
								Output:  "output",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, content *genai.Content, err error) {
				assert.NoError(t, err)
				assert.Len(t, content.Parts, 2)
				assert.NotNil(t, content.Parts[0].ExecutableCode)
				assert.Equal(t, "print('hello')", content.Parts[0].ExecutableCode.Code)
				assert.Equal(t, genai.Language(LanguagePython), content.Parts[0].ExecutableCode.Language)
				assert.NotNil(t, content.Parts[1].CodeExecutionResult)
				assert.Equal(t, content.Parts[1].CodeExecutionResult.Outcome, genai.OutcomeOK)
				assert.Equal(t, content.Parts[1].CodeExecutionResult.Output, "output")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := g.convAgenticMessage(tt.message)
			tt.validate(t, content, err)
		})
	}
}

func TestConvAgenticMessage_ThoughtSignature(t *testing.T) {
	g := &gemini{}
	thoughtSig := []byte("test-signature")

	cb := schema.NewContentBlock(&schema.AssistantGenText{
		Text: "Response text",
	})
	setThoughtSignature(cb, thoughtSig)

	message := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			cb,
		},
	}

	content, err := g.convAgenticMessage(message)
	assert.NoError(t, err)
	assert.Len(t, content.Parts, 1)
	assert.Equal(t, thoughtSig, content.Parts[0].ThoughtSignature)
}

func TestConvAgenticCandidate_Text(t *testing.T) {
	tests := []struct {
		name      string
		candidate *genai.Candidate
		lastType  schema.ContentBlockType
		validate  func(t *testing.T, message *schema.AgenticMessage, err error)
	}{
		{
			name: "normal text",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{Text: "Hello from Gemini"},
					},
				},
				FinishReason: genai.FinishReasonStop,
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Equal(t, schema.AgenticRoleTypeAssistant, message.Role)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenText, message.ContentBlocks[0].Type)
				assert.Equal(t, "Hello from Gemini", message.ContentBlocks[0].AssistantGenText.Text)
			},
		},
		{
			name: "thought text",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{Text: "Let me think...", Thought: true},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeReasoning, message.ContentBlocks[0].Type)
				assert.Equal(t, "Let me think...", message.ContentBlocks[0].Reasoning.Summary[0].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := convAgenticCandidate(tt.candidate, tt.lastType)
			tt.validate(t, message, err)
		})
	}
}

func TestConvAgenticCandidate_Multimedia(t *testing.T) {
	testImageData := []byte("fake-image-data")

	tests := []struct {
		name      string
		candidate *genai.Candidate
		validate  func(t *testing.T, message *schema.AgenticMessage, err error)
	}{
		{
			name: "inline image data",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								MIMEType: "image/png",
								Data:     testImageData,
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenImage, message.ContentBlocks[0].Type)
				// Verify base64 encoding
				expectedB64 := base64.StdEncoding.EncodeToString(testImageData)
				assert.Equal(t, expectedB64, message.ContentBlocks[0].AssistantGenImage.Base64Data)
				assert.Equal(t, "image/png", message.ContentBlocks[0].AssistantGenImage.MIMEType)
			},
		},
		{
			name: "file image data",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							FileData: &genai.FileData{
								MIMEType: "image/png",
								FileURI:  "url",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenImage, message.ContentBlocks[0].Type)
				assert.Equal(t, "url", message.ContentBlocks[0].AssistantGenImage.URL)
				assert.Equal(t, "image/png", message.ContentBlocks[0].AssistantGenImage.MIMEType)
			},
		},
		{
			name: "inline audio data",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								MIMEType: "audio/mpeg",
								Data:     testImageData,
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenAudio, message.ContentBlocks[0].Type)
				// Verify base64 encoding
				expectedB64 := base64.StdEncoding.EncodeToString(testImageData)
				assert.Equal(t, expectedB64, message.ContentBlocks[0].AssistantGenAudio.Base64Data)
				assert.Equal(t, "audio/mpeg", message.ContentBlocks[0].AssistantGenAudio.MIMEType)
			},
		},
		{
			name: "file data audio",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							FileData: &genai.FileData{
								MIMEType: "audio/mpeg",
								FileURI:  "https://example.com/audio.mp3",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenAudio, message.ContentBlocks[0].Type)
				assert.Equal(t, "https://example.com/audio.mp3", message.ContentBlocks[0].AssistantGenAudio.URL)
			},
		},
		{
			name: "inline video data",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							InlineData: &genai.Blob{
								MIMEType: "video/mp4",
								Data:     testImageData,
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenVideo, message.ContentBlocks[0].Type)
			},
		},
		{
			name: "file video data",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							FileData: &genai.FileData{
								MIMEType: "video/mp4",
								FileURI:  "url",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeAssistantGenVideo, message.ContentBlocks[0].Type)
				assert.Equal(t, "url", message.ContentBlocks[0].AssistantGenVideo.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := convAgenticCandidate(tt.candidate, "")
			tt.validate(t, message, err)
		})
	}
}

func TestConvAgenticCandidate_Tools(t *testing.T) {
	tests := []struct {
		name      string
		candidate *genai.Candidate
		validate  func(t *testing.T, message *schema.AgenticMessage, err error)
	}{
		{
			name: "function call",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							FunctionCall: &genai.FunctionCall{
								Name: "search",
								Args: map[string]any{
									"query": "golang",
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, message.ContentBlocks[0].Type)
				assert.Equal(t, "search", message.ContentBlocks[0].FunctionToolCall.Name)

				var args map[string]any
				err = sonic.UnmarshalString(message.ContentBlocks[0].FunctionToolCall.Arguments, &args)
				assert.NoError(t, err)
				assert.Equal(t, "golang", args["query"])
			},
		},
		{
			name: "code execution call",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							ExecutableCode: &genai.ExecutableCode{
								Code:     "x = 1 + 1",
								Language: genai.LanguagePython,
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeServerToolCall, message.ContentBlocks[0].Type)
				assert.Equal(t, ServerToolNameCodeExecution, message.ContentBlocks[0].ServerToolCall.Name)

				execCode, ok := message.ContentBlocks[0].ServerToolCall.Arguments.(*ExecutableCode)
				assert.True(t, ok)
				assert.Equal(t, "x = 1 + 1", execCode.Code)
				assert.Equal(t, LanguagePython, execCode.Language)
			},
		},
		{
			name: "code execution result",
			candidate: &genai.Candidate{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							CodeExecutionResult: &genai.CodeExecutionResult{
								Outcome: genai.OutcomeOK,
								Output:  "2",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.Len(t, message.ContentBlocks, 1)
				assert.Equal(t, schema.ContentBlockTypeServerToolResult, message.ContentBlocks[0].Type)
				assert.Equal(t, ServerToolNameCodeExecution, message.ContentBlocks[0].ServerToolResult.Name)

				result, ok := message.ContentBlocks[0].ServerToolResult.Result.(*CodeExecutionResult)
				assert.True(t, ok)
				assert.Equal(t, OutcomeOK, result.Outcome)
				assert.Equal(t, "2", result.Output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := convAgenticCandidate(tt.candidate, "")
			tt.validate(t, message, err)
		})
	}
}

func TestConvAgenticCandidate_ThoughtSignature(t *testing.T) {
	thoughtSig := []byte("signature-data")

	candidate := &genai.Candidate{
		Content: &genai.Content{
			Role: roleModel,
			Parts: []*genai.Part{
				{
					Text:             "Response",
					ThoughtSignature: thoughtSig,
				},
			},
		},
	}

	message, err := convAgenticCandidate(candidate, "")
	assert.NoError(t, err)
	assert.Len(t, message.ContentBlocks, 1)

	// Verify thought signature is stored in extra
	retrievedSig := getThoughtSignature(message.ContentBlocks[0])
	assert.Equal(t, thoughtSig, retrievedSig)
}

func TestConvAgenticResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *genai.GenerateContentResponse
		validate func(t *testing.T, message *schema.AgenticMessage, err error)
	}{
		{
			name: "with token usage",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Role: roleModel,
							Parts: []*genai.Part{
								{Text: "Response"},
							},
						},
						FinishReason: genai.FinishReasonStop,
					},
				},
				UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
					PromptTokenCount:        10,
					CandidatesTokenCount:    20,
					TotalTokenCount:         30,
					CachedContentTokenCount: 5,
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, message.ResponseMeta)
				assert.NotNil(t, message.ResponseMeta.TokenUsage)
				assert.Equal(t, 10, message.ResponseMeta.TokenUsage.PromptTokens)
				assert.Equal(t, 20, message.ResponseMeta.TokenUsage.CompletionTokens)
				assert.Equal(t, 30, message.ResponseMeta.TokenUsage.TotalTokens)
				assert.Equal(t, 5, message.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens)
			},
		},
		{
			name: "with finish reason",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Role:  roleModel,
							Parts: []*genai.Part{{Text: "Done"}},
						},
						FinishReason: genai.FinishReasonMaxTokens,
					},
				},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, message.ResponseMeta)
				assert.NotNil(t, message.ResponseMeta.GeminiExtension)
				ext := message.ResponseMeta.GeminiExtension
				assert.Equal(t, string(genai.FinishReasonMaxTokens), ext.FinishReason)
			},
		},
		{
			name: "empty response",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			},
			validate: func(t *testing.T, message *schema.AgenticMessage, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "gemini result is empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := convAgenticResponse(tt.response, "")
			tt.validate(t, message, err)
		})
	}
}

func TestConversionRoundTrip(t *testing.T) {
	g := &gemini{}

	// Create a comprehensive message
	originalMessage := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			{
				Type: schema.ContentBlockTypeAssistantGenText,
				AssistantGenText: &schema.AssistantGenText{
					Text: "Hello",
				},
			},
			{
				Type: schema.ContentBlockTypeFunctionToolCall,
				FunctionToolCall: &schema.FunctionToolCall{
					Name:      "test_func",
					Arguments: `{"key":"value"}`,
				},
			},
		},
	}

	// Convert to genai.Content
	content, err := g.convAgenticMessage(originalMessage)
	assert.NoError(t, err)

	// Convert back via candidate
	candidate := &genai.Candidate{
		Content:      content,
		FinishReason: genai.FinishReasonStop,
	}

	resultMessage, err := convAgenticCandidate(candidate, "")
	assert.NoError(t, err)

	// Verify basic structure
	assert.Equal(t, originalMessage.Role, resultMessage.Role)
	assert.Len(t, resultMessage.ContentBlocks, 2)
	assert.Equal(t, schema.ContentBlockTypeAssistantGenText, resultMessage.ContentBlocks[0].Type)
	assert.Equal(t, "Hello", resultMessage.ContentBlocks[0].AssistantGenText.Text)
	assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, resultMessage.ContentBlocks[1].Type)
	assert.Equal(t, "test_func", resultMessage.ContentBlocks[1].FunctionToolCall.Name)
}

func TestCreateContentBlockFromType(t *testing.T) {
	result := createContentBlockFromType(schema.ContentBlockTypeReasoning)
	assert.NotNil(t, result.Reasoning)
	result = createContentBlockFromType(schema.ContentBlockTypeUserInputText)
	assert.NotNil(t, result.UserInputText)
	result = createContentBlockFromType(schema.ContentBlockTypeUserInputImage)
	assert.NotNil(t, result.UserInputImage)
	result = createContentBlockFromType(schema.ContentBlockTypeUserInputAudio)
	assert.NotNil(t, result.UserInputAudio)
	result = createContentBlockFromType(schema.ContentBlockTypeUserInputVideo)
	assert.NotNil(t, result.UserInputVideo)
	result = createContentBlockFromType(schema.ContentBlockTypeUserInputFile)
	assert.NotNil(t, result.UserInputFile)
	result = createContentBlockFromType(schema.ContentBlockTypeAssistantGenText)
	assert.NotNil(t, result.AssistantGenText)
	result = createContentBlockFromType(schema.ContentBlockTypeAssistantGenImage)
	assert.NotNil(t, result.AssistantGenImage)
	result = createContentBlockFromType(schema.ContentBlockTypeAssistantGenAudio)
	assert.NotNil(t, result.AssistantGenAudio)
	result = createContentBlockFromType(schema.ContentBlockTypeAssistantGenVideo)
	assert.NotNil(t, result.AssistantGenVideo)
	result = createContentBlockFromType(schema.ContentBlockTypeFunctionToolCall)
	assert.NotNil(t, result.FunctionToolCall)
	result = createContentBlockFromType(schema.ContentBlockTypeFunctionToolResult)
	assert.NotNil(t, result.FunctionToolResult)
	result = createContentBlockFromType(schema.ContentBlockTypeServerToolCall)
	assert.NotNil(t, result.ServerToolCall)
	result = createContentBlockFromType(schema.ContentBlockTypeServerToolResult)
	assert.NotNil(t, result.ServerToolResult)
	result = createContentBlockFromType(schema.ContentBlockTypeMCPToolCall)
	assert.NotNil(t, result.MCPToolCall)
	result = createContentBlockFromType(schema.ContentBlockTypeMCPToolResult)
	assert.NotNil(t, result.MCPToolResult)
	result = createContentBlockFromType(schema.ContentBlockTypeMCPListTools)
	assert.NotNil(t, result.MCPListToolsResult)
	result = createContentBlockFromType(schema.ContentBlockTypeMCPToolApprovalRequest)
	assert.NotNil(t, result.MCPToolApprovalRequest)
	result = createContentBlockFromType(schema.ContentBlockTypeMCPToolApprovalResponse)
	assert.NotNil(t, result.MCPToolApprovalResponse)
}
