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

package pgvector

import (
	"github.com/cloudwego/eino/schema"
)

// GetExtraPGVectorFields get extra pgvector fields from document
func GetExtraPGVectorFields(doc *schema.Document) (map[string]interface{}, bool) {
	if doc.MetaData == nil {
		return nil, false
	}

	fields, ok := doc.MetaData[extraKeyPGVectorFields]
	if !ok {
		return nil, false
	}

	fieldsMap, ok := fields.(map[string]interface{})
	if !ok {
		return nil, false
	}

	return fieldsMap, true
}

// GetExtraPGVectorTTL get extra pgvector ttl from document
func GetExtraPGVectorTTL(doc *schema.Document) (int64, bool) {
	if doc.MetaData == nil {
		return 0, false
	}

	ttl, ok := doc.MetaData[extraKeyPGVectorTTL]
	if !ok {
		return 0, false
	}

	ttlInt, ok := ttl.(int64)
	if !ok {
		return 0, false
	}

	return ttlInt, true
}

// SetExtraPGVectorFields set extra pgvector fields to document
func SetExtraPGVectorFields(doc *schema.Document, fields map[string]interface{}) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}

	doc.MetaData[extraKeyPGVectorFields] = fields
}

// SetExtraPGVectorTTL set extra pgvector ttl to document
func SetExtraPGVectorTTL(doc *schema.Document, ttl int64) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}

	doc.MetaData[extraKeyPGVectorTTL] = ttl
}
