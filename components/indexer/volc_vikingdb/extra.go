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

package volc_vikingdb


// SetExtraDataFields set data fields for vikingdb UpsertData
// see: https://www.volcengine.com/docs/84313/1254578
func SetExtraDataFields(doc *schema.Document, fields map[string]interface{}) {
	if doc == nil {
		return
	}

	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}

	doc.MetaData[extraKeyVikingDBFields] = fields
}

// SetExtraDataTTL set data ttl for vikingdb UpsertData
// see: https://www.volcengine.com/docs/84313/1254578
func SetExtraDataTTL(doc *schema.Document, ttl int64) {
	if doc == nil {
		return
	}

	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}

	doc.MetaData[extraKeyVikingDBTTL] = ttl
}

func GetExtraVikingDBFields(doc *schema.Document) (map[string]interface{}, bool) {
	if doc == nil || doc.MetaData == nil {
		return nil, false
	}

	val, ok := doc.MetaData[extraKeyVikingDBFields].(map[string]interface{})
	return val, ok
}

func GetExtraVikingDBTTL(doc *schema.Document) (int64, bool) {
	if doc == nil || doc.MetaData == nil {
		return 0, false
	}

	val, ok := doc.MetaData[extraKeyVikingDBTTL].(int64)
	return val, ok
}
