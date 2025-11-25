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
	"io"
	"iter"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestNewAgenticModel(t *testing.T) {
	config := &Config{
		Client:              &genai.Client{},
		Model:               "gemini-pro",
		MaxTokens:           ptrOf(100),
		Temperature:         ptrOf(float32(0.7)),
		TopP:                ptrOf(float32(0.9)),
		TopK:                ptrOf(int32(40)),
		EnableCodeExecution: &genai.ToolCodeExecution{},
		SafetySettings: []*genai.SafetySetting{
			{
				Category:  genai.HarmCategoryHarassment,
				Threshold: genai.HarmBlockThresholdBlockMediumAndAbove,
			},
		},
		ResponseModalities: []ResponseModality{ResponseModalityText, ResponseModalityImage},
		Cache: &CacheConfig{
			TTL: time.Hour,
		},
	}
	model, err := NewAgenticModel(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "gemini-pro", model.model)
	assert.Equal(t, 100, *model.maxTokens)
	assert.Equal(t, float32(0.7), *model.temperature)
	assert.Equal(t, float32(0.9), *model.topP)
	assert.Equal(t, int32(40), *model.topK)
	assert.NotNil(t, model.enableCodeExecution)
	assert.Len(t, model.safetySettings, 1)
	assert.Len(t, model.responseModalities, 2)
	assert.NotNil(t, model.cache)
	assert.Equal(t, time.Hour, model.cache.TTL)

	assert.Equal(t, "Gemini", model.GetType())
	assert.True(t, model.IsCallbacksEnabled())

	tm, err := model.WithTools([]*schema.ToolInfo{{}})
	assert.NoError(t, err)
	model = tm.(*Gemini)
	assert.Len(t, model.tools, 1)
	assert.Len(t, model.origTools, 1)
	assert.NotNil(t, model.toolChoice)
	assert.Equal(t, schema.ToolChoiceAllowed, model.toolChoice.Type)
}

func TestGemini_GenInputAndConf(t *testing.T) {
	g := &Gemini{
		cli:         &genai.Client{},
		model:       "gemini-pro",
		temperature: ptrOf(float32(0.5)),
		topP:        ptrOf(float32(0.9)),
	}

	input := []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				{
					Type: schema.ContentBlockTypeUserInputText,
					UserInputText: &schema.UserInputText{
						Text: "Hello",
					},
				},
			},
		},
	}

	newTemp := float32(0.9)
	newModel := "gemini-1.5-flash"

	modelName, _, genaiConf, conf, err := g.genInputAndConf(
		input,
		model.WithTemperature(newTemp),
		model.WithModel(newModel),
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type:   schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{Tools: []*schema.AllowedTool{{FunctionName: "tool1"}}},
		}),
		model.WithTools([]*schema.ToolInfo{{}}),
	)

	assert.NoError(t, err)
	assert.Equal(t, newModel, modelName)
	assert.Equal(t, newTemp, *genaiConf.Temperature)
	assert.Equal(t, newTemp, conf.Temperature)
	assert.Equal(t, newModel, conf.Model)
	assert.Equal(t, genai.FunctionCallingConfigModeAny, genaiConf.ToolConfig.FunctionCallingConfig.Mode)
	assert.Equal(t, []string{"tool1"}, genaiConf.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
}

func TestGemini_Generate_Success(t *testing.T) {
	defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{Text: "Hello! How can I help you?"},
					},
				},
				FinishReason: genai.FinishReasonStop,
			},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}, nil).Build().UnPatch()

	g := &Gemini{
		cli:   &genai.Client{Models: &genai.Models{}},
		model: "gemini-pro",
	}

	input := []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				{
					Type: schema.ContentBlockTypeUserInputText,
					UserInputText: &schema.UserInputText{
						Text: "Hello",
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := g.Generate(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, schema.AgenticRoleTypeAssistant, result.Role)
	assert.Len(t, result.ContentBlocks, 1)
	assert.Equal(t, schema.ContentBlockTypeAssistantGenText, result.ContentBlocks[0].Type)
	assert.Equal(t, "Hello! How can I help you?", result.ContentBlocks[0].AssistantGenText.Text)

	// Check token usage
	assert.NotNil(t, result.ResponseMeta)
	assert.NotNil(t, result.ResponseMeta.TokenUsage)
	assert.Equal(t, 10, result.ResponseMeta.TokenUsage.PromptTokens)
	assert.Equal(t, 20, result.ResponseMeta.TokenUsage.CompletionTokens)
	assert.Equal(t, 30, result.ResponseMeta.TokenUsage.TotalTokens)
}

func TestGemini_Generate_WithFunctionCall(t *testing.T) {
	defer mockey.Mock(genai.Models.GenerateContent).Return(&genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: roleModel,
					Parts: []*genai.Part{
						{
							FunctionCall: &genai.FunctionCall{
								Name: "get_weather",
								Args: map[string]any{
									"location": "San Francisco",
								},
							},
						},
					},
				},
				FinishReason: genai.FinishReasonStop,
			},
		},
	}, nil).Build().UnPatch()

	g := &Gemini{
		cli:   &genai.Client{Models: &genai.Models{}},
		model: "gemini-pro",
	}

	input := []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				{
					Type: schema.ContentBlockTypeUserInputText,
					UserInputText: &schema.UserInputText{
						Text: "What's the weather in San Francisco?",
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := g.Generate(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.ContentBlocks, 1)
	assert.Equal(t, schema.ContentBlockTypeFunctionToolCall, result.ContentBlocks[0].Type)
	assert.Equal(t, "get_weather", result.ContentBlocks[0].FunctionToolCall.Name)
}

func TestGemini_Stream_Success(t *testing.T) {
	// Create a mock stream iterator
	mockIterator := func(yield func(*genai.GenerateContentResponse, error) bool) {
		// First chunk
		if !yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: roleModel,
						Parts: []*genai.Part{
							{Text: "Hello"},
						},
					},
				},
			},
		}, nil) {
			return
		}

		// Second chunk
		if !yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: roleModel,
						Parts: []*genai.Part{
							{Text: " world"},
						},
					},
				},
			},
		}, nil) {
			return
		}

		// Third chunk with finish reason
		yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: roleModel,
						Parts: []*genai.Part{
							{Text: "!"},
						},
					},
					FinishReason: genai.FinishReasonStop,
				},
			},
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     5,
				CandidatesTokenCount: 10,
				TotalTokenCount:      15,
			},
		}, nil)
	}

	defer mockey.Mock(genai.Models.GenerateContentStream).Return(
		iter.Seq2[*genai.GenerateContentResponse, error](mockIterator),
	).Build().UnPatch()

	g := &Gemini{
		cli:   &genai.Client{Models: &genai.Models{}},
		model: "gemini-pro",
	}

	input := []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				{
					Type: schema.ContentBlockTypeUserInputText,
					UserInputText: &schema.UserInputText{
						Text: "Hello",
					},
				},
			},
		},
	}

	ctx := context.Background()
	streamReader, err := g.Stream(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, streamReader)

	// Read all chunks
	var chunks []*schema.AgenticMessage
	for {
		chunk, err := streamReader.Recv()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Should have 3 chunks
	assert.Len(t, chunks, 3)

	// Check first chunk
	assert.Equal(t, "Hello", chunks[0].ContentBlocks[0].AssistantGenText.Text)
	assert.NotNil(t, chunks[0].ContentBlocks[0].StreamingMeta)
	assert.Equal(t, 0, chunks[0].ContentBlocks[0].StreamingMeta.Index)

	// Check second chunk
	assert.Equal(t, " world", chunks[1].ContentBlocks[0].AssistantGenText.Text)

	// Check third chunk (has finish reason and usage metadata)
	assert.Equal(t, "!", chunks[2].ContentBlocks[0].AssistantGenText.Text)
	assert.NotNil(t, chunks[2].ResponseMeta)
	assert.NotNil(t, chunks[2].ResponseMeta.TokenUsage)
	assert.Equal(t, 15, chunks[2].ResponseMeta.TokenUsage.TotalTokens)

	assert.Equal(t, string(genai.FinishReasonStop), chunks[2].ResponseMeta.GeminiExtension.FinishReason)
}

func TestConvCallbackOutput(t *testing.T) {
	o := convCallbackOutput(&schema.AgenticMessage{}, &model.AgenticConfig{})
	assert.NotNil(t, o.Message)
	assert.NotNil(t, o.Config)
}

func TestPrefixCache(t *testing.T) {
	g := &Gemini{cli: &genai.Client{Caches: &genai.Caches{}}}

	defer mockey.Mock(genai.Caches.Create).Return(&genai.CachedContent{
		Name: "name",
	}, nil).Build().UnPatch()

	ctx := context.Background()

	ret, err := g.CreatePrefixCache(ctx, []*schema.AgenticMessage{{}})
	assert.NoError(t, err)
	assert.Equal(t, "name", ret.Name)
}

func ptrOf[T any](v T) *T {
	return &v
}
