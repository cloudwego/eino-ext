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
	setResponseID(msgs[0], "resp id")
	setResponseCaching(msgs[0], cachingEnabled)
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

	caching_, ok := getResponseCaching(msg)
	assert.Equal(t, true, ok)
	assert.Equal(t, string(cachingEnabled), caching_)

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
		setInputVideoFPS(inputVideo, 3.0)
		assert.Equal(t, ptrOf(3.0), GetInputVideoFPS(inputVideo))

		// Boundary case: nil input
		setInputVideoFPS(nil, 3.0)
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
