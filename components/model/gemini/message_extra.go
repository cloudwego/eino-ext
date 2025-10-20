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

package gemini

import (
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

const videoMetaDataKey = "gemini_video_meta_data"

// Deprecated: use SetInputVideoMetaData or SetOutputVideoMetaData instead.
func SetVideoMetaData(part *schema.ChatMessageVideoURL, metaData *genai.VideoMetadata) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setVideoMetaData(part.Extra, metaData)
}

// Deprecated: use GetInputVideoMetaData or GetOutputVideoMetaData instead.
func GetVideoMetaData(part *schema.ChatMessageVideoURL) *genai.VideoMetadata {
	if part == nil || part.Extra == nil {
		return nil
	}
	return getVideoMetaData(part.Extra)
}

func setInputVideoMetaData(part *schema.MessageInputVideo, metaData *genai.VideoMetadata) {
	if part == nil {
		return
	}
	if part.Extra == nil {
		part.Extra = make(map[string]any)
	}
	setVideoMetaData(part.Extra, metaData)
}

func GetInputVideoMetaData(part *schema.MessageInputVideo) *genai.VideoMetadata {
	if part == nil || part.Extra == nil {
		return nil
	}
	return getVideoMetaData(part.Extra)
}

func setVideoMetaData(extra map[string]any, metaData *genai.VideoMetadata) {
	extra[videoMetaDataKey] = metaData
}

func getVideoMetaData(extra map[string]any) *genai.VideoMetadata {
	if extra == nil {
		return nil
	}
	videoMetaData, ok := extra[videoMetaDataKey].(*genai.VideoMetadata)
	if !ok {
		return nil
	}
	return videoMetaData
}
