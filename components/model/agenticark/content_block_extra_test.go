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

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestSetUserInputVideoFPS(t *testing.T) {
	mockey.PatchConvey("TestSetUserInputVideoFPS", t, func() {
		video := &schema.UserInputVideo{
			URL:      "http://example.com/video.mp4",
			MIMEType: "video/mp4",
		}

		block := schema.NewContentBlock(video)
		setBlockExtraValue(block, videoURLFPS, 30.0)

		assert.NotNil(t, block.Extra)
		assert.Equal(t, 30.0, block.Extra[videoURLFPS])
	})
}

func TestGetUserInputVideoFPS(t *testing.T) {
	mockey.PatchConvey("TestGetUserInputVideoFPS", t, func() {
		video := &schema.UserInputVideo{
			URL:      "http://example.com/video.mp4",
			MIMEType: "video/mp4",
		}

		mockey.PatchConvey("not set", func() {
			fps, ok := GetUserInputVideoFPS(video)
			assert.False(t, ok)
			assert.Equal(t, 0.0, fps)
		})

		mockey.PatchConvey("set and get", func() {
			block := schema.NewContentBlock(video)
			setBlockExtraValue(block, videoURLFPS, 60.0)

			fps, ok := getBlockExtraValue[float64](block, videoURLFPS)
			assert.True(t, ok)
			assert.Equal(t, 60.0, fps)
		})
	})
}

func TestSetAndGetItemID(t *testing.T) {
	mockey.PatchConvey("TestSetAndGetItemID", t, func() {
		block := schema.NewContentBlock(&schema.AssistantGenText{Text: "test"})

		mockey.PatchConvey("not set", func() {
			id, ok := getItemID(block)
			assert.False(t, ok)
			assert.Equal(t, "", id)
		})

		mockey.PatchConvey("set and get", func() {
			setItemID(block, "item-123")
			id, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "item-123", id)
		})

		mockey.PatchConvey("nil block", func() {
			id, ok := getItemID(nil)
			assert.False(t, ok)
			assert.Equal(t, "", id)
		})
	})
}

func TestSetAndGetItemStatus(t *testing.T) {
	mockey.PatchConvey("TestSetAndGetItemStatus", t, func() {
		block := schema.NewContentBlock(&schema.AssistantGenText{Text: "test"})

		mockey.PatchConvey("not set", func() {
			status, ok := GetItemStatus(block)
			assert.False(t, ok)
			assert.Equal(t, "", status)
		})

		mockey.PatchConvey("set and get", func() {
			setItemStatus(block, "completed")
			status, ok := GetItemStatus(block)
			assert.True(t, ok)
			assert.Equal(t, "completed", status)
		})

		mockey.PatchConvey("nil block", func() {
			status, ok := GetItemStatus(nil)
			assert.False(t, ok)
			assert.Equal(t, "", status)
		})
	})
}

func TestSetBlockExtraValue(t *testing.T) {
	mockey.PatchConvey("TestSetBlockExtraValue", t, func() {
		mockey.PatchConvey("valid block", func() {
			block := schema.NewContentBlock(&schema.AssistantGenText{Text: "test"})
			setBlockExtraValue(block, "test-key", "test-value")
			assert.NotNil(t, block.Extra)
			assert.Equal(t, "test-value", block.Extra["test-key"])
		})

		mockey.PatchConvey("nil block", func() {
			assert.NotPanics(t, func() {
				setBlockExtraValue[string](nil, "key", "value")
			})
		})

		mockey.PatchConvey("create extra map", func() {
			newBlock := &schema.ContentBlock{Type: schema.ContentBlockTypeAssistantGenText}
			assert.Nil(t, newBlock.Extra)
			setBlockExtraValue(newBlock, "new-key", 123)
			assert.NotNil(t, newBlock.Extra)
			assert.Equal(t, 123, newBlock.Extra["new-key"])
		})
	})
}

func TestGetBlockExtraValue(t *testing.T) {
	mockey.PatchConvey("TestGetBlockExtraValue", t, func() {
		block := schema.NewContentBlock(&schema.AssistantGenText{Text: "test"})
		setBlockExtraValue(block, "string-key", "string-value")
		setBlockExtraValue(block, "int-key", 42)

		mockey.PatchConvey("existing value", func() {
			val, ok := getBlockExtraValue[string](block, "string-key")
			assert.True(t, ok)
			assert.Equal(t, "string-value", val)

			intVal, ok := getBlockExtraValue[int](block, "int-key")
			assert.True(t, ok)
			assert.Equal(t, 42, intVal)
		})

		mockey.PatchConvey("non-existent value", func() {
			val, ok := getBlockExtraValue[string](block, "non-existent")
			assert.False(t, ok)
			assert.Equal(t, "", val)
		})

		mockey.PatchConvey("nil block", func() {
			val, ok := getBlockExtraValue[string](nil, "key")
			assert.False(t, ok)
			assert.Equal(t, "", val)
		})

		mockey.PatchConvey("nil extra", func() {
			emptyBlock := &schema.ContentBlock{Type: schema.ContentBlockTypeAssistantGenText}
			val, ok := getBlockExtraValue[string](emptyBlock, "key")
			assert.False(t, ok)
			assert.Equal(t, "", val)
		})

		mockey.PatchConvey("type mismatch", func() {
			wrongType, ok := getBlockExtraValue[int](block, "string-key")
			assert.False(t, ok)
			assert.Equal(t, 0, wrongType)
		})
	})
}

func TestConcatFirstNonZero(t *testing.T) {
	mockey.PatchConvey("TestConcatFirstNonZero", t, func() {
		mockey.PatchConvey("all zero values", func() {
			result, err := concatFirstNonZero([]string{"", "", ""})
			assert.NoError(t, err)
			assert.Equal(t, "", result)
		})

		mockey.PatchConvey("first non-zero", func() {
			result, err := concatFirstNonZero([]string{"first", "second", "third"})
			assert.NoError(t, err)
			assert.Equal(t, "first", result)
		})

		mockey.PatchConvey("middle non-zero", func() {
			result, err := concatFirstNonZero([]string{"", "middle", "last"})
			assert.NoError(t, err)
			assert.Equal(t, "middle", result)
		})

		mockey.PatchConvey("empty slice", func() {
			result, err := concatFirstNonZero([]string{})
			assert.NoError(t, err)
			assert.Equal(t, "", result)
		})

		mockey.PatchConvey("integers", func() {
			intResult, err := concatFirstNonZero([]int{0, 0, 5, 10})
			assert.NoError(t, err)
			assert.Equal(t, 5, intResult)
		})

		mockey.PatchConvey("pointers", func() {
			str1 := "value"
			var nilPtr *string
			ptrResult, err := concatFirstNonZero([]*string{nilPtr, &str1})
			assert.NoError(t, err)
			assert.Equal(t, &str1, ptrResult)
		})
	})
}

func TestConcatFirst(t *testing.T) {
	mockey.PatchConvey("TestConcatFirst", t, func() {
		mockey.PatchConvey("non-empty slice", func() {
			result, err := concatFirst([]string{"first", "second", "third"})
			assert.NoError(t, err)
			assert.Equal(t, "first", result)
		})

		mockey.PatchConvey("empty slice", func() {
			result, err := concatFirst([]string{})
			assert.NoError(t, err)
			assert.Equal(t, "", result)
		})

		mockey.PatchConvey("single element", func() {
			result, err := concatFirst([]string{"only"})
			assert.NoError(t, err)
			assert.Equal(t, "only", result)
		})

		mockey.PatchConvey("integers", func() {
			intResult, err := concatFirst([]int{1, 2, 3})
			assert.NoError(t, err)
			assert.Equal(t, 1, intResult)
		})
	})
}

func TestConcatLast(t *testing.T) {
	mockey.PatchConvey("TestConcatLast", t, func() {
		mockey.PatchConvey("non-empty slice", func() {
			result, err := concatLast([]string{"first", "second", "third"})
			assert.NoError(t, err)
			assert.Equal(t, "third", result)
		})

		mockey.PatchConvey("empty slice", func() {
			result, err := concatLast([]string{})
			assert.NoError(t, err)
			assert.Equal(t, "", result)
		})

		mockey.PatchConvey("single element", func() {
			result, err := concatLast([]string{"only"})
			assert.NoError(t, err)
			assert.Equal(t, "only", result)
		})

		mockey.PatchConvey("integers", func() {
			intResult, err := concatLast([]int{1, 2, 3})
			assert.NoError(t, err)
			assert.Equal(t, 3, intResult)
		})
	})
}
