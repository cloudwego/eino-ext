/*
 * Copyright 2026 CloudWeGo Authors
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
	"encoding/json"
	"fmt"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/schema"
)

// The pinned SDK does not expose Chat file-reference fields. Its ChatRequest
// interface lets us extend serialization while retaining the SDK transport.
type chatCompletionRequest struct {
	model.CreateChatCompletionRequest
	contents map[int]*chatCompletionMessageContent
}

var _ model.ChatRequest = chatCompletionRequest{}

func (r chatCompletionRequest) MarshalJSON() ([]byte, error) {
	if len(r.contents) == 0 {
		return r.CreateChatCompletionRequest.MarshalJSON()
	}
	type sdkRequest model.CreateChatCompletionRequest
	messages := make([]any, len(r.Messages))
	for i, message := range r.Messages {
		if content, ok := r.contents[i]; ok {
			messages[i] = struct {
				*model.ChatCompletionMessage
				Content *chatCompletionMessageContent `json:"content"`
			}{message, content}
		} else {
			messages[i] = message
		}
	}
	return json.Marshal(struct {
		*sdkRequest
		Messages []any `json:"messages"`
	}{(*sdkRequest)(&r.CreateChatCompletionRequest), messages})
}

// Override the embedded SDK method so streaming retains the extended content.
func (r chatCompletionRequest) WithStream(stream bool) model.ChatRequest {
	r.Stream = &stream
	return r
}

type chatCompletionMessageContent struct {
	model.ChatCompletionMessageContent
	fileIDParts map[int]*chatCompletionFileIDPart
}

func (c chatCompletionMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.fileIDParts) == 0 {
		return c.ChatCompletionMessageContent.MarshalJSON()
	}
	parts := make([]any, len(c.ListValue))
	for i, part := range c.ListValue {
		if fileIDPart, ok := c.fileIDParts[i]; ok {
			parts[i] = fileIDPart
		} else {
			parts[i] = part
		}
	}
	return json.Marshal(parts)
}

type chatCompletionFileIDPart struct {
	Type       model.ChatCompletionMessageContentPartType `json:"type"`
	ImageURL   *chatCompletionFileIDSource                `json:"image_url,omitempty"`
	VideoURL   *chatCompletionFileIDSource                `json:"video_url,omitempty"`
	InputAudio *chatCompletionFileIDSource                `json:"input_audio,omitempty"`
	File       *chatCompletionFileIDSource                `json:"file,omitempty"`
}

type chatCompletionFileIDSource struct {
	FileID   string               `json:"file_id"`
	Detail   model.ImageURLDetail `json:"detail,omitempty"`
	FPS      *float64             `json:"fps,omitempty"`
	Filename string               `json:"filename,omitempty"`
}

func toChatFileIDPart(part schema.MessageInputPart) (*chatCompletionFileIDPart, error) {
	var common schema.MessagePartCommon
	switch {
	case part.Type == schema.ChatMessagePartTypeImageURL && part.Image != nil:
		common = part.Image.MessagePartCommon
	case part.Type == schema.ChatMessagePartTypeVideoURL && part.Video != nil:
		common = part.Video.MessagePartCommon
	case part.Type == schema.ChatMessagePartTypeAudioURL && part.Audio != nil:
		common = part.Audio.MessagePartCommon
	case part.Type == schema.ChatMessagePartTypeFileURL && part.File != nil:
		common = part.File.MessagePartCommon
	default:
		return nil, nil
	}
	// Preserve the existing URL, then Base64Data selection before FileID.
	if common.URL != nil && *common.URL != "" || common.Base64Data != nil && *common.Base64Data != "" || common.FileID == nil {
		return nil, nil
	}
	if *common.FileID == "" {
		return nil, fmt.Errorf("FileID must not be empty")
	}
	source := &chatCompletionFileIDSource{FileID: *common.FileID}
	result := &chatCompletionFileIDPart{}
	switch part.Type {
	case schema.ChatMessagePartTypeImageURL:
		result.Type = model.ChatCompletionMessageContentPartTypeImageURL
		source.Detail = model.ImageURLDetail(part.Image.Detail)
		result.ImageURL = source
	case schema.ChatMessagePartTypeVideoURL:
		result.Type = model.ChatCompletionMessageContentPartTypeVideoURL
		source.FPS = GetInputVideoFPS(part.Video)
		result.VideoURL = source
	case schema.ChatMessagePartTypeAudioURL:
		result.Type = model.ChatCompletionMessageContentPartTypeAudioURL
		result.InputAudio = source
	case schema.ChatMessagePartTypeFileURL:
		result.Type = "file"
		source.Filename = part.File.Name
		result.File = source
	}
	return result, nil
}
