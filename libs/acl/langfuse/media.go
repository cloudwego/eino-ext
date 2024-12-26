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

package langfuse

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

type fieldType string

const (
	fieldTypeInput    = "input"
	fieldTypeOutput   = "output"
	fieldTypeMetadata = "metadata"
)

func tryNewMediaFromBase64(data string) *media {
	if !strings.HasPrefix(data, "data:") {
		return nil
	}
	contents := strings.SplitN(data[5:], ",", 2)
	if len(contents) != 2 {
		return nil
	}
	headParts := strings.Split(contents[0], ";")
	bBase64 := false
	for _, part := range headParts {
		if part == "base64" {
			bBase64 = true
		}
	}
	if !bBase64 {
		return nil
	}

	contentBytes, err := base64.StdEncoding.DecodeString(contents[1])
	if err != nil {
		return nil
	}
	hasher := sha256.New()
	hasher.Write(contentBytes)
	hash := hasher.Sum(nil)
	return &media{
		contentBytes:      contentBytes,
		contentType:       headParts[0],
		source:            "base64_data_uri",
		contentSHA256Hash: base64.StdEncoding.EncodeToString(hash),
	}
}

type media struct {
	contentBytes      []byte
	contentType       string
	source            string
	mediaID           string
	contentSHA256Hash string
}

type mediaMessage struct {
	Content []schema.ChatMessagePart `json:"content"`

	Role         schema.RoleType      `json:"role"`
	Name         string               `json:"name,omitempty"`
	ToolCalls    []schema.ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string               `json:"tool_call_id,omitempty"`
	ResponseMeta *schema.ResponseMeta `json:"response_meta,omitempty"`
	Extra        map[string]any       `json:"extra,omitempty"`
}

func convMessageIfNeeded(message *schema.Message) any {
	if len(message.MultiContent) > 0 {
		return &mediaMessage{
			Role:         message.Role,
			Content:      message.MultiContent,
			Name:         message.Name,
			ToolCalls:    message.ToolCalls,
			ToolCallID:   message.ToolCallID,
			ResponseMeta: message.ResponseMeta,
			Extra:        message.Extra,
		}
	}
	return message
}

func marshalMessage(message *schema.Message) (string, error) {
	return sonic.MarshalString(convMessageIfNeeded(message))
}

func marshalMessages(messages []*schema.Message) (string, error) {
	var nMessages []any
	for _, message := range messages {
		nMessages = append(nMessages, convMessageIfNeeded(message))
	}
	return sonic.MarshalString(nMessages)
}
