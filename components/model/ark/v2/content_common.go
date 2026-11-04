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
	"strings"

	"github.com/cloudwego/eino/schema"
)

type mediaKind string

const (
	mediaKindImage mediaKind = "image"
	mediaKindAudio mediaKind = "audio"
	mediaKindVideo mediaKind = "video"
	mediaKindFile  mediaKind = "file"
)

// Validate content ownership without converting inputs and outputs to one type.
func validateMessageContent(msg *schema.Message) error {
	if len(msg.UserInputMultiContent) > 0 && len(msg.AssistantGenMultiContent) > 0 {
		return fmt.Errorf("a message cannot contain both UserInputMultiContent and AssistantGenMultiContent")
	}
	if len(msg.UserInputMultiContent) > 0 && msg.Role != schema.User && msg.Role != schema.Tool {
		return fmt.Errorf("user input multi content only supports user and tool roles, got %s", msg.Role)
	}
	if len(msg.AssistantGenMultiContent) > 0 && msg.Role != schema.Assistant {
		return fmt.Errorf("assistant gen multi content only supports assistant role, got %s", msg.Role)
	}
	return nil
}

// Select URL, then raw Base64Data, then FileID. Validate only the selected source.
func mediaSource(c schema.MessagePartCommon, kind mediaKind) (value, fileID string, base64 bool, err error) {
	if c.URL != nil && *c.URL != "" {
		return *c.URL, "", false, nil
	}
	if c.Base64Data != nil && *c.Base64Data != "" {
		raw := *c.Base64Data
		if strings.HasPrefix(raw, "data:") {
			return "", "", true, fmt.Errorf("%s Base64Data must be raw base64, without a data URL prefix", kind)
		}
		if c.MIMEType == "" {
			return "", "", true, fmt.Errorf("%s part must have MIMEType when using Base64Data", kind)
		}
		return raw, "", true, nil
	}
	if c.FileID != nil && *c.FileID != "" {
		return "", *c.FileID, false, nil
	}
	return "", "", false, fmt.Errorf("%s part must contain URL, Base64Data or FileID", kind)
}

func mediaDataURL(mimeType, data string) string {
	return "data:" + mimeType + ";base64," + data
}
