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

package ark

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestBindTools(t *testing.T) {
	t.Run("chat model force tool call", func(t *testing.T) {
		ctx := context.Background()

		chatModel, err := NewChatModel(ctx, &ChatModelConfig{Model: "gpt-3.5-turbo", APIKey: "test"})
		assert.NoError(t, err)

		doNothingParams := map[string]*schema.ParameterInfo{
			"test": {
				Type:     schema.String,
				Desc:     "no meaning",
				Required: true,
			},
		}

		stockParams := map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Desc:     "The name of the stock",
				Required: true,
			},
		}

		tools := []*schema.ToolInfo{
			{
				Name:        "do_nothing",
				Desc:        "do nothing",
				ParamsOneOf: schema.NewParamsOneOfByParams(doNothingParams),
			},
			{
				Name:        "get_current_stock_price",
				Desc:        "Get the current stock price given the name of the stock",
				ParamsOneOf: schema.NewParamsOneOfByParams(stockParams),
			},
		}

		err = chatModel.BindTools([]*schema.ToolInfo{tools[0]})
		assert.Nil(t, err)

	})
}

func TestWithTools(t *testing.T) {
	cm := &ChatModel{
		chatModel: &completionAPIChatModel{
			model: "test model",
			rawTools: []*schema.ToolInfo{
				{
					Name: "test name",
				},
			},
		},
		respChatModel: &responsesAPIChatModel{
			model: "test model",
			rawTools: []*schema.ToolInfo{
				{
					Name: "test name",
				},
			},
		},
	}

	ncm, err := cm.WithTools([]*schema.ToolInfo{{Name: "test tool name"}})
	assert.Nil(t, err)

	assert.Equal(t, "test model", ncm.(*ChatModel).chatModel.model)
	assert.Equal(t, "test model", ncm.(*ChatModel).respChatModel.model)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).chatModel.rawTools[0].Name)
	assert.Equal(t, "test tool name", ncm.(*ChatModel).respChatModel.rawTools[0].Name)
	assert.Equal(t, "test name", cm.chatModel.rawTools[0].Name)
	assert.Equal(t, "test name", cm.respChatModel.rawTools[0].Name)
}

func TestCallByResponsesAPI(t *testing.T) {
	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{},
		}
		opt := WithCache(&CacheOption{
			APIType: ResponsesAPI,
		})

		ok, err := cm.callByResponsesAPI(opt)
		assert.Nil(t, err)
		assert.True(t, ok)
	})

	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{
				cache: &CacheConfig{
					APIType: ptrOf(ContextAPI),
				},
			},
		}
		opt := WithCache(&CacheOption{
			APIType: ResponsesAPI,
		})

		ok, err := cm.callByResponsesAPI(opt)
		assert.Nil(t, err)
		assert.True(t, ok)
	})

	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{
				cache: &CacheConfig{
					APIType: ptrOf(ResponsesAPI),
				},
			},
		}
		opt := WithCache(&CacheOption{
			APIType: ContextAPI,
		})

		ok, err := cm.callByResponsesAPI(opt)
		assert.Nil(t, err)
		assert.False(t, ok)
	})

	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{
				cache: &CacheConfig{
					APIType: (*APIType)(ptrOf("")),
				},
			},
		}
		opt := WithCache(&CacheOption{
			APIType: ContextAPI,
		})

		_, err := cm.callByResponsesAPI(opt)
		assert.Nil(t, err)
	})

	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{
				cache: &CacheConfig{
					APIType: (*APIType)(ptrOf("")),
				},
			},
		}
		opt := WithCache(&CacheOption{})

		_, err := cm.callByResponsesAPI(opt)
		assert.NotNil(t, err)
	})

	mockey.PatchConvey("", t, func() {
		cm := &ChatModel{
			respChatModel: &responsesAPIChatModel{
				cache: &CacheConfig{},
			},
		}

		ok, err := cm.callByResponsesAPI()
		assert.Nil(t, err)
		assert.False(t, ok)
	})
}

func TestBuildResponsesAPIChatModel(t *testing.T) {
	mockey.PatchConvey("invalid config", t, func() {
		_, err := buildResponsesAPIChatModel(&ChatModelConfig{
			Stop:   []string{"test"},
			APIKey: "test",
			Cache: &CacheConfig{
				APIType: ptrOf(ResponsesAPI),
			},
		})
		assert.NotNil(t, err)
	})

	mockey.PatchConvey("valid config", t, func() {
		_, err := buildResponsesAPIChatModel(&ChatModelConfig{
			APIKey: "test",
			Cache: &CacheConfig{
				APIType: ptrOf(ResponsesAPI),
			},
		})
		assert.Nil(t, err)
	})
}

func TestBuildChatCompletionAPIChatModel(t *testing.T) {
	mockey.PatchConvey("invalid config", t, func() {
		cli, err := buildChatCompletionAPIChatModel(&ChatModelConfig{
			Stop: []string{"test"},
			Cache: &CacheConfig{
				APIType: ptrOf(ContextAPI),
			},
		})
		assert.NotNil(t, cli)
		assert.Nil(t, err)
	})
}
