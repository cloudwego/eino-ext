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

package agenticopenai

import (
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestGetServerToolCallArguments(t *testing.T) {
	mockey.PatchConvey("getServerToolCallArguments", t, func() {
		mockey.PatchConvey("success", func() {
			args := &ServerToolCallArguments{}
			call := &schema.ServerToolCall{
				Arguments: args,
			}
			res, err := getServerToolCallArguments(call)
			assert.NoError(t, err)
			assert.Equal(t, args, res)
		})

		mockey.PatchConvey("nil input", func() {
			res, err := getServerToolCallArguments(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("nil arguments", func() {
			call := &schema.ServerToolCall{
				Arguments: nil,
			}
			res, err := getServerToolCallArguments(call)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("wrong type", func() {
			call := &schema.ServerToolCall{
				Arguments: "wrong type",
			}
			res, err := getServerToolCallArguments(call)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("map[string]any success", func() {
			call := &schema.ServerToolCall{
				Arguments: map[string]any{
					"web_search": map[string]any{
						"action_type": "search",
						"search": map[string]any{
							"queries": []any{"query1", "query2"},
						},
					},
				},
			}
			res, err := getServerToolCallArguments(call)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.WebSearch)
			assert.Equal(t, WebSearchAction("search"), res.WebSearch.ActionType)
			assert.NotNil(t, res.WebSearch.Search)
			assert.Equal(t, []string{"query1", "query2"}, res.WebSearch.Search.Queries)
		})

		mockey.PatchConvey("map[string]any with file_search", func() {
			call := &schema.ServerToolCall{
				Arguments: map[string]any{
					"file_search": map[string]any{
						"queries": []any{"file query"},
					},
				},
			}
			res, err := getServerToolCallArguments(call)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.FileSearch)
			assert.Equal(t, []string{"file query"}, res.FileSearch.Queries)
		})

		mockey.PatchConvey("map[string]any with shell", func() {
			call := &schema.ServerToolCall{
				Arguments: map[string]any{
					"shell": map[string]any{
						"created_by": "user1",
						"action": map[string]any{
							"commands":   []any{"ls", "pwd"},
							"timeout_ms": float64(5000),
						},
					},
				},
			}
			res, err := getServerToolCallArguments(call)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.Shell)
			assert.Equal(t, "user1", res.Shell.CreatedBy)
			assert.NotNil(t, res.Shell.Action)
			assert.Equal(t, []string{"ls", "pwd"}, res.Shell.Action.Commands)
			assert.Equal(t, int64(5000), res.Shell.Action.TimeoutMs)
		})
	})
}

func TestGetServerToolResult(t *testing.T) {
	mockey.PatchConvey("getServerToolResult", t, func() {
		mockey.PatchConvey("success", func() {
			result := &ServerToolResult{}
			content := &schema.ServerToolResult{
				Result: result,
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.Equal(t, result, res)
		})

		mockey.PatchConvey("nil input", func() {
			res, err := getServerToolResult(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("nil result", func() {
			content := &schema.ServerToolResult{
				Result: nil,
			}
			res, err := getServerToolResult(content)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("wrong type", func() {
			content := &schema.ServerToolResult{
				Result: "wrong type",
			}
			res, err := getServerToolResult(content)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("map[string]any with web_search", func() {
			content := &schema.ServerToolResult{
				Result: map[string]any{
					"web_search": map[string]any{
						"action_type": "search",
						"search": map[string]any{
							"sources": []any{
								map[string]any{"url": "https://example.com"},
							},
						},
					},
				},
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.WebSearch)
			assert.Equal(t, WebSearchAction("search"), res.WebSearch.ActionType)
			assert.NotNil(t, res.WebSearch.Search)
			assert.Len(t, res.WebSearch.Search.Sources, 1)
			assert.Equal(t, "https://example.com", res.WebSearch.Search.Sources[0].URL)
		})

		mockey.PatchConvey("map[string]any with code_interpreter", func() {
			content := &schema.ServerToolResult{
				Result: map[string]any{
					"code_interpreter": map[string]any{
						"code":         "print('hello')",
						"container_id": "container123",
						"outputs": []any{
							map[string]any{
								"type": "logs",
								"logs": map[string]any{
									"logs": "hello",
								},
							},
						},
					},
				},
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.CodeInterpreter)
			assert.Equal(t, "print('hello')", res.CodeInterpreter.Code)
			assert.Equal(t, "container123", res.CodeInterpreter.ContainerID)
			assert.Len(t, res.CodeInterpreter.Outputs, 1)
			assert.Equal(t, CodeInterpreterOutputType("logs"), res.CodeInterpreter.Outputs[0].Type)
		})

		mockey.PatchConvey("map[string]any with image_generation", func() {
			content := &schema.ServerToolResult{
				Result: map[string]any{
					"image_generation": map[string]any{
						"image_base64": "base64data",
					},
				},
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.ImageGeneration)
			assert.Equal(t, "base64data", res.ImageGeneration.ImageBase64)
		})

		mockey.PatchConvey("map[string]any with shell", func() {
			content := &schema.ServerToolResult{
				Result: map[string]any{
					"shell": map[string]any{
						"max_output_length": float64(1000),
						"created_by":        "user1",
						"outputs": []any{
							map[string]any{
								"stdout": "output",
								"stderr": "",
							},
						},
					},
				},
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.NotNil(t, res.Shell)
			assert.Equal(t, int64(1000), res.Shell.MaxOutputLength)
			assert.Equal(t, "user1", res.Shell.CreatedBy)
			assert.Len(t, res.Shell.Outputs, 1)
			assert.Equal(t, "output", res.Shell.Outputs[0].Stdout)
		})
	})
}

func TestConcatServerToolCallArguments(t *testing.T) {
	mockey.PatchConvey("concatServerToolCallArguments", t, func() {
		mockey.PatchConvey("empty chunks", func() {
			res, err := concatServerToolCallArguments(nil)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("one chunk", func() {
			args := &ServerToolCallArguments{}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args})
			assert.NoError(t, err)
			assert.Equal(t, args, res)
		})

		mockey.PatchConvey("multiple nil chunks", func() {
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{nil, nil, nil})
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("multiple web search chunks", func() {
			args1 := &ServerToolCallArguments{WebSearch: &WebSearchArguments{}}
			args2 := &ServerToolCallArguments{WebSearch: &WebSearchArguments{}}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args1, args2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("multiple shell chunks", func() {
			args1 := &ServerToolCallArguments{Shell: &ShellArguments{CreatedBy: "user1"}}
			args2 := &ServerToolCallArguments{Shell: &ShellArguments{CreatedBy: "user2"}}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args1, args2})
			assert.NoError(t, err)
			assert.Equal(t, "user2", res.Shell.CreatedBy)
		})

		mockey.PatchConvey("mixed type chunks", func() {
			args1 := &ServerToolCallArguments{WebSearch: &WebSearchArguments{}}
			args2 := &ServerToolCallArguments{Shell: &ShellArguments{}}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args1, args2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	})
}

func TestConcatServerToolResult(t *testing.T) {
	mockey.PatchConvey("concatServerToolResult", t, func() {
		mockey.PatchConvey("empty chunks", func() {
			res, err := concatServerToolResult(nil)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("one chunk", func() {
			result := &ServerToolResult{}
			res, err := concatServerToolResult([]*ServerToolResult{result})
			assert.NoError(t, err)
			assert.Equal(t, result, res)
		})

		mockey.PatchConvey("multiple WebSearch chunks error", func() {
			result1 := &ServerToolResult{WebSearch: &WebSearchResult{}}
			result2 := &ServerToolResult{WebSearch: &WebSearchResult{}}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("multiple FileSearch chunks error", func() {
			result1 := &ServerToolResult{FileSearch: &FileSearchResult{}}
			result2 := &ServerToolResult{FileSearch: &FileSearchResult{}}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("concat CodeInterpreter code delta", func() {
			result1 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{
					Code: "print('hello",
				},
			}
			result2 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{
					Code: " world')",
				},
			}
			result3 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{
					ContainerID: "container123",
					Outputs: []*CodeInterpreterOutput{
						{Logs: &CodeInterpreterOutputLogs{Logs: "hello world"}},
					},
				},
			}
			result4 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{
					Outputs: []*CodeInterpreterOutput{
						{Image: &CodeInterpreterOutputImage{URL: "https://example.com/img.png"}},
					},
				},
			}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2, result3, result4})
			assert.NoError(t, err)
			assert.NotNil(t, res.CodeInterpreter)
			assert.Equal(t, "print('hello world')", res.CodeInterpreter.Code)
			assert.Equal(t, "container123", res.CodeInterpreter.ContainerID)
			assert.Len(t, res.CodeInterpreter.Outputs, 2)
		})

		mockey.PatchConvey("skip nil chunks", func() {
			result1 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{Code: "x = 1"},
			}
			res, err := concatServerToolResult([]*ServerToolResult{nil, result1, nil})
			assert.NoError(t, err)
			assert.NotNil(t, res.CodeInterpreter)
			assert.Equal(t, "x = 1", res.CodeInterpreter.Code)
		})

		mockey.PatchConvey("concat ImageGeneration partial images", func() {
			result1 := &ServerToolResult{
				ImageGeneration: &ImageGenerationResult{
					ImageBase64: "partial1",
				},
			}
			result2 := &ServerToolResult{
				ImageGeneration: &ImageGenerationResult{
					ImageBase64: "partial2",
				},
			}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2})
			assert.NoError(t, err)
			assert.NotNil(t, res.ImageGeneration)
			assert.Equal(t, "partial1partial2", res.ImageGeneration.ImageBase64)
		})

		mockey.PatchConvey("cannot mix CodeInterpreter and ImageGeneration", func() {
			result1 := &ServerToolResult{
				CodeInterpreter: &CodeInterpreterResult{Code: "x = 1"},
			}
			result2 := &ServerToolResult{
				ImageGeneration: &ImageGenerationResult{ImageBase64: "img"},
			}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2})
			assert.Error(t, err)
			assert.Nil(t, res)
			assert.Contains(t, err.Error(), "type mismatch")
		})
	})
}
