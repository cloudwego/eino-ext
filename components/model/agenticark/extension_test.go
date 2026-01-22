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

	one := &ServerToolCallArguments{}
	ret, err = concatServerToolCallArguments([]*ServerToolCallArguments{one})
	assert.NoError(t, err)
	assert.Equal(t, one, ret)

	two := &ServerToolCallArguments{}
	_, err = concatServerToolCallArguments([]*ServerToolCallArguments{one, two})
	assert.Error(t, err)
}
