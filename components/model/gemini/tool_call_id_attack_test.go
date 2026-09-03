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

package gemini

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestAttack_ToolCallIDWithThoughtSignatureRoundTrip(t *testing.T) {
	signature := []byte("opaque-thought-signature")
	message, err := convCandidate(&genai.Candidate{
		Content: &genai.Content{
			Role: roleModel,
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					ID:   "provider/call:opaque",
					Name: "same_tool",
					Args: map[string]any{"key": "alpha"},
				},
				ThoughtSignature: signature,
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, message.ToolCalls, 1)
	require.Equal(t, "provider/call:opaque", message.ToolCalls[0].ID)

	content, err := convSchemaMessage(message)
	require.NoError(t, err)
	require.Len(t, content.Parts, 1)
	require.Equal(t, "provider/call:opaque", content.Parts[0].FunctionCall.ID)
	require.Equal(t, signature, content.Parts[0].ThoughtSignature)
}

func TestAttack_MultimodalToolResultKeepsID(t *testing.T) {
	text := `{"opaque":"result"}`
	image := base64.StdEncoding.EncodeToString([]byte("image-content"))
	message := &schema.Message{
		Role:       schema.Tool,
		ToolCallID: "provider-call-id",
		ToolName:   "same_tool",
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: text,
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &image,
						MIMEType:   "image/png",
					},
				},
			},
		},
	}

	content, err := convSchemaMessage(message)
	require.NoError(t, err)
	require.Len(t, content.Parts, 1)
	require.Equal(t, "provider-call-id", content.Parts[0].FunctionResponse.ID)
	require.Equal(t, "same_tool", content.Parts[0].FunctionResponse.Name)
	require.Equal(t, "result", content.Parts[0].FunctionResponse.Response["opaque"])
	require.Len(t, content.Parts[0].FunctionResponse.Parts, 1)
	require.Equal(t, []byte("image-content"), content.Parts[0].FunctionResponse.Parts[0].InlineData.Data)

	_, err = convSchemaMessage(&schema.Message{
		Role:       schema.Tool,
		ToolCallID: "provider-call-id",
		ToolName:   "same_tool",
		UserInputMultiContent: []schema.MessageInputPart{{
			Type: schema.ChatMessagePartTypeImageURL,
		}},
	})
	require.EqualError(t, err, "image field must not be nil when Type is ChatMessagePartTypeImageURL in tool message")
}
