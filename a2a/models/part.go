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

package models

type PartKind string

const (
	PartKindText PartKind = "text"
	PartKindFile PartKind = "file"
	PartKindData PartKind = "data"
)

type Part struct {
	Kind PartKind `json:"kind"`
	// Text is the text content for text parts
	Text *string `json:"text,omitempty"`
	// File is the file content for file parts
	File *FileContent `json:"file,omitempty"`
	// Data is the structured data content for data parts
	Data map[string]any `json:"data,omitempty"`
	// Metadata is optional metadata associated with this part
	Metadata map[string]any `json:"metadata,omitempty"`
}

// FileContent represents the base structure for file content
type FileContent struct {
	// Name is the optional name of the file
	Name string `json:"name,omitempty"`
	// MimeType is the optional MIME type of the file content
	MimeType string `json:"mimeType,omitempty"`

	// Bytes is the file content encoded as a Base64 string
	Bytes *string `json:"bytes,omitempty"`
	// URI is the URI pointing to the file content
	URI *string `json:"uri,omitempty"`
}
