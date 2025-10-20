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

package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestVideoMetaDataFunctions(t *testing.T) {
	ptr := func(f float64) *float64 { return &f }

	t.Run("TestSetVideoMetaData", func(t *testing.T) {
		videoURL := &schema.ChatMessageVideoURL{}

		// Success case
		metaData := &genai.VideoMetadata{FPS: ptr(24.0)}
		SetVideoMetaData(videoURL, metaData)
		assert.Equal(t, metaData, GetVideoMetaData(videoURL))

		// Boundary case: nil input
		SetVideoMetaData(nil, metaData)
		assert.Nil(t, GetVideoMetaData(nil))
	})

	t.Run("TestSetInputVideoMetaData", func(t *testing.T) {
		inputVideo := &schema.MessageInputVideo{}

		// Success case
		metaData := &genai.VideoMetadata{FPS: ptr(10.0)}
		setInputVideoMetaData(inputVideo, metaData)
		assert.Equal(t, metaData, GetInputVideoMetaData(inputVideo))

		// Boundary case: nil input
		setInputVideoMetaData(nil, metaData)
		assert.Nil(t, GetInputVideoMetaData(nil))
	})
}
