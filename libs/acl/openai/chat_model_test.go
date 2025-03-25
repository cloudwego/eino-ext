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
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestToXXXUtils(t *testing.T) {
	t.Run("toOpenAIMultiContent", func(t *testing.T) {

		multiContents := []schema.ChatMessagePart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "image_desc",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{
					URL:    "https://{RL_ADDRESS}",
					Detail: schema.ImageURLDetailAuto,
				},
			},
		}

		mc, err := toOpenAIMultiContent(multiContents)
		assert.NoError(t, err)
		assert.Len(t, mc, 2)
		assert.Equal(t, openai.ChatCompletionContentPartUnionParam{
			OfText: &openai.ChatCompletionContentPartTextParam{
				Text: "image_desc",
			},
		}, mc[0])

		assert.Equal(t, openai.ChatCompletionContentPartUnionParam{
			OfImageURL: &openai.ChatCompletionContentPartImageParam{
				ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
					URL:    "https://{RL_ADDRESS}",
					Detail: "auto",
				},
			}}, mc[1])

		mc, err = toOpenAIMultiContent(nil)
		assert.Nil(t, err)
		assert.Nil(t, mc)
	})
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.NotNil(t, err)
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestChatCompletion(t *testing.T) {

	ctx := context.Background()

	cli, err := NewClient(ctx, &Config{
		ByAzure:    true,
		BaseURL:    "https://xxxx.com/api",
		APIKey:     "{your-api-key}",
		APIVersion: "2024-06-01",
		Model:      "gpt-4o-2024-05-13",
	})
	assert.NoError(t, err)

	defer mockey.Mock(mockey.GetMethod(cli.cli.Chat.Completions, "New")).Return(
		&openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{
				{
					FinishReason: "stop",
					Message: openai.ChatCompletionMessage{
						Content: "hello world",
						Role:    "assistant",
						ToolCalls: []openai.ChatCompletionMessageToolCall{
							{
								ID: "tool call id",
								Function: openai.ChatCompletionMessageToolCallFunction{
									Arguments: "arguments",
									Name:      "name",
								},
							},
						},
					},
				},
			},
			Usage: openai.CompletionUsage{},
		}, nil).Build().Patch().UnPatch()

	result, err := cli.Generate(ctx, []*schema.Message{schema.UserMessage("hello world")})
	assert.NoError(t, err)
	assert.Equal(t, "hello world", result.Content)
	assert.Equal(t, []schema.ToolCall{
		{
			ID:   "tool call id",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "name",
				Arguments: "arguments",
			},
		},
	}, result.ToolCalls)
}
