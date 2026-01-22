/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticark

import (
	"reflect"

	"github.com/cloudwego/eino/schema"
)

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
	if block.Extra == nil {
		block.Extra = map[string]any{}
	}
	block.Extra[key] = value
}

func getBlockExtraValue[T any](block *schema.ContentBlock, key string) (T, bool) {
	var zero T
	if block == nil {
		return zero, false
	}
	if block.Extra == nil {
		return zero, false
	}
	val, ok := block.Extra[key].(T)
	if !ok {
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
