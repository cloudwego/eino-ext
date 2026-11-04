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
	"strings"

	"github.com/bytedance/sonic"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Modified JSON must retain fields the SDK does not know about, while local
// file URLs must reflect SDK preprocessing. Patch only those media sources.
type responsesFileInput struct {
	input *responses.ResponsesInput
	raw   json.RawMessage
	files []responsesFilePart
}

type responsesFilePart struct{ item, content int }

func responsesInputWithFiles(input *responses.ResponsesInput, raw json.RawMessage) interface{} {
	items, _ := input.GetInputItemArray()
	var files []responsesFilePart
	for i := range items {
		message, ok := items[i].OneOf.GetItemEasyMessage()
		if !ok {
			continue
		}
		parts, _ := message.Content.GetContentItemArray()
		for j := range parts {
			_, url, _ := responsesFileSource(parts[j])
			if strings.HasPrefix(url, "file://") {
				files = append(files, responsesFilePart{item: i, content: j})
			}
		}
	}
	if len(files) == 0 {
		return raw
	}
	return &responsesFileInput{input: input, raw: raw, files: files}
}

func (in *responsesFileInput) MarshalJSON() ([]byte, error) {
	var rawItems []json.RawMessage
	if err := sonic.Unmarshal(in.raw, &rawItems); err != nil {
		return nil, err
	}
	items, _ := in.input.GetInputItemArray()
	for _, file := range in.files {
		part := items[file.item].OneOf.ItemEasyMessage.Content.ContentItemArray[file.content]
		key, url, id := responsesFileSource(part)
		if url != "" || !id.IsSet() {
			continue
		}
		var message map[string]json.RawMessage
		if err := sonic.Unmarshal(rawItems[file.item], &message); err != nil {
			return nil, err
		}
		var parts []map[string]json.RawMessage
		if err := sonic.Unmarshal(message["content"], &parts); err != nil {
			return nil, err
		}
		delete(parts[file.content], key)
		var err error
		parts[file.content]["file_id"], err = sonic.Marshal(id.Value)
		if err != nil {
			return nil, err
		}
		message["content"], err = sonic.Marshal(parts)
		if err != nil {
			return nil, err
		}
		rawItems[file.item], err = sonic.Marshal(message)
		if err != nil {
			return nil, err
		}
	}
	return sonic.Marshal(rawItems)
}

func responsesFileSource(part responses.ContentItem) (key, url string, id responses.OptString) {
	switch part.OneOf.Type {
	case responses.ContentItemImageContentItemSum:
		image := part.OneOf.ContentItemImage
		return "image_url", image.ImageURL.Or(""), image.FileID
	case responses.ContentItemVideoContentItemSum:
		video := part.OneOf.ContentItemVideo
		return "video_url", video.VideoURL, video.FileID
	case responses.ContentItemFileContentItemSum:
		file := part.OneOf.ContentItemFile
		return "file_url", file.FileURL.Or(""), file.FileID
	default:
		return "", "", responses.OptString{}
	}
}
