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
