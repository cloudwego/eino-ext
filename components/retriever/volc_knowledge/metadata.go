/*
 * Copyright 2025 CloudWeGo Authors
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

package knowledge

import "github.com/cloudwego/eino/schema"

const (
	docIDKey       = "doc_id"
	docNameKey     = "doc_name"
	chunkIDKey     = "chunk_id"
	attachmentsKey = "attachment"
	tableChunksKey = "table"
)

func setDocID(doc *schema.Document, id string) {
	doc.MetaData[docIDKey] = id
}

func setDocName(doc *schema.Document, name string) {
	doc.MetaData[docNameKey] = name
}

func setChunkID(doc *schema.Document, id int32) {
	doc.MetaData[chunkIDKey] = id
}

func setAttachments(doc *schema.Document, attachments []*ChunkAttachment) {
	doc.MetaData[attachmentsKey] = attachments
}

func setTableChunks(doc *schema.Document, tableChunks []*TableChunkField) {
	doc.MetaData[tableChunksKey] = tableChunks
}

func GetDocID(doc *schema.Document) string {
	if v, ok := doc.MetaData[docIDKey]; ok {
		return v.(string)
	}
	return ""
}

func GetDocName(doc *schema.Document) string {
	if v, ok := doc.MetaData[docNameKey]; ok {
		return v.(string)
	}
	return ""
}

func GetChunkID(doc *schema.Document) int32 {
	if v, ok := doc.MetaData[chunkIDKey]; ok {
		return v.(int32)
	}
	return 0
}

func GetAttachments(doc *schema.Document) []*ChunkAttachment {
	if v, ok := doc.MetaData[attachmentsKey]; ok {
		return v.([]*ChunkAttachment)
	}
	return nil
}

func GetTableChunks(doc *schema.Document) []*TableChunkField {
	if v, ok := doc.MetaData[tableChunksKey]; ok {
		return v.([]*TableChunkField)
	}
	return nil
}
