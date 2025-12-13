/*
 * Copyright 2025 CloudWeGo Authors
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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/schema"
)

func TestConcatMessages(t *testing.T) {
	msgs := []*schema.Message{
		{},
		{},
	}

	setArkRequestID(msgs[0], "123456")
	setArkRequestID(msgs[1], "123456")
	setReasoningContent(msgs[0], "how ")
	setReasoningContent(msgs[1], "are you")
	setModelName(msgs[0], "model name")
	setModelName(msgs[1], "model name")
	setServiceTier(msgs[0], "service tier")
	setServiceTier(msgs[1], "service tier")

	setResponseCacheExpireAt(msgs[0], arkResponseCacheExpireAt(10))
	setResponseCacheExpireAt(msgs[1], arkResponseCacheExpireAt(10))
	setResponseID(msgs[0], "resp id")
	setResponseID(msgs[1], "resp id")
	setContextID(msgs[0], "context id")
	setContextID(msgs[1], "context id")

	msg, err := schema.ConcatMessages(msgs)
	assert.NoError(t, err)
	assert.Equal(t, "123456", GetArkRequestID(msg))

	reasoningContent, ok := GetReasoningContent(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "how are you", reasoningContent)

	modelName, ok := GetModelName(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "model name", modelName)

	serviceTier, ok := GetServiceTier(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "service tier", serviceTier)

	responseID, ok := GetResponseID(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "resp id", responseID)

	expireAt, ok := GetCacheExpiration(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(10), expireAt)

	respID, ok := GetResponseID(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "resp id", respID)

	respID, ok = GetResponseID(&schema.Message{
		Extra: map[string]any{
			keyOfResponseID: "resp id",
		},
	})
	assert.Equal(t, true, ok)
	assert.Equal(t, "resp id", respID)

	contextID, ok := GetContextID(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, "context id", contextID)

	contextID, ok = GetContextID(&schema.Message{
		Extra: map[string]any{
			keyOfContextID: "context id",
		},
	})
	assert.Equal(t, true, ok)
	assert.Equal(t, "context id", contextID)

	expireAt, ok = GetCacheExpiration(&schema.Message{
		Extra: map[string]any{
			keyOfResponseCacheExpireAt: int64(10),
		},
	})
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(10), expireAt)
}

func TestImageSizeFunctions(t *testing.T) {
	t.Run("TestImageSize", func(t *testing.T) {
		imgURL := &schema.ChatMessageImageURL{}
		size := "1024x1024"

		// Test Set and Get
		SetImageSize(imgURL, size)
		retrievedSize, ok := GetImageSize(imgURL)
		assert.True(t, ok)
		assert.Equal(t, size, retrievedSize)

		// Test Get on new object
		newImgURL := &schema.ChatMessageImageURL{}
		_, ok = GetImageSize(newImgURL)
		assert.False(t, ok)

		// Test on nil object
		SetImageSize(nil, size)
		_, ok = GetImageSize(nil)
		assert.False(t, ok)
	})

	t.Run("TestInputImageSize", func(t *testing.T) {
		inputImg := &schema.MessageInputImage{}
		size := "2048x2048"

		// Test Set and Get
		setInputImageSize(inputImg, size)
		retrievedSize, ok := GetInputImageSize(inputImg)
		assert.True(t, ok)
		assert.Equal(t, size, retrievedSize)

		// Test Get on new object
		newInputImg := &schema.MessageInputImage{}
		_, ok = GetInputImageSize(newInputImg)
		assert.False(t, ok)

		// Test on nil object
		setInputImageSize(nil, size)
		_, ok = GetInputImageSize(nil)
		assert.False(t, ok)
	})

	t.Run("TestOutputImageSize", func(t *testing.T) {
		outputImg := &schema.MessageOutputImage{}
		size := "4096x4096"

		// Test Set and Get
		setOutputImageSize(outputImg, size)
		retrievedSize, ok := GetOutputImageSize(outputImg)
		assert.True(t, ok)
		assert.Equal(t, size, retrievedSize)

		// Test Get on new object
		newOutputImg := &schema.MessageOutputImage{}
		_, ok = GetOutputImageSize(newOutputImg)
		assert.False(t, ok)

		// Test on nil object
		setOutputImageSize(nil, size)
		_, ok = GetOutputImageSize(nil)
		assert.False(t, ok)
	})
}

func TestFPSFunctions(t *testing.T) {
	t.Run("TestSetFPS", func(t *testing.T) {
		videoURL := &schema.ChatMessageVideoURL{}

		// Success case
		SetFPS(videoURL, 2.5)
		assert.Equal(t, ptrOf(2.5), GetFPS(videoURL))

		// Boundary case: nil input
		SetFPS(nil, 2.5)
		assert.Nil(t, GetFPS(nil))
	})

	t.Run("TestSetInputVideoFPS", func(t *testing.T) {
		inputVideo := &schema.MessageInputVideo{}

		// Success case
		SetInputVideoFPS(inputVideo, 3.0)
		assert.Equal(t, ptrOf(3.0), GetInputVideoFPS(inputVideo))

		// Boundary case: nil input
		SetInputVideoFPS(nil, 3.0)
		assert.Nil(t, GetInputVideoFPS(nil))
	})

	t.Run("TestSetOutputVideoFPS", func(t *testing.T) {
		outputVideo := &schema.MessageOutputVideo{}

		// Success case
		setOutputVideoFPS(outputVideo, 4.0)
		assert.Equal(t, ptrOf(4.0), GetOutputVideoFPS(outputVideo))

		// Boundary case: nil input
		setOutputVideoFPS(nil, 4.0)
		assert.Nil(t, GetOutputVideoFPS(nil))
	})
}

func TestInvalidateMessageCaches(t *testing.T) {
	msgs := []*schema.Message{
		{
			Extra: map[string]any{
				keyOfResponseID:            "1",
				keyOfResponseCacheExpireAt: int64(10),
			},
		},
		{
			Extra: map[string]any{
				keyOfResponseID:            "2",
				keyOfResponseCacheExpireAt: int64(10),
			},
		},
	}

	err := InvalidateMessageCaches(msgs)
	assert.Nil(t, err)
	for _, msg := range msgs {
		assert.Nil(t, msg.Extra[keyOfResponseCacheExpireAt])
	}
}
