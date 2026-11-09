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
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
)

const (
	keyResponsesItemIndex        = "ark-responses-item-index"
	keyResponsesSection          = "ark-responses-section"
	keyResponsesContentIndex     = "ark-responses-content-index"
	keyResponsesItemType         = "ark-responses-item-type"
	keyResponsesContentType      = "ark-responses-content-type"
	keyResponsesItemID           = "ark-responses-item-id"
	keyResponsesItemStatus       = "ark-responses-item-status"
	keyResponsesItemRole         = "ark-responses-item-role"
	keyResponsesPartial          = "ark-responses-partial"
	keyResponsesAnnotations      = "ark-responses-annotations"
	keyResponsesItemRawPrefix    = "ark-responses-item-raw."
	keyResponsesContentRawPrefix = "ark-responses-content-raw."
)

// Scalar provider metadata has named, directly readable Extra entries. Complex
// annotations and unknown fields retain JSON only for that individual field.
var responseItemMetadataKeys = map[string]string{
	"type": keyResponsesItemType, "id": keyResponsesItemID,
	"status": keyResponsesItemStatus, "role": keyResponsesItemRole,
	"partial": keyResponsesPartial,
}

var responseContentMetadataKeys = map[string]string{
	"type": keyResponsesContentType, "annotations": keyResponsesAnnotations,
}

type responseObject map[string]json.RawMessage

func decodeResponseObject(raw []byte) (responseObject, error) {
	var object responseObject
	if err := sonic.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("Responses item must be an object")
	}
	return object, nil
}
func responseObjectType(object responseObject) string {
	var value string
	_ = sonic.Unmarshal(object["type"], &value)
	return value
}
func responseJSONNull(raw []byte) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// Mapped values live only in public Message fields. Extra stores provider
// fields that Eino does not model; no reverse output representation is built.
func takeResponseString(object responseObject, field string) (string, error) {
	raw, exists := object[field]
	delete(object, field)
	if !exists || responseJSONNull(raw) {
		return "", nil
	}
	var value string
	if err := sonic.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("Responses %s: %w", field, err)
	}
	return value, nil
}

func storeResponseMetadata(extra map[string]any, object responseObject, keys map[string]string, rawPrefix string) {
	for field, raw := range object {
		if key, known := keys[field]; known {
			if field == "annotations" {
				extra[key] = metadataString(raw)
				continue
			}
			if responseJSONNull(raw) {
				extra[key] = nil
				continue
			}
			if field == "partial" {
				var value bool
				if sonic.Unmarshal(raw, &value) == nil {
					extra[key] = metadataBool(value)
					continue
				}
			} else {
				var value string
				if sonic.Unmarshal(raw, &value) == nil {
					extra[key] = metadataString(value)
					continue
				}
			}
		}
		// Unknown variants can reuse familiar field names with a new shape. Preserve
		// those values as raw JSON too, rather than rejecting or coercing them.
		extra[rawPrefix+field] = metadataString(raw)
	}
}

func responseRawField(extra map[string]any, key string) (json.RawMessage, error) {
	raw, ok := extraString(extra, key)
	if !ok || !sonic.ValidString(raw) {
		return nil, fmt.Errorf("Extra[%q] must contain valid JSON", key)
	}
	return json.RawMessage(raw), nil
}
