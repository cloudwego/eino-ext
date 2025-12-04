/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ark

import (
	"reflect"
	"sort"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[blockExtraItemID]("_eino_ext_ark_block_extra_item_id")
	schema.RegisterName[blockExtraItemStatus]("_eino_ext_ark_block_extra_item_status")
	schema.RegisterName[*ResponseMetaExtension]("_eino_ext_ark_response_meta_extension")
	schema.RegisterName[*AssistantGenTextExtension]("_eino_ext_ark_assistant_gen_text_extension")
	schema.RegisterName[*ServerToolCallArguments]("_eino_ext_ark_server_tool_call_arguments")

	compose.RegisterStreamChunkConcatFunc(concatFirstNonZero[blockExtraItemID])
	compose.RegisterStreamChunkConcatFunc(concatLast[blockExtraItemStatus])
	compose.RegisterStreamChunkConcatFunc(concatResponseMetaExtension)
	compose.RegisterStreamChunkConcatFunc(concatAssistantGenTextExtension)
}

type blockExtraItemID string
type blockExtraItemStatus string

const (
	videoURLFPS   = "ark-user-input-video-url-fps"
	itemIDKey     = "ark-item-id"
	itemStatusKey = "ark-item-status"
)

func SetUserInputVideoFPS(block *schema.UserInputVideo, fps float64) {
	setBlockExtraValue(schema.NewContentBlock(block), videoURLFPS, fps)
}

func GetUserInputVideoFPS(block *schema.UserInputVideo) (float64, bool) {
	return getBlockExtraValue[float64](schema.NewContentBlock(block), videoURLFPS)
}

func setItemID(block *schema.ContentBlock, itemID string) {
	setBlockExtraValue(block, itemIDKey, blockExtraItemID(itemID))
}

func getItemID(block *schema.ContentBlock) (string, bool) {
	itemID, ok := getBlockExtraValue[blockExtraItemID](block, itemIDKey)
	if !ok {
		return "", false
	}
	return string(itemID), true
}

func setItemStatus(block *schema.ContentBlock, status string) {
	setBlockExtraValue(block, itemStatusKey, blockExtraItemStatus(status))
}

func GetItemStatus(block *schema.ContentBlock) (string, bool) {
	itemStatus, ok := getBlockExtraValue[blockExtraItemStatus](block, itemStatusKey)
	if !ok {
		return "", false
	}
	return string(itemStatus), true
}

func setBlockExtraValue[T any](block *schema.ContentBlock, key string, value T) {
	if block == nil {
		return
	}
	extraPtr := getBlockExtraPtr(block)
	if extraPtr != nil {
		*extraPtr = setExtra(*extraPtr, key, value)
	}
}

func getBlockExtraValue[T any](block *schema.ContentBlock, key string) (T, bool) {
	var zero T
	if block == nil {
		return zero, false
	}
	extraPtr := getBlockExtraPtr(block)
	if extraPtr != nil {
		return getExtraValue[T](*extraPtr, key)
	}
	return zero, false
}

func getBlockExtraPtr(block *schema.ContentBlock) *map[string]any {
	if block == nil {
		return nil
	}

	switch block.Type {
	case schema.ContentBlockTypeReasoning:
		if block.Reasoning != nil {
			return &block.Reasoning.Extra
		}
	case schema.ContentBlockTypeUserInputText:
		if block.UserInputText != nil {
			return &block.UserInputText.Extra
		}
	case schema.ContentBlockTypeUserInputImage:
		if block.UserInputImage != nil {
			return &block.UserInputImage.Extra
		}
	case schema.ContentBlockTypeUserInputVideo:
		if block.UserInputVideo != nil {
			return &block.UserInputVideo.Extra
		}
	case schema.ContentBlockTypeUserInputAudio:
		if block.UserInputAudio != nil {
			return &block.UserInputAudio.Extra
		}
	case schema.ContentBlockTypeUserInputFile:
		if block.UserInputFile != nil {
			return &block.UserInputFile.Extra
		}
	case schema.ContentBlockTypeAssistantGenText:
		if block.AssistantGenText != nil {
			return &block.AssistantGenText.Extra
		}
	case schema.ContentBlockTypeAssistantGenImage:
		if block.AssistantGenImage != nil {
			return &block.AssistantGenImage.Extra
		}
	case schema.ContentBlockTypeAssistantGenVideo:
		if block.AssistantGenVideo != nil {
			return &block.AssistantGenVideo.Extra
		}
	case schema.ContentBlockTypeAssistantGenAudio:
		if block.AssistantGenAudio != nil {
			return &block.AssistantGenAudio.Extra
		}
	case schema.ContentBlockTypeFunctionToolCall:
		if block.FunctionToolCall != nil {
			return &block.FunctionToolCall.Extra
		}
	case schema.ContentBlockTypeFunctionToolResult:
		if block.FunctionToolResult != nil {
			return &block.FunctionToolResult.Extra
		}
	case schema.ContentBlockTypeServerToolCall:
		if block.ServerToolCall != nil {
			return &block.ServerToolCall.Extra
		}
	case schema.ContentBlockTypeServerToolResult:
		if block.ServerToolResult != nil {
			return &block.ServerToolResult.Extra
		}
	case schema.ContentBlockTypeMCPToolCall:
		if block.MCPToolCall != nil {
			return &block.MCPToolCall.Extra
		}
	case schema.ContentBlockTypeMCPToolResult:
		if block.MCPToolResult != nil {
			return &block.MCPToolResult.Extra
		}
	case schema.ContentBlockTypeMCPListToolsResult:
		if block.MCPListToolsResult != nil {
			return &block.MCPListToolsResult.Extra
		}
	case schema.ContentBlockTypeMCPToolApprovalRequest:
		if block.MCPToolApprovalRequest != nil {
			return &block.MCPToolApprovalRequest.Extra
		}
	case schema.ContentBlockTypeMCPToolApprovalResponse:
		if block.MCPToolApprovalResponse != nil {
			return &block.MCPToolApprovalResponse.Extra
		}
	}

	return nil
}

func setExtra[T any](extra map[string]any, key string, value T) map[string]any {
	extra_ := extra
	if extra_ == nil {
		extra_ = make(map[string]any)
	}
	extra_[key] = value
	return extra_
}

func getExtraValue[T any](extra map[string]any, key string) (T, bool) {
	if extra == nil {
		var zero T
		return zero, false
	}
	val, ok := extra[key].(T)
	if !ok {
		var zero T
		return zero, false
	}
	return val, true
}

func concatFirstNonZero[T any](chunks []T) (T, error) {
	for _, chunk := range chunks {
		if !reflect.ValueOf(chunk).IsZero() {
			return chunk, nil
		}
	}
	var zero T
	return zero, nil
}

func concatFirst[T any](chunks []T) (T, error) {
	if len(chunks) == 0 {
		var zero T
		return zero, nil
	}
	return chunks[0], nil
}

func concatLast[T any](chunks []T) (T, error) {
	if len(chunks) == 0 {
		var zero T
		return zero, nil
	}
	return chunks[len(chunks)-1], nil
}

func concatResponseMetaExtension(chunks []*ResponseMetaExtension) (final *ResponseMetaExtension, err error) {
	final = &ResponseMetaExtension{}

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.ID != "" {
			final.ID = chunk.ID
		}
		if chunk.Status != "" {
			final.Status = chunk.Status
		}
		if chunk.IncompleteDetails != nil {
			final.IncompleteDetails = chunk.IncompleteDetails
		}
		if chunk.Error != nil {
			final.Error = chunk.Error
		}
		if chunk.PreviousResponseID != "" {
			final.PreviousResponseID = chunk.PreviousResponseID
		}
		if chunk.Thinking != nil {
			final.Thinking = chunk.Thinking
		}
		if chunk.ExpireAt != nil {
			final.ExpireAt = chunk.ExpireAt
		}
		if chunk.ServiceTier != "" {
			final.ServiceTier = chunk.ServiceTier
		}
		if chunk.StreamError != nil {
			final.StreamError = chunk.StreamError
		}
	}

	return final, nil
}

func concatAssistantGenTextExtension(chunks []*AssistantGenTextExtension) (final *AssistantGenTextExtension, err error) {
	indices := make(map[int64]*TextAnnotation)
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		for _, annotation := range chunk.Annotations {
			if annotation != nil {
				indices[annotation.Index] = annotation
			}
		}
	}

	annotations := make([]*TextAnnotation, 0, len(indices))
	for _, annotation := range indices {
		annotations = append(annotations, annotation)
	}

	sort.SliceStable(annotations, func(i, j int) bool {
		return annotations[i].Index < annotations[j].Index
	})

	final = &AssistantGenTextExtension{
		Annotations: annotations,
	}

	return final, nil
}
