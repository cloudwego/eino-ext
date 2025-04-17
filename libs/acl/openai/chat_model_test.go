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
	"math/rand"
	"testing"

	goopenai "github.com/meguminnnnnnnnn/go-openai"
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
					URL:    "test_url",
					Detail: schema.ImageURLDetailAuto,
				},
			},
			{
				Type: schema.ChatMessagePartTypeAudioURL,
				AudioURL: &schema.ChatMessageAudioURL{
					URL:      "test_url",
					MIMEType: "mp3",
				},
			},
			{
				Type: schema.ChatMessagePartTypeVideoURL,
				VideoURL: &schema.ChatMessageVideoURL{
					URL: "test_url",
				},
			},
		}

		mc, err := toOpenAIMultiContent(multiContents)
		assert.NoError(t, err)
		assert.Len(t, mc, 4)
		assert.Equal(t, mc[0], goopenai.ChatMessagePart{
			Type: goopenai.ChatMessagePartTypeText,
			Text: "image_desc",
		})

		assert.Equal(t, mc[1], goopenai.ChatMessagePart{
			Type: goopenai.ChatMessagePartTypeImageURL,
			ImageURL: &goopenai.ChatMessageImageURL{
				URL:    "test_url",
				Detail: goopenai.ImageURLDetailAuto,
			},
		})

		assert.Equal(t, mc[2], goopenai.ChatMessagePart{
			Type: goopenai.ChatMessagePartTypeInputAudio,
			InputAudio: &goopenai.ChatMessageInputAudio{
				Data:   "test_url",
				Format: "mp3",
			},
		})
		assert.Equal(t, mc[3], goopenai.ChatMessagePart{
			Type: goopenai.ChatMessagePartTypeVideoURL,
			VideoURL: &goopenai.ChatMessageVideoURL{
				URL: "test_url",
			},
		})

		mc, err = toOpenAIMultiContent(nil)
		assert.Nil(t, err)
		assert.Nil(t, mc)
	})
}

func TestToOpenAIToolCalls(t *testing.T) {
	t.Run("empty tools", func(t *testing.T) {
		tools := toOpenAIToolCalls([]schema.ToolCall{})
		assert.Len(t, tools, 0)
	})

	t.Run("normal tools", func(t *testing.T) {
		fakeToolCall1 := schema.ToolCall{
			ID:       randStr(),
			Function: schema.FunctionCall{Name: randStr(), Arguments: randStr()},
		}

		toolCalls := toOpenAIToolCalls([]schema.ToolCall{fakeToolCall1})

		assert.Len(t, toolCalls, 1)
		assert.Equal(t, fakeToolCall1.ID, toolCalls[0].ID)
		assert.Equal(t, fakeToolCall1.Function.Name, toolCalls[0].Function.Name)
	})
}

func randStr() string {
	seeds := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, 8)
	for i := range b {
		b[i] = seeds[rand.Intn(len(seeds))]
	}
	return string(b)
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("info", []byte("stack"))
	assert.Equal(t, "panic error: info, \nstack: stack", err.Error())
}

func TestWithTools(t *testing.T) {
	cm := &Client{config: &Config{Model: "test model"}}
	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)
	assert.Equal(t, "test model", ncm.(*Client).config.Model)
	assert.Equal(t, "test tool name", ncm.(*Client).rawTools[0].Name)
}

func TestLogProbs(t *testing.T) {
	assert.Equal(t, &schema.LogProbs{Content: []schema.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []int64{'a'},
			TopLogProbs: []schema.TopLogProb{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []int64{'b'},
				},
			},
		},
	}}, toLogProbs(&goopenai.LogProbs{Content: []goopenai.LogProb{
		{
			Token:   "1",
			LogProb: 1,
			Bytes:   []byte{'a'},
			TopLogProbs: []goopenai.TopLogProbs{
				{
					Token:   "2",
					LogProb: 2,
					Bytes:   []byte{'b'},
				},
			},
		},
	}}))
}
