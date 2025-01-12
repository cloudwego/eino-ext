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

package field_mapping

import (
	"fmt"

	"github.com/cloudwego/eino-ext/components/retriever/es8/internal"
	"github.com/cloudwego/eino/schema"
)

// GetDefaultVectorFieldKeyContent get default es key for Document.Content
func GetDefaultVectorFieldKeyContent() FieldName {
	return defaultVectorFieldKeyContent
}

// GetDefaultVectorFieldKey generate default vector field name from its field name
func GetDefaultVectorFieldKey(fieldName string) FieldName {
	return FieldName(fmt.Sprintf("vector_%s", fieldName))
}

// GetExtraDataFields get data fields from *schema.Document
func GetExtraDataFields(doc *schema.Document) (fields map[string]interface{}, ok bool) {
	if doc == nil || doc.MetaData == nil {
		return nil, false
	}

	fields, ok = doc.MetaData[internal.DocExtraKeyEsFields].(map[string]interface{})

	return fields, ok
}

type FieldKV struct {
	// FieldNameVector vector field name (if needed)
	FieldNameVector FieldName `json:"field_name_vector,omitempty"`
	// FieldName field name
	FieldName FieldName `json:"field_name,omitempty"`
	// Value original value
	Value string `json:"value,omitempty"`
}

type FieldName string

var defaultVectorFieldKeyContent = GetDefaultVectorFieldKey(DocFieldNameContent)
