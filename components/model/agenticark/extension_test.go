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
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestGetResponseMeta(t *testing.T) {
	var nilMeta *schema.AgenticResponseMeta
	assert.Nil(t, getResponseMeta(nilMeta))

	metaWithoutExt := &schema.AgenticResponseMeta{}
	assert.Nil(t, getResponseMeta(metaWithoutExt))

	meta := &schema.AgenticResponseMeta{
		Extension: &ResponseMetaExtension{
			ID:     "id",
			Status: "ok",
		},
	}
	ext := getResponseMeta(meta)
	assert.NotNil(t, ext)
	assert.Equal(t, "id", ext.ID)
	assert.Equal(t, ResponseStatus("ok"), ext.Status)

	// map[string]any conversion
	metaFromMap := &schema.AgenticResponseMeta{
		Extension: map[string]any{
			"id":     "id-from-map",
			"status": "completed",
		},
	}
	extFromMap := getResponseMeta(metaFromMap)
	assert.NotNil(t, extFromMap)
	assert.Equal(t, "id-from-map", extFromMap.ID)
	assert.Equal(t, ResponseStatus("completed"), extFromMap.Status)

	// map[string]any with nested fields
	metaFromMapNested := &schema.AgenticResponseMeta{
		Extension: map[string]any{
			"id":     "id-nested",
			"status": "failed",
			"error": map[string]any{
				"code":    "ERR001",
				"message": "something went wrong",
			},
			"incomplete_details": map[string]any{
				"reason": "timeout",
			},
		},
	}
	extFromMapNested := getResponseMeta(metaFromMapNested)
	assert.NotNil(t, extFromMapNested)
	assert.Equal(t, "id-nested", extFromMapNested.ID)
	assert.NotNil(t, extFromMapNested.Error)
	assert.Equal(t, "ERR001", extFromMapNested.Error.Code)
	assert.Equal(t, "something went wrong", extFromMapNested.Error.Message)
	assert.NotNil(t, extFromMapNested.IncompleteDetails)
	assert.Equal(t, "timeout", extFromMapNested.IncompleteDetails.Reason)

	// wrong type returns nil
	metaWrongType := &schema.AgenticResponseMeta{
		Extension: "wrong type",
	}
	assert.Nil(t, getResponseMeta(metaWrongType))
}

func TestGetServerToolCallArguments(t *testing.T) {
	args, err := getServerToolCallArguments(nil)
	assert.Error(t, err)
	assert.Nil(t, args)

	callWithNilArgs := &schema.ServerToolCall{}
	args, err = getServerToolCallArguments(callWithNilArgs)
	assert.Error(t, err)
	assert.Nil(t, args)

	callWithWrongType := &schema.ServerToolCall{
		Arguments: struct{ X string }{X: "v"},
	}
	args, err = getServerToolCallArguments(callWithWrongType)
	assert.Error(t, err)
	assert.Nil(t, args)

	expected := &ServerToolCallArguments{
		WebSearch: &WebSearchArguments{
			ActionType: WebSearchActionSearch,
			Search: &WebSearchQuery{
				Query: "q",
			},
		},
	}
	callWithCorrectArgs := &schema.ServerToolCall{
		Arguments: expected,
	}
	args, err = getServerToolCallArguments(callWithCorrectArgs)
	assert.NoError(t, err)
	assert.Equal(t, expected, args)

	// map[string]any with web_search
	callWithMapWebSearch := &schema.ServerToolCall{
		Arguments: map[string]any{
			"web_search": map[string]any{
				"action_type": "search",
				"search": map[string]any{
					"query": "test query",
				},
			},
		},
	}
	args, err = getServerToolCallArguments(callWithMapWebSearch)
	assert.NoError(t, err)
	assert.NotNil(t, args)
	assert.NotNil(t, args.WebSearch)
	assert.Equal(t, WebSearchAction("search"), args.WebSearch.ActionType)
	assert.Equal(t, "test query", args.WebSearch.Search.Query)

	// map[string]any with image_process
	callWithMapImageProcess := &schema.ServerToolCall{
		Arguments: map[string]any{
			"image_process": map[string]any{
				"action_type": "point",
				"point": map[string]any{
					"image_index": float64(0),
					"points":      "100,200",
					"draw_line":   true,
				},
			},
		},
	}
	args, err = getServerToolCallArguments(callWithMapImageProcess)
	assert.NoError(t, err)
	assert.NotNil(t, args)
	assert.NotNil(t, args.ImageProcess)
	assert.Equal(t, ImageProcessAction("point"), args.ImageProcess.ActionType)
	assert.NotNil(t, args.ImageProcess.Point)
	assert.Equal(t, int32(0), args.ImageProcess.Point.ImageIndex)
	assert.Equal(t, "100,200", args.ImageProcess.Point.Points)
	assert.True(t, args.ImageProcess.Point.DrawLine)

	// map[string]any with doubao_app
	callWithMapDoubaoApp := &schema.ServerToolCall{
		Arguments: map[string]any{
			"doubao_app": map[string]any{
				"feature": "ai_search",
			},
		},
	}
	args, err = getServerToolCallArguments(callWithMapDoubaoApp)
	assert.NoError(t, err)
	assert.NotNil(t, args)
	assert.NotNil(t, args.DoubaoApp)
	assert.Equal(t, DoubaoAppFeature("ai_search"), args.DoubaoApp.Feature)

	// map[string]any with knowledge_search
	callWithMapKnowledgeSearch := &schema.ServerToolCall{
		Arguments: map[string]any{
			"knowledge_search": map[string]any{
				"knowledge_resource_id": "res-123",
				"queries":               []any{"query1", "query2"},
			},
		},
	}
	args, err = getServerToolCallArguments(callWithMapKnowledgeSearch)
	assert.NoError(t, err)
	assert.NotNil(t, args)
	assert.NotNil(t, args.KnowledgeSearch)
	assert.Equal(t, "res-123", args.KnowledgeSearch.KnowledgeResourceID)
	assert.Equal(t, []string{"query1", "query2"}, args.KnowledgeSearch.Queries)
}

func TestGetServerToolResult(t *testing.T) {
	res, err := getServerToolResult(nil)
	assert.Error(t, err)
	assert.Nil(t, res)

	resWithNilResult := &schema.ServerToolResult{}
	res, err = getServerToolResult(resWithNilResult)
	assert.Error(t, err)
	assert.Nil(t, res)

	resWithWrongType := &schema.ServerToolResult{
		Result: struct{ X string }{X: "v"},
	}
	res, err = getServerToolResult(resWithWrongType)
	assert.Error(t, err)
	assert.Nil(t, res)

	expected := &ServerToolResult{
		ImageProcess: &ImageProcessResult{
			Action: &ImageProcessResultAction{
				Type:           ImageProcessActionPoint,
				ResultImageURL: "https://example.com/image.png",
			},
		},
	}
	resWithCorrectResult := &schema.ServerToolResult{
		Result: expected,
	}
	res, err = getServerToolResult(resWithCorrectResult)
	assert.NoError(t, err)
	assert.Equal(t, expected, res)

	// map[string]any with image_process
	resWithMapImageProcess := &schema.ServerToolResult{
		Result: map[string]any{
			"image_process": map[string]any{
				"action": map[string]any{
					"type":             "point",
					"result_image_url": "https://example.com/result.png",
				},
			},
		},
	}
	res, err = getServerToolResult(resWithMapImageProcess)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.ImageProcess)
	assert.NotNil(t, res.ImageProcess.Action)
	assert.Equal(t, ImageProcessAction("point"), res.ImageProcess.Action.Type)
	assert.Equal(t, "https://example.com/result.png", res.ImageProcess.Action.ResultImageURL)

	// map[string]any with image_process error
	resWithMapImageProcessError := &schema.ServerToolResult{
		Result: map[string]any{
			"image_process": map[string]any{
				"error": map[string]any{
					"message": "processing failed",
				},
			},
		},
	}
	res, err = getServerToolResult(resWithMapImageProcessError)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.ImageProcess)
	assert.NotNil(t, res.ImageProcess.Error)
	assert.Equal(t, "processing failed", res.ImageProcess.Error.Message)

	// map[string]any with doubao_app
	resWithMapDoubaoApp := &schema.ServerToolResult{
		Result: map[string]any{
			"doubao_app": map[string]any{
				"blocks": []any{
					map[string]any{
						"type": "output_text",
						"output_text": map[string]any{
							"id":   "text-1",
							"text": "hello world",
						},
					},
				},
			},
		},
	}
	res, err = getServerToolResult(resWithMapDoubaoApp)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.DoubaoApp)
	assert.Len(t, res.DoubaoApp.Blocks, 1)
	assert.Equal(t, DoubaoAppBlockType("output_text"), res.DoubaoApp.Blocks[0].Type)
	assert.NotNil(t, res.DoubaoApp.Blocks[0].OutputText)
	assert.Equal(t, "text-1", res.DoubaoApp.Blocks[0].OutputText.ID)
	assert.Equal(t, "hello world", res.DoubaoApp.Blocks[0].OutputText.Text)
}

func TestConcatResponseMetaExtensions(t *testing.T) {
	ret, err := concatResponseMetaExtensions(nil)
	assert.Error(t, err)
	assert.Nil(t, ret)

	one := &ResponseMetaExtension{ID: "id1"}
	ret, err = concatResponseMetaExtensions([]*ResponseMetaExtension{one})
	assert.NoError(t, err)
	assert.Equal(t, one, ret)

	id2 := &ResponseMetaExtension{ID: "id2"}
	err2 := &ResponseError{Code: "c"}
	meta1 := &ResponseMetaExtension{
		ID:                "base",
		Status:            "s1",
		IncompleteDetails: &IncompleteDetails{Reason: "r"},
		Error:             err2,
	}
	meta2 := &ResponseMetaExtension{
		ID:                 id2.ID,
		Status:             "s2",
		PreviousResponseID: "prev",
	}
	ret, err = concatResponseMetaExtensions([]*ResponseMetaExtension{meta1, meta2, nil})
	assert.NoError(t, err)
	assert.Equal(t, meta2.ID, ret.ID)
	assert.Equal(t, ResponseStatus("s2"), ret.Status)
	assert.Equal(t, meta1.IncompleteDetails, ret.IncompleteDetails)
	assert.Equal(t, err2, ret.Error)
	assert.Equal(t, "prev", ret.PreviousResponseID)
}

func TestConcatAssistantGenTextExtensions(t *testing.T) {
	a0 := &TextAnnotation{Index: 0}
	a1 := &TextAnnotation{Index: 1}
	e0 := &AssistantGenTextExtension{Annotations: []*TextAnnotation{a0}}
	e1 := &AssistantGenTextExtension{Annotations: []*TextAnnotation{a1}}
	ret, err := concatAssistantGenTextExtensions([]*AssistantGenTextExtension{e0, e1})
	assert.NoError(t, err)
	assert.Len(t, ret.Annotations, 2)
	assert.Equal(t, &TextAnnotation{Index: 0}, ret.Annotations[0])
	assert.Equal(t, &TextAnnotation{Index: 0}, ret.Annotations[1])

	dup := &TextAnnotation{Index: 0}
	_, err = concatAssistantGenTextExtensions([]*AssistantGenTextExtension{
		{Annotations: []*TextAnnotation{a0}},
		{Annotations: []*TextAnnotation{dup}},
	})
	assert.Error(t, err)
}

func TestConcatServerToolCallArguments(t *testing.T) {
	ret, err := concatServerToolCallArguments(nil)
	assert.Error(t, err)
	assert.Nil(t, ret)

	one := &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: "search"}}
	ret, err = concatServerToolCallArguments([]*ServerToolCallArguments{one})
	assert.NoError(t, err)
	assert.Equal(t, one, ret)

	_, err = concatServerToolCallArguments([]*ServerToolCallArguments{nil, nil, nil})
	assert.Error(t, err)

	ret, err = concatServerToolCallArguments([]*ServerToolCallArguments{nil, one, nil})
	assert.NoError(t, err)
	assert.NotNil(t, ret.WebSearch)
	assert.Equal(t, WebSearchAction("search"), ret.WebSearch.ActionType)

	two := &ServerToolCallArguments{WebSearch: &WebSearchArguments{ActionType: "updated"}}
	_, err = concatServerToolCallArguments([]*ServerToolCallArguments{one, two})
	assert.Error(t, err)

	ws := &ServerToolCallArguments{WebSearch: &WebSearchArguments{}}
	da := &ServerToolCallArguments{DoubaoApp: &DoubaoAppArguments{}}
	_, err = concatServerToolCallArguments([]*ServerToolCallArguments{ws, da})
	assert.Error(t, err)

	da1 := &ServerToolCallArguments{DoubaoApp: &DoubaoAppArguments{}}
	da2 := &ServerToolCallArguments{DoubaoApp: &DoubaoAppArguments{Feature: "ai_search"}}
	da3 := &ServerToolCallArguments{DoubaoApp: &DoubaoAppArguments{}}
	ret, err = concatServerToolCallArguments([]*ServerToolCallArguments{da1, da2, da3})
	assert.NoError(t, err)
	assert.Equal(t, DoubaoAppFeature("ai_search"), ret.DoubaoApp.Feature)
}

func TestConcatServerToolResult(t *testing.T) {
	ret, err := concatServerToolResult(nil)
	assert.Error(t, err)
	assert.Nil(t, ret)

	one := &ServerToolResult{}
	ret, err = concatServerToolResult([]*ServerToolResult{one})
	assert.NoError(t, err)
	assert.Equal(t, one, ret)

	_, err = concatServerToolResult([]*ServerToolResult{nil, nil, nil})
	assert.Error(t, err)

	ip1 := &ServerToolResult{ImageProcess: &ImageProcessResult{}}
	ret, err = concatServerToolResult([]*ServerToolResult{nil, ip1, nil})
	assert.NoError(t, err)
	assert.NotNil(t, ret.ImageProcess)

	ip2 := &ServerToolResult{ImageProcess: &ImageProcessResult{}}
	_, err = concatServerToolResult([]*ServerToolResult{ip1, ip2})
	assert.Error(t, err)

	ip3 := &ServerToolResult{ImageProcess: &ImageProcessResult{}}
	da1 := &ServerToolResult{DoubaoApp: &DoubaoAppResult{}}
	_, err = concatServerToolResult([]*ServerToolResult{ip3, da1})
	assert.Error(t, err)

	da2 := &ServerToolResult{DoubaoApp: &DoubaoAppResult{}}
	ret, err = concatServerToolResult([]*ServerToolResult{nil, da1, da2, nil})
	assert.NoError(t, err)
	assert.NotNil(t, ret.DoubaoApp)
}

func TestConcatDoubaoAppResults(t *testing.T) {
	ret, err := concatDoubaoAppResults(nil)
	assert.NoError(t, err)
	assert.Nil(t, ret)

	one := &DoubaoAppResult{}
	ret, err = concatDoubaoAppResults([]*DoubaoAppResult{one})
	assert.NoError(t, err)
	assert.Equal(t, one, ret)

	chunk1 := &DoubaoAppResult{
		Blocks: []*DoubaoAppBlock{
			{
				StreamingMeta: &DoubaoAppStreamingMeta{Index: 0},
				Type:          DoubaoAppBlockTypeOutputText,
				OutputText:    &DoubaoAppOutputText{ID: "text1", Text: "hello "},
			},
		},
	}
	chunk2 := &DoubaoAppResult{
		Blocks: []*DoubaoAppBlock{
			{
				StreamingMeta: &DoubaoAppStreamingMeta{Index: 0},
				Type:          DoubaoAppBlockTypeOutputText,
				OutputText:    &DoubaoAppOutputText{Text: "world"},
			},
		},
	}
	chunk3 := &DoubaoAppResult{
		Blocks: []*DoubaoAppBlock{
			{
				StreamingMeta: &DoubaoAppStreamingMeta{Index: 1},
				Type:          DoubaoAppBlockTypeSearch,
				Search:        &DoubaoAppSearch{ID: "s1", SearchingState: "searching"},
			},
		},
	}
	chunk4 := &DoubaoAppResult{
		Blocks: []*DoubaoAppBlock{
			{
				StreamingMeta: &DoubaoAppStreamingMeta{Index: 1},
				Type:          DoubaoAppBlockTypeSearch,
				Search:        &DoubaoAppSearch{Summary: "done", Queries: []string{"q1"}},
			},
		},
	}

	ret, err = concatDoubaoAppResults([]*DoubaoAppResult{chunk1, chunk2, chunk3, chunk4, nil})
	assert.NoError(t, err)
	assert.Len(t, ret.Blocks, 2)

	assert.Equal(t, DoubaoAppBlockTypeOutputText, ret.Blocks[0].Type)
	assert.Equal(t, "text1", ret.Blocks[0].OutputText.ID)
	assert.Equal(t, "hello world", ret.Blocks[0].OutputText.Text)

	assert.Equal(t, DoubaoAppBlockTypeSearch, ret.Blocks[1].Type)
	assert.Equal(t, "s1", ret.Blocks[1].Search.ID)
	assert.Equal(t, "done", ret.Blocks[1].Search.Summary)
	assert.Equal(t, []string{"q1"}, ret.Blocks[1].Search.Queries)
}

func TestConcatDoubaoAppBlock(t *testing.T) {
	dst := &DoubaoAppBlock{}

	concatDoubaoAppBlock(dst, &DoubaoAppBlock{
		Type:          DoubaoAppBlockTypeReasoningText,
		ReasoningText: &DoubaoAppReasoningText{ID: "r1", ReasoningText: "thinking "},
	})
	assert.Equal(t, DoubaoAppBlockTypeReasoningText, dst.Type)
	assert.Equal(t, "r1", dst.ReasoningText.ID)
	assert.Equal(t, "thinking ", dst.ReasoningText.ReasoningText)

	concatDoubaoAppBlock(dst, &DoubaoAppBlock{
		Type:          DoubaoAppBlockTypeReasoningText,
		ReasoningText: &DoubaoAppReasoningText{ReasoningText: "more"},
	})
	assert.Equal(t, "thinking more", dst.ReasoningText.ReasoningText)

	dst2 := &DoubaoAppBlock{}
	concatDoubaoAppBlock(dst2, &DoubaoAppBlock{
		Type: DoubaoAppBlockTypeSearch,
		Search: &DoubaoAppSearch{
			ID:             "s1",
			SearchingState: "找到 2 篇资料",
		},
	})

	concatDoubaoAppBlock(dst2, &DoubaoAppBlock{
		Type: DoubaoAppBlockTypeSearch,
		Search: &DoubaoAppSearch{
			Summary: "搜索摘要",
			Queries: []string{"q1", "q2"},
			Results: []*DoubaoAppSearchResult{{Title: "t1"}},
		},
	})
	assert.Equal(t, "s1", dst2.Search.ID)
	assert.Equal(t, "搜索摘要", dst2.Search.Summary)
	assert.Equal(t, []string{"q1", "q2"}, dst2.Search.Queries)
	assert.Len(t, dst2.Search.Results, 1)

	dst3 := &DoubaoAppBlock{}
	concatDoubaoAppBlock(dst3, &DoubaoAppBlock{
		Type: DoubaoAppBlockTypeReasoningSearch,
		ReasoningSearch: &DoubaoAppReasoningSearch{
			ID:             "rs1",
			SearchingState: "searching",
		},
	})
	concatDoubaoAppBlock(dst3, &DoubaoAppBlock{
		Type: DoubaoAppBlockTypeReasoningSearch,
		ReasoningSearch: &DoubaoAppReasoningSearch{
			Summary: "done",
			Results: []*DoubaoAppSearchResult{{Title: "t1"}},
		},
	})
	assert.Equal(t, "rs1", dst3.ReasoningSearch.ID)
	assert.Equal(t, "done", dst3.ReasoningSearch.Summary)
	assert.Len(t, dst3.ReasoningSearch.Results, 1)
}
