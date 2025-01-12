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
