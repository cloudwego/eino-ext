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

	"github.com/cloudwego/eino-ext/components/indexer/es8/internal"
	"github.com/cloudwego/eino/schema"
)

// SetExtraDataFields set data fields for es
func SetExtraDataFields(doc *schema.Document, fields map[string]interface{}) {
	if doc == nil {
		return
	}

	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}

	doc.MetaData[internal.DocExtraKeyEsFields] = fields
}

// GetExtraDataFields get data fields from *schema.Document
func GetExtraDataFields(doc *schema.Document) (fields map[string]interface{}, ok bool) {
	if doc == nil || doc.MetaData == nil {
		return nil, false
	}

	fields, ok = doc.MetaData[internal.DocExtraKeyEsFields].(map[string]interface{})

	return fields, ok
}

// DefaultFieldKV build default names by fieldName
// docFieldName should be DocFieldNameContent or key got from GetExtraDataFields
func DefaultFieldKV(docFieldName FieldName) FieldKV {
	return FieldKV{
		FieldNameVector: FieldName(fmt.Sprintf("vector_%s", docFieldName)),
		FieldName:       docFieldName,
	}
}

type FieldKV struct {
	// FieldNameVector vector field name (if needed)
	FieldNameVector FieldName `json:"field_name_vector,omitempty"`
	// FieldName field name
	FieldName FieldName `json:"field_name,omitempty"`
}

type FieldName string

func (v FieldName) Find(doc *schema.Document) (string, bool) {
	if v == DocFieldNameContent {
		return doc.Content, true
	}

	kvs, ok := GetExtraDataFields(doc)
	if !ok {
		return "", false
	}

	s, ok := kvs[string(v)].(string)
	return s, ok
}
