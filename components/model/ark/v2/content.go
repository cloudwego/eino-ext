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
	"fmt"
	"mime"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

type mediaKind string

const (
	mediaKindImage mediaKind = "image"
	mediaKindAudio mediaKind = "audio"
	mediaKindVideo mediaKind = "video"
	mediaKindFile  mediaKind = "file"
)

func toArkContent(msg *schema.Message) (chat.ChatCompletionMessageContent, error) {
	empty := chat.ChatCompletionMessageContent{}
	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return empty, fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}
	var parts []chat.ChatCompletionContentPart
	if len(msg.UserInputMultiContent) > 0 {
		if msg.Role != schema.User && msg.Role != schema.Tool {
			return empty, fmt.Errorf("user input multi content only supports user and tool roles, got %s", msg.Role)
		}
		for _, part := range msg.UserInputMultiContent {
			converted, err := toInputPart(part)
			if err != nil {
				return empty, err
			}
			parts = append(parts, converted)
		}
	} else if len(msg.AssistantGenMultiContent) > 0 {
		if msg.Role != schema.Assistant {
			return empty, fmt.Errorf("assistant gen multi content only supports assistant role, got %s", msg.Role)
		}
		for _, part := range msg.AssistantGenMultiContent {
			input := schema.MessageInputPart{Type: part.Type, Text: part.Text}
			switch part.Type {
			case schema.ChatMessagePartTypeText:
			case schema.ChatMessagePartTypeImageURL:
				if part.Image == nil {
					return empty, fmt.Errorf("assistant image must not be nil")
				}
				input.Image = &schema.MessageInputImage{MessagePartCommon: part.Image.MessagePartCommon}
			case schema.ChatMessagePartTypeVideoURL:
				if part.Video == nil {
					return empty, fmt.Errorf("assistant video must not be nil")
				}
				input.Video = &schema.MessageInputVideo{MessagePartCommon: part.Video.MessagePartCommon}
			default:
				return empty, fmt.Errorf("unsupported assistant part type %s", part.Type)
			}
			converted, err := toInputPart(input)
			if err != nil {
				return empty, err
			}
			parts = append(parts, converted)
		}
	} else if len(msg.MultiContent) > 0 {
		// Preserve the deprecated text/image/video representation for existing callers.
		for _, part := range msg.MultiContent {
			input := schema.MessageInputPart{Type: part.Type, Text: part.Text}
			switch part.Type {
			case schema.ChatMessagePartTypeText:
			case schema.ChatMessagePartTypeImageURL:
				if part.ImageURL == nil {
					return empty, fmt.Errorf("ImageURL must not be nil")
				}
				input.Image = &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &part.ImageURL.URL}, Detail: part.ImageURL.Detail}
			case schema.ChatMessagePartTypeVideoURL:
				if part.VideoURL == nil {
					return empty, fmt.Errorf("VideoURL must not be nil")
				}
				input.Video = &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &part.VideoURL.URL, Extra: part.VideoURL.Extra}}
			default:
				return empty, fmt.Errorf("unsupported legacy part type %s", part.Type)
			}
			converted, err := toInputPart(input)
			if err != nil {
				return empty, err
			}
			parts = append(parts, converted)
		}
	} else {
		return chat.NewStringChatCompletionMessageContent(msg.Content), nil
	}
	return chat.NewChatCompletionContentPartArrayChatCompletionMessageContent(parts), nil
}

func toInputPart(part schema.MessageInputPart) (chat.ChatCompletionContentPart, error) {
	var sum chat.ChatCompletionContentPartSum
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		sum = chat.NewChatCompletionContentPartTextChatCompletionContentPartSum(chat.ChatCompletionContentPartText{Type: chat.ChatCompletionContentPartTextTypeText, Text: part.Text})
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("image must not be nil")
		}
		url, fileID, _, err := mediaSource(part.Image.MessagePartCommon, mediaKindImage)
		if err != nil {
			return chat.ChatCompletionContentPart{}, err
		}
		image := chat.ChatCompletionContentPartImageImageUrl{URL: url, FileID: fileID}
		if part.Image.Detail != "" {
			image.Detail = chat.NewOptImageDetail(chat.ImageDetail(part.Image.Detail))
		}
		sum = chat.NewChatCompletionContentPartImageChatCompletionContentPartSum(chat.ChatCompletionContentPartImage{
			Type: chat.ChatCompletionContentPartImageTypeImageURL, ImageURL: chat.NewChatCompletionContentPartImageImageUrlChatCompletionContentPartImageImageURL(image),
		})
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("video must not be nil")
		}
		url, fileID, _, err := mediaSource(part.Video.MessagePartCommon, mediaKindVideo)
		if err != nil {
			return chat.ChatCompletionContentPart{}, err
		}
		video := chat.ChatCompletionContentPartVideoVideoUrl{URL: url, FileID: fileID}
		if fps := GetInputVideoFPS(part.Video); fps != nil {
			video.Fps = chat.NewOptFloat64(*fps)
		}
		sum = chat.NewChatCompletionContentPartVideoChatCompletionContentPartSum(chat.ChatCompletionContentPartVideo{Type: chat.ChatCompletionContentPartVideoTypeVideoURL, VideoURL: video})
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("audio must not be nil")
		}
		raw, fileID, base64, err := mediaSource(part.Audio.MessagePartCommon, mediaKindAudio)
		if err != nil {
			return chat.ChatCompletionContentPart{}, err
		}
		audio := chat.ChatCompletionContentPartAudioInputAudio{FileID: fileID}
		if base64 {
			audio.Data = raw
		} else {
			audio.URL = raw
		}
		if part.Audio.MIMEType != "" {
			format := audioFormatFromMIME(part.Audio.MIMEType)
			audio.Format = chat.NewOptString(format)
		}
		sum = chat.NewChatCompletionContentPartAudioChatCompletionContentPartSum(chat.ChatCompletionContentPartAudio{Type: chat.ChatCompletionContentPartAudioTypeInputAudio, InputAudio: audio})
	case schema.ChatMessagePartTypeFileURL:
		if part.File == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("file must not be nil")
		}
		raw, fileID, base64, err := mediaSource(part.File.MessagePartCommon, mediaKindFile)
		if err != nil {
			return chat.ChatCompletionContentPart{}, err
		}
		file := chat.ChatCompletionContentPartFileFile{FileID: fileID}
		if base64 {
			if part.File.Name == "" {
				return chat.ChatCompletionContentPart{}, fmt.Errorf("file Name is required with Base64Data")
			}
			file.FileData = raw
		} else {
			file.FileURL = raw
		}
		if part.File.Name != "" {
			file.Filename = chat.NewOptString(part.File.Name)
		}
		sum = chat.NewChatCompletionContentPartFileChatCompletionContentPartSum(chat.ChatCompletionContentPartFile{Type: chat.ChatCompletionContentPartFileTypeFile, File: file})
	default:
		return chat.ChatCompletionContentPart{}, fmt.Errorf("unsupported input part type %s", part.Type)
	}
	return chat.ChatCompletionContentPart{OneOf: sum}, nil
}

// Select the first non-empty source: URL, then Base64Data, then FileID.
// Only the selected source is serialized; validation failures do not fall through.
func mediaSource(c schema.MessagePartCommon, kind mediaKind) (value, fileID chat.OptString, base64 bool, err error) {
	if c.URL != nil && *c.URL != "" {
		return chat.NewOptString(*c.URL), fileID, false, nil
	}
	if c.Base64Data != nil && *c.Base64Data != "" {
		raw := *c.Base64Data
		if strings.HasPrefix(raw, "data:") {
			return value, fileID, true, fmt.Errorf("%s Base64Data must be raw base64, without a data URL prefix", kind)
		}
		if kind != mediaKindFile && c.MIMEType == "" {
			return value, fileID, true, fmt.Errorf("%s part must have MIMEType when using Base64Data", kind)
		}
		if kind == mediaKindImage || kind == mediaKindVideo {
			raw = fmt.Sprintf("data:%s;base64,%s", c.MIMEType, raw)
		}
		return chat.NewOptString(raw), fileID, true, nil
	}
	if c.FileID != nil && *c.FileID != "" {
		return value, chat.NewOptString(*c.FileID), false, nil
	}
	return value, fileID, false, fmt.Errorf("%s part must contain URL, Base64Data or FileID", kind)
}

func audioFormatFromMIME(value string) string {
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		value = parsed
	}
	// Ark uses format names, which differ from the MIME suffix for these formats.
	switch value {
	case "audio/mpeg":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "audio/L16", "audio/l16":
		return "pcm"
	}
	return strings.TrimPrefix(value, "audio/")
}
