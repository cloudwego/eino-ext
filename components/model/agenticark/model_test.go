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

package agenticark

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func TestNew(t *testing.T) {
	mockey.PatchConvey("TestNew", t, func() {
		ctx := context.Background()
		config := &Config{
			Model:  "test-model",
			APIKey: "test-api-key",
		}

		mockey.Mock(arkruntime.NewClientWithApiKey).Return(&arkruntime.Client{}).Build()
		mockey.Mock(arkruntime.NewClientWithAkSk).Return(&arkruntime.Client{}).Build()

		mockey.PatchConvey("success with api key", func() {
			m, err := New(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.Equal(t, "test-model", m.model)
		})

		mockey.PatchConvey("success with ak/sk", func() {
			config.APIKey = ""
			config.AccessKey = "ak"
			config.SecretKey = "sk"
			m, err := New(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
		})

		mockey.PatchConvey("fail with missing credentials", func() {
			config.APIKey = ""
			config.AccessKey = ""
			m, err := New(ctx, config)
			assert.Error(t, err)
			assert.Nil(t, m)
		})

		mockey.PatchConvey("full config", func() {
			timeout := 10 * time.Second
			retry := 3
			config.APIKey = "key"
			config.Timeout = &timeout
			config.RetryTimes = &retry
			config.Region = "cn-beijing"
			config.BaseURL = "http://test.com"
			config.HTTPClient = &http.Client{}

			m, err := New(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
		})
	})
}

func TestModelWithTools(t *testing.T) {
	mockey.PatchConvey("TestModelWithTools", t, func() {
		m := &Model{}
		tools := []*schema.ToolInfo{
			{
				Name: "test_tool",
				Desc: "test tool desc",
			},
		}

		mockey.Mock((*Model).toFunctionTools).Return([]*responses.ResponsesTool{{}}, nil).Build()

		mockey.PatchConvey("success", func() {
			nm, err := m.WithTools(tools)
			assert.NoError(t, err)
			assert.NotNil(t, nm)
			assert.Equal(t, tools, nm.(*Model).rawFunctionTools)
			assert.Equal(t, 1, len(nm.(*Model).functionTools))
		})

		mockey.PatchConvey("empty tools", func() {
			_, err := m.WithTools(nil)
			assert.Error(t, err)
		})
	})
}

func TestModelGetType(t *testing.T) {
	mockey.PatchConvey("TestModelGetType", t, func() {
		m := &Model{}
		assert.Equal(t, implType, m.GetType())
	})
}

func TestModelIsCallbacksEnabled(t *testing.T) {
	mockey.PatchConvey("TestModelIsCallbacksEnabled", t, func() {
		m := &Model{}
		assert.True(t, m.IsCallbacksEnabled())
	})
}

func TestModelToCallbackConfig(t *testing.T) {
	mockey.PatchConvey("TestModelToCallbackConfig", t, func() {
		m := &Model{}
		temp := float32(0.7)
		topP := float32(0.9)
		req := &responses.ResponsesRequest{
			Model:       "m",
			Temperature: ptrOf(float64(temp)),
			TopP:        ptrOf(float64(topP)),
		}
		cfg := m.toCallbackConfig(req)
		assert.Equal(t, "m", cfg.Model)
		assert.InDelta(t, temp, cfg.Temperature, 1e-6)
		assert.InDelta(t, topP, cfg.TopP, 1e-6)
	})
}

func TestModelGenerate(t *testing.T) {
	mockey.PatchConvey("TestModelGenerate", t, func() {
		ctx := context.Background()
		m := &Model{
			cli:   &arkruntime.Client{},
			model: "m",
		}
		input := []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeUser,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
				},
			},
		}

		mockey.Mock((*arkruntime.Client).CreateResponses).Return(&responses.ResponseObject{
			Id: "rid",
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_OutputMessage{
						OutputMessage: &responses.ItemOutputMessage{
							Role:   responses.MessageRole_assistant,
							Status: responses.ItemStatus_completed,
							Content: []*responses.OutputContentItem{
								{
									Union: &responses.OutputContentItem_Text{
										Text: &responses.OutputContentItemText{Text: "hi"},
									},
								},
							},
						},
					},
				},
			},
			Usage: &responses.Usage{
				InputTokens: 10,
			},
		}, nil).Build()

		mockey.PatchConvey("success", func() {
			out, err := m.Generate(ctx, input)
			assert.NoError(t, err)
			assert.NotNil(t, out)
			assert.Equal(t, "hi", out.ContentBlocks[0].AssistantGenText.Text)
		})

		mockey.PatchConvey("error", func() {
			mockey.Mock((*arkruntime.Client).CreateResponses).Return(nil, errors.New("err")).Build()
			_, err := m.Generate(ctx, input)
			assert.Error(t, err)
		})
	})
}

func TestModelStream(t *testing.T) {
	mockey.PatchConvey("TestModelStream", t, func() {
		ctx := context.Background()
		m := &Model{
			cli:   &arkruntime.Client{},
			model: "m",
		}
		input := []*schema.AgenticMessage{
			{Role: schema.AgenticRoleTypeUser},
		}

		mockey.PatchConvey("error creating stream", func() {
			mockey.Mock((*arkruntime.Client).CreateResponsesStream).Return(nil, errors.New("err")).Build()
			_, err := m.Stream(ctx, input)
			assert.Error(t, err)
		})
	})
}

func TestModelCreatePrefixCache(t *testing.T) {
	mockey.PatchConvey("TestModelCreatePrefixCache", t, func() {
		ctx := context.Background()
		m := &Model{
			cli:   &arkruntime.Client{},
			model: "m",
		}
		prefix := []*schema.AgenticMessage{
			{Role: schema.AgenticRoleTypeUser},
		}

		mockey.Mock((*arkruntime.Client).CreateResponses).Return(&responses.ResponseObject{
			Id: "rid",
			Usage: &responses.Usage{
				InputTokens: 10,
			},
		}, nil).Build()

		info, err := m.CreatePrefixCache(ctx, prefix, ptrOf(int64(3600)))
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, "rid", info.ResponseID)
	})
}

func TestModelToServerTools(t *testing.T) {
	mockey.PatchConvey("TestModelToServerTools", t, func() {
		m := &Model{}
		serverTools := []*ServerToolConfig{
			{
				WebSearch: &responses.ToolWebSearch{},
			},
			{}, // empty one to trigger continue
		}

		tools, err := m.toServerTools(serverTools)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(tools))
		assert.NotNil(t, tools[0].Union.(*responses.ResponsesTool_ToolWebSearch))
	})
}

func TestModelToMCPTools(t *testing.T) {
	mockey.PatchConvey("TestModelToMCPTools", t, func() {
		m := &Model{}
		mcpTools := []*responses.ToolMcp{
			{},
		}

		tools := m.toMCPTools(mcpTools)
		assert.Equal(t, 1, len(tools))
		assert.NotNil(t, tools[0].Union.(*responses.ResponsesTool_ToolMcp))
	})
}

func TestModelToFunctionTools(t *testing.T) {
	mockey.PatchConvey("TestModelToFunctionTools", t, func() {
		m := &Model{}
		tools := []*schema.ToolInfo{
			{
				Name: "t",
				Desc: "d",
			},
		}

		mockey.Mock(sonic.Marshal).Return([]byte("{}"), nil).Build()

		res, err := m.toFunctionTools(tools)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res))
		assert.Equal(t, "t", res[0].Union.(*responses.ResponsesTool_ToolFunction).ToolFunction.Name)
	})
}

func TestModelGetOptions(t *testing.T) {
	mockey.PatchConvey("TestModelGetOptions", t, func() {
		m := &Model{
			model:       "m",
			temperature: ptrOf(float32(0.7)),
		}

		mockey.PatchConvey("default", func() {
			opts, specOpts, err := m.getOptions(nil)
			assert.NoError(t, err)
			assert.Equal(t, "m", *opts.Model)
			assert.InDelta(t, float32(0.7), *opts.Temperature, 1e-6)
			assert.NotNil(t, specOpts)
		})

		mockey.PatchConvey("override", func() {
			opts, _, err := m.getOptions([]model.Option{
				model.WithTemperature(float32(0.9)),
			})
			assert.NoError(t, err)
			assert.InDelta(t, float32(0.9), *opts.Temperature, 1e-6)
		})
	})
}

func TestModelGenRequestAndOptions(t *testing.T) {
	mockey.PatchConvey("TestModelGenRequestAndOptions", t, func() {
		m := &Model{model: "m"}
		input := []*schema.AgenticMessage{{Role: schema.AgenticRoleTypeUser}}
		opts := &model.Options{Model: ptrOf("m")}
		arkOpts := &arkOptions{}

		req, err := m.genRequestAndOptions(input, opts, arkOpts)
		assert.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, "m", req.Model)
	})
}

func TestModelPrePopulateConfig(t *testing.T) {
	mockey.PatchConvey("TestModelPrePopulateConfig", t, func() {
		m := &Model{serviceTier: responses.ResponsesServiceTier_default.Enum()}
		req := &responses.ResponsesRequest{}
		opts := &model.Options{
			TopP:        ptrOf(float32(0.9)),
			Temperature: ptrOf(float32(0.7)),
			Model:       ptrOf("m2"),
		}
		specOpts := &arkOptions{
			thinking: &responses.ResponsesThinking{Type: responses.ThinkingType_enabled.Enum()},
		}

		err := m.prePopulateConfig(req, opts, specOpts)
		assert.NoError(t, err)
		assert.Equal(t, "m2", req.Model)
		assert.InDelta(t, 0.9, *req.TopP, 1e-6)
		assert.Equal(t, responses.ThinkingType_enabled, *req.Thinking.Type)
		assert.Equal(t, responses.ResponsesServiceTier_default, *req.ServiceTier)
	})
}

func TestModelPopulateCache(t *testing.T) {
	mockey.PatchConvey("TestModelPopulateCache", t, func() {
		m := &Model{}
		req := &responses.ResponsesRequest{}
		specOpts := &arkOptions{}

		mockey.PatchConvey("no cache", func() {
			in := []*schema.AgenticMessage{{Role: schema.AgenticRoleTypeUser}}
			outIn, err := m.populateCache(in, req, specOpts)
			assert.NoError(t, err)
			assert.Equal(t, in, outIn)
			assert.False(t, *req.Store)
		})

		mockey.PatchConvey("session cache enabled in model", func() {
			m.cache = &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
					ExpireAtSec: 3600,
				},
			}
			in := []*schema.AgenticMessage{{Role: schema.AgenticRoleTypeUser}}
			_, err := m.populateCache(in, req, specOpts)
			assert.NoError(t, err)
			assert.True(t, *req.Store)
			assert.Equal(t, responses.CacheType_enabled, *req.Caching.Type)
		})

		mockey.PatchConvey("response id in messages", func() {
			m.cache = &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
					ExpireAtSec: 3600,
				},
			}

			// Mock time.Now to control expiration check
			mockey.Mock(time.Now).Return(time.Unix(1000, 0)).Build()

			in := []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeAssistant,
					ResponseMeta: &schema.AgenticResponseMeta{
						Extension: &ResponseMetaExtension{
							ID:       "rid",
							ExpireAt: ptrOf(int64(2000)),
						},
					},
				},
				{Role: schema.AgenticRoleTypeUser},
			}

			outIn, err := m.populateCache(in, req, specOpts)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(outIn))
			assert.Equal(t, "rid", *req.PreviousResponseId)
		})

		mockey.PatchConvey("response id in messages - no incremental input", func() {
			m.cache = &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
					ExpireAtSec: 3600,
				},
			}
			mockey.Mock(time.Now).Return(time.Unix(1000, 0)).Build()
			in := []*schema.AgenticMessage{
				{
					Role: schema.AgenticRoleTypeAssistant,
					ResponseMeta: &schema.AgenticResponseMeta{
						Extension: &ResponseMetaExtension{
							ID:       "rid",
							ExpireAt: ptrOf(int64(2000)),
						},
					},
				},
			}
			_, err := m.populateCache(in, req, specOpts)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "incremental input not found")
		})
	})
}

func TestModelPopulateInput(t *testing.T) {
	mockey.PatchConvey("TestModelPopulateInput", t, func() {
		m := &Model{}
		req := &responses.ResponsesRequest{}

		mockey.PatchConvey("mixed roles", func() {
			in := []*schema.AgenticMessage{
				{Role: schema.AgenticRoleTypeSystem},
				{Role: schema.AgenticRoleTypeUser},
				{Role: schema.AgenticRoleTypeAssistant},
			}
			err := m.populateInput(in, req)
			assert.NoError(t, err)
			assert.NotNil(t, req.Input)
		})

		mockey.PatchConvey("invalid role", func() {
			in := []*schema.AgenticMessage{
				{Role: "invalid"},
			}
			err := m.populateInput(in, req)
			assert.Error(t, err)
		})
	})
}

func TestModelPopulateTools(t *testing.T) {
	mockey.PatchConvey("TestModelPopulateTools", t, func() {
		m := &Model{
			functionTools: []*responses.ResponsesTool{{}},
		}
		req := &responses.ResponsesRequest{}
		opts := &model.Options{}
		specOpts := &arkOptions{}

		mockey.PatchConvey("default tools", func() {
			err := m.populateTools(req, opts, specOpts)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(req.Tools))
		})

		mockey.PatchConvey("override tools", func() {
			opts.Tools = []*schema.ToolInfo{{Name: "t"}}
			err := m.populateTools(req, opts, specOpts)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(req.Tools))
		})

		mockey.PatchConvey("previous response id exists", func() {
			req.PreviousResponseId = ptrOf("rid")
			err := m.populateTools(req, opts, specOpts)
			assert.NoError(t, err)
			assert.Equal(t, 0, len(req.Tools))
		})
	})
}

func TestModelPopulateToolChoice(t *testing.T) {
	mockey.PatchConvey("TestModelPopulateToolChoice", t, func() {
		m := &Model{}
		req := &responses.ResponsesRequest{}
		opts := &model.Options{}

		mockey.PatchConvey("forbidden", func() {
			opts.AgenticToolChoice = &schema.AgenticToolChoice{
				Type: schema.ToolChoiceForbidden,
			}
			err := m.populateToolChoice(req, opts)
			assert.NoError(t, err)
			assert.Equal(t, responses.ToolChoiceMode_none, req.ToolChoice.Union.(*responses.ResponsesToolChoice_Mode).Mode)
		})

		mockey.PatchConvey("allowed - auto", func() {
			opts.AgenticToolChoice = &schema.AgenticToolChoice{
				Type: schema.ToolChoiceAllowed,
			}
			err := m.populateToolChoice(req, opts)
			assert.NoError(t, err)
			assert.Equal(t, responses.ToolChoiceMode_auto, req.ToolChoice.Union.(*responses.ResponsesToolChoice_Mode).Mode)
		})

		mockey.PatchConvey("forced - required", func() {
			opts.AgenticToolChoice = &schema.AgenticToolChoice{
				Type: schema.ToolChoiceForced,
			}
			err := m.populateToolChoice(req, opts)
			assert.NoError(t, err)
			assert.Equal(t, responses.ToolChoiceMode_required, req.ToolChoice.Union.(*responses.ResponsesToolChoice_Mode).Mode)
		})

		mockey.PatchConvey("forced - specific", func() {
			opts.AgenticToolChoice = &schema.AgenticToolChoice{
				Type: schema.ToolChoiceForced,
				Forced: &schema.AgenticForcedToolChoice{
					Tools: []*schema.AllowedTool{{FunctionName: "f"}},
				},
			}
			err := m.populateToolChoice(req, opts)
			assert.NoError(t, err)
			assert.Equal(t, "f", req.ToolChoice.Union.(*responses.ResponsesToolChoice_FunctionToolChoice).FunctionToolChoice.Name)
		})
	})
}

func TestToForcedToolChoice(t *testing.T) {
	mockey.PatchConvey("TestToForcedToolChoice", t, func() {
		mockey.PatchConvey("function", func() {
			res, err := toForcedToolChoice(&schema.AllowedTool{FunctionName: "f"})
			assert.NoError(t, err)
			assert.Equal(t, "f", res.Union.(*responses.ResponsesToolChoice_FunctionToolChoice).FunctionToolChoice.Name)
		})

		mockey.PatchConvey("mcp", func() {
			res, err := toForcedToolChoice(&schema.AllowedTool{MCPTool: &schema.AllowedMCPTool{Name: "m", ServerLabel: "s"}})
			assert.NoError(t, err)
			assert.Equal(t, "m", *res.Union.(*responses.ResponsesToolChoice_McpToolChoice).McpToolChoice.Name)
		})

		mockey.PatchConvey("server tool - web search", func() {
			res, err := toForcedToolChoice(&schema.AllowedTool{ServerTool: &schema.AllowedServerTool{Name: string(ServerToolNameWebSearch)}})
			assert.NoError(t, err)
			assert.Equal(t, responses.ToolType_web_search, res.Union.(*responses.ResponsesToolChoice_WebSearchToolChoice).WebSearchToolChoice.Type)
		})

		mockey.PatchConvey("unknown", func() {
			_, err := toForcedToolChoice(&schema.AllowedTool{})
			assert.Error(t, err)
		})
	})
}
