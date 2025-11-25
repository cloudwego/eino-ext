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

func TestSetItemID(t *testing.T) {
	mockey.PatchConvey("TestSetItemID", t, func() {
		mockey.PatchConvey("set value into Extra", func() {
			block := &schema.ContentBlock{}
			setItemID(block, "id-1")

			val, ok := block.Extra[itemIDKey]
			assert.True(t, ok)
			assert.Equal(t, blockExtraItemID("id-1"), val)
		})

		mockey.PatchConvey("nil block should not panic", func() {
			assert.NotPanics(t, func() {
				setItemID(nil, "id-1")
			})
		})
	})
}

func TestGetItemID(t *testing.T) {
	mockey.PatchConvey("TestGetItemID", t, func() {
		mockey.PatchConvey("found", func() {
			block := &schema.ContentBlock{Extra: map[string]any{itemIDKey: blockExtraItemID("id-2")}}
			itemID, ok := getItemID(block)
			assert.True(t, ok)
			assert.Equal(t, "id-2", itemID)
		})

		mockey.PatchConvey("not found", func() {
			block := &schema.ContentBlock{Extra: map[string]any{}}
			itemID, ok := getItemID(block)
			assert.False(t, ok)
			assert.Equal(t, "", itemID)
		})
	})
}

func TestSetItemStatus(t *testing.T) {
	mockey.PatchConvey("TestSetItemStatus", t, func() {
		mockey.PatchConvey("set value into Extra", func() {
			block := &schema.ContentBlock{}
			setItemStatus(block, "in_progress")

			val, ok := block.Extra[itemStatusKey]
			assert.True(t, ok)
			assert.Equal(t, blockExtraItemStatus("in_progress"), val)
		})

		mockey.PatchConvey("nil block should not panic", func() {
			assert.NotPanics(t, func() {
				setItemStatus(nil, "in_progress")
			})
		})
	})
}

func TestGetItemStatus(t *testing.T) {
	mockey.PatchConvey("TestGetItemStatus", t, func() {
		mockey.PatchConvey("found", func() {
			block := &schema.ContentBlock{Extra: map[string]any{itemStatusKey: blockExtraItemStatus("completed")}}
			status, ok := GetItemStatus(block)
			assert.True(t, ok)
			assert.Equal(t, "completed", status)
		})

		mockey.PatchConvey("not found", func() {
			block := &schema.ContentBlock{Extra: map[string]any{}}
			status, ok := GetItemStatus(block)
			assert.False(t, ok)
			assert.Equal(t, "", status)
		})
	})
}

func TestSetBlockExtraValue(t *testing.T) {
	mockey.PatchConvey("TestSetBlockExtraValue", t, func() {
		mockey.PatchConvey("nil block should not panic", func() {
			assert.NotPanics(t, func() {
				setBlockExtraValue[*schema.ContentBlock](nil, "k", nil)
			})
		})

		mockey.PatchConvey("init Extra map when nil", func() {
			block := &schema.ContentBlock{}
			setBlockExtraValue(block, "k", 123)
			assert.Equal(t, 123, block.Extra["k"])
		})
	})
}

func TestGetBlockExtraValue(t *testing.T) {
	mockey.PatchConvey("TestGetBlockExtraValue", t, func() {
		mockey.PatchConvey("nil block", func() {
			v, ok := getBlockExtraValue[int](nil, "k")
			assert.False(t, ok)
			assert.Equal(t, 0, v)
		})

		mockey.PatchConvey("type mismatch", func() {
			block := &schema.ContentBlock{Extra: map[string]any{"k": "v"}}
			v, ok := getBlockExtraValue[int](block, "k")
			assert.False(t, ok)
			assert.Equal(t, 0, v)
		})
	})
}

func TestConcatFirstNonZero(t *testing.T) {
	mockey.PatchConvey("TestConcatFirstNonZero", t, func() {
		mockey.PatchConvey("pick first non-zero", func() {
			v, err := concatFirstNonZero([]blockExtraItemID{"", "id"})
			assert.NoError(t, err)
			assert.Equal(t, blockExtraItemID("id"), v)
		})

		mockey.PatchConvey("all zero", func() {
			v, err := concatFirstNonZero([]blockExtraItemID{"", ""})
			assert.NoError(t, err)
			assert.Equal(t, blockExtraItemID(""), v)
		})
	})
}

func TestConcatLast(t *testing.T) {
	mockey.PatchConvey("TestConcatLast", t, func() {
		mockey.PatchConvey("non-empty", func() {
			v, err := concatLast([]blockExtraItemStatus{"a", "b"})
			assert.NoError(t, err)
			assert.Equal(t, blockExtraItemStatus("b"), v)
		})

		mockey.PatchConvey("empty", func() {
			v, err := concatLast([]blockExtraItemStatus{})
			assert.NoError(t, err)
			assert.Equal(t, blockExtraItemStatus(""), v)
		})
	})
}
