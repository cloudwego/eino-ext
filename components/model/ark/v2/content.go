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

func toArkContent(msg *schema.Message) (chat.ChatCompletionMessageContent, error) {
	if err := validateMessageContent(msg); err != nil {
		return chat.ChatCompletionMessageContent{}, err
	}
	var parts []chat.ChatCompletionContentPart
	switch {
	case len(msg.UserInputMultiContent) > 0:
		for _, input := range msg.UserInputMultiContent {
			part, err := toInputPart(input)
			if err != nil {
				return chat.ChatCompletionMessageContent{}, err
			}
			parts = append(parts, part)
		}
	case len(msg.AssistantGenMultiContent) > 0:
		for _, output := range msg.AssistantGenMultiContent {
			if output.Type == schema.ChatMessagePartTypeReasoning {
				if output.Reasoning == nil {
					return chat.ChatCompletionMessageContent{}, fmt.Errorf("assistant reasoning must not be nil")
				}
				continue
			}
			part, err := toChatOutputPart(output)
			if err != nil {
				return chat.ChatCompletionMessageContent{}, err
			}
			parts = append(parts, part)
		}
	case len(msg.MultiContent) > 0:
		for _, legacy := range msg.MultiContent {
			part, err := toChatLegacyPart(legacy)
			if err != nil {
				return chat.ChatCompletionMessageContent{}, err
			}
			parts = append(parts, part)
		}
	default:
		return chat.NewStringChatCompletionMessageContent(msg.Content), nil
	}
	return chat.NewChatCompletionContentPartArrayChatCompletionMessageContent(parts), nil
}

func chatString(value string) chat.OptString {
	if value == "" {
		return chat.OptString{}
	}
	return chat.NewOptString(value)
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
		return chatImagePart(part.Image.MessagePartCommon, part.Image.Detail)
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("video must not be nil")
		}
		return chatVideoPart(part.Video.MessagePartCommon, GetInputVideoFPS(part.Video))
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("audio must not be nil")
		}
		raw, fileID, base64, err := mediaSource(part.Audio.MessagePartCommon, mediaKindAudio)
		if err != nil {
			return chat.ChatCompletionContentPart{}, err
		}
		audio := chat.ChatCompletionContentPartAudioInputAudio{FileID: chatString(fileID)}
		if base64 {
			audio.Data = chatString(raw)
		} else {
			audio.URL = chatString(raw)
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
		file := chat.ChatCompletionContentPartFileFile{FileID: chatString(fileID)}
		if base64 {
			if part.File.Name == "" {
				return chat.ChatCompletionContentPart{}, fmt.Errorf("file Name is required with Base64Data")
			}
			file.FileData = chat.NewOptString(mediaDataURL(part.File.MIMEType, raw))
		} else {
			file.FileURL = chatString(raw)
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

func chatTextPart(text string) chat.ChatCompletionContentPart {
	var sum chat.ChatCompletionContentPartSum
	sum.SetChatCompletionContentPartText(chat.ChatCompletionContentPartText{Type: chat.ChatCompletionContentPartTextTypeText, Text: text})
	return chat.ChatCompletionContentPart{OneOf: sum}
}

func chatImagePart(common schema.MessagePartCommon, detail schema.ImageURLDetail) (chat.ChatCompletionContentPart, error) {
	var sum chat.ChatCompletionContentPartSum
	url, fileID, base64, err := mediaSource(common, mediaKindImage)
	if err != nil {
		return chat.ChatCompletionContentPart{}, err
	}
	if base64 {
		url = mediaDataURL(common.MIMEType, url)
	}
	image := chat.ChatCompletionContentPartImageImageUrl{URL: chatString(url), FileID: chatString(fileID)}
	if detail != "" {
		image.Detail = chat.NewOptImageDetail(chat.ImageDetail(detail))
	}
	sum = chat.NewChatCompletionContentPartImageChatCompletionContentPartSum(chat.ChatCompletionContentPartImage{
		Type: chat.ChatCompletionContentPartImageTypeImageURL, ImageURL: chat.NewChatCompletionContentPartImageImageUrlChatCompletionContentPartImageImageURL(image),
	})
	return chat.ChatCompletionContentPart{OneOf: sum}, nil
}

func chatVideoPart(common schema.MessagePartCommon, fps *float64) (chat.ChatCompletionContentPart, error) {
	var sum chat.ChatCompletionContentPartSum
	url, fileID, base64, err := mediaSource(common, mediaKindVideo)
	if err != nil {
		return chat.ChatCompletionContentPart{}, err
	}
	if base64 {
		url = mediaDataURL(common.MIMEType, url)
	}
	video := chat.ChatCompletionContentPartVideoVideoUrl{URL: chatString(url), FileID: chatString(fileID)}
	if fps != nil {
		video.Fps = chat.NewOptFloat64(*fps)
	}
	sum = chat.NewChatCompletionContentPartVideoChatCompletionContentPartSum(chat.ChatCompletionContentPartVideo{Type: chat.ChatCompletionContentPartVideoTypeVideoURL, VideoURL: video})
	return chat.ChatCompletionContentPart{OneOf: sum}, nil
}

func toChatOutputPart(part schema.MessageOutputPart) (chat.ChatCompletionContentPart, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return chatTextPart(part.Text), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("assistant image must not be nil")
		}
		return chatImagePart(part.Image.MessagePartCommon, "")
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("assistant video must not be nil")
		}
		return chatVideoPart(part.Video.MessagePartCommon, GetOutputVideoFPS(part.Video))
	default:
		return chat.ChatCompletionContentPart{}, fmt.Errorf("unsupported assistant part type %s", part.Type)
	}
}

func toChatLegacyPart(part schema.ChatMessagePart) (chat.ChatCompletionContentPart, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return chatTextPart(part.Text), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.ImageURL == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("ImageURL must not be nil")
		}
		return chatImagePart(schema.MessagePartCommon{URL: &part.ImageURL.URL}, part.ImageURL.Detail)
	case schema.ChatMessagePartTypeVideoURL:
		if part.VideoURL == nil {
			return chat.ChatCompletionContentPart{}, fmt.Errorf("VideoURL must not be nil")
		}
		return chatVideoPart(schema.MessagePartCommon{URL: &part.VideoURL.URL}, getFPS(part.VideoURL.Extra))
	default:
		return chat.ChatCompletionContentPart{}, fmt.Errorf("unsupported legacy part type %s", part.Type)
	}
}
